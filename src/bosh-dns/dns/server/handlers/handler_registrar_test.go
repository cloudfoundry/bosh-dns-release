package handlers_test

import (
	"time"

	"code.cloudfoundry.org/clock/fakeclock"

	"bosh-dns/dns/server/handlers"
	"bosh-dns/dns/server/handlers/handlersfakes"

	"github.com/cloudfoundry/bosh-utils/logger/loggerfakes"
	"github.com/miekg/dns"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type HandlerRegistrarTestHandler struct{}

func (*HandlerRegistrarTestHandler) ServeDNS(dns.ResponseWriter, *dns.Msg) {}

var _ = Describe("HandlerRegistrar", func() {
	var (
		logger           *loggerfakes.FakeLogger
		domainProvider   *handlersfakes.FakeDomainProvider
		mux              *handlersfakes.FakeServerMux
		handlerRegistrar handlers.HandlerRegistrar
		childHandler     dns.Handler
		clock            *fakeclock.FakeClock
	)

	BeforeEach(func() {
		logger = &loggerfakes.FakeLogger{}
		domainProvider = &handlersfakes.FakeDomainProvider{}
		mux = &handlersfakes.FakeServerMux{}
		childHandler = &HandlerRegistrarTestHandler{}
		clock = fakeclock.NewFakeClock(time.Now())
		handlerRegistrar = handlers.NewHandlerRegistrar(logger, clock, domainProvider, mux, childHandler)
	})

	Describe("Run", func() {
		var shutdown chan struct{}

		BeforeEach(func() {
			shutdown = make(chan struct{})

			domainProvider.DomainsReturns([]string{"initial-domain1", "initial-domain2"})
		})

		invoke := func(run func(signal chan struct{}) error) {
			go func() {
				defer GinkgoRecover()
				err := run(shutdown)
				Expect(err).ToNot(HaveOccurred())
			}()
		}

		It("registers the initial domains", func() {
			invoke(handlerRegistrar.Run)
			defer close(shutdown)

			clock.WaitForWatcherAndIncrement(handlers.RegisterInterval)
			Eventually(mux.HandleCallCount).Should(Equal(3))

			pattern, handler := mux.HandleArgsForCall(1)
			Expect(pattern).To(Equal("initial-domain1"))
			Expect(handler).To(BeAssignableToTypeOf(handlers.RequestLoggerHandler{}))
			Expect(handler.(handlers.RequestLoggerHandler).Handler).To(Equal(childHandler))

			pattern, handler = mux.HandleArgsForCall(2)
			Expect(pattern).To(Equal("initial-domain2"))
			Expect(handler).To(BeAssignableToTypeOf(handlers.RequestLoggerHandler{}))
			Expect(handler.(handlers.RequestLoggerHandler).Handler).To(Equal(childHandler))
		})

		Context("new domain added", func() {
			It("registers the new domain", func() {
				invoke(handlerRegistrar.Run)
				defer close(shutdown)

				clock.WaitForWatcherAndIncrement(handlers.RegisterInterval)
				Eventually(mux.HandleCallCount).Should(Equal(3))

				domainProvider.DomainsReturns([]string{"initial-domain1", "initial-domain2", "new-domain"})

				clock.WaitForWatcherAndIncrement(handlers.RegisterInterval)
				Eventually(mux.HandleCallCount).Should(Equal(4))

				pattern, handler := mux.HandleArgsForCall(3)
				Expect(pattern).To(Equal("new-domain"))
				Expect(handler).To(BeAssignableToTypeOf(handlers.RequestLoggerHandler{}))
				Expect(handler.(handlers.RequestLoggerHandler).Handler).To(Equal(childHandler))
			})
		})

		Context("existing domain removed", func() {
			It("removes the domain", func() {
				invoke(handlerRegistrar.Run)
				defer close(shutdown)

				clock.WaitForWatcherAndIncrement(handlers.RegisterInterval)
				Eventually(mux.HandleCallCount).Should(Equal(3))

				domainProvider.DomainsReturns([]string{"initial-domain2"})

				clock.WaitForWatcherAndIncrement(handlers.RegisterInterval)

				Eventually(mux.HandleRemoveCallCount).Should(Equal(1))

				Expect(mux.HandleRemoveArgsForCall(0)).To(Equal("initial-domain1"))
			})
		})
	})
})
