package handlers_test

import (
	"errors"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"

	"github.com/cloudfoundry/bosh-utils/logger/loggerfakes"
	"github.com/cloudfoundry/dns-release/src/dns/server/handlers"
	"github.com/cloudfoundry/dns-release/src/dns/server/handlers/handlersfakes"
	"github.com/cloudfoundry/dns-release/src/dns/server/records"
	"github.com/cloudfoundry/dns-release/src/dns/server/records/dnsresolver/dnsresolverfakes"
	"github.com/miekg/dns"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type HandlerRegistrarTestHandler struct{}

func (*HandlerRegistrarTestHandler) ServeDNS(dns.ResponseWriter, *dns.Msg) {}

var _ = Describe("HandlerRegistrar", func() {
	var (
		logger           *loggerfakes.FakeLogger
		recordsRepo      *dnsresolverfakes.FakeRecordSetRepo
		mux              *handlersfakes.FakeServerMux
		handlerRegistrar handlers.HandlerRegistrar
		childHandler     dns.Handler
		clock            *fakeclock.FakeClock
	)

	BeforeEach(func() {
		logger = &loggerfakes.FakeLogger{}
		recordsRepo = &dnsresolverfakes.FakeRecordSetRepo{}
		mux = &handlersfakes.FakeServerMux{}
		childHandler = &HandlerRegistrarTestHandler{}
		clock = fakeclock.NewFakeClock(time.Now())
		handlerRegistrar = handlers.NewHandlerRegistrar(logger, clock, recordsRepo, mux, childHandler)
	})

	Describe("Run", func() {
		var shutdown chan struct{}

		BeforeEach(func() {
			shutdown = make(chan struct{})

			recordsRepo.GetReturns(records.RecordSet{
				Domains: []string{"initial-domain1", "initial-domain2"},
			}, nil)
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
			Eventually(mux.HandleCallCount).Should(Equal(2))

			pattern, handler := mux.HandleArgsForCall(0)
			Expect(pattern).To(Equal("initial-domain1"))
			Expect(handler).To(BeAssignableToTypeOf(handlers.RequestLoggerHandler{}))
			Expect(handler.(handlers.RequestLoggerHandler).Handler).To(Equal(childHandler))

			pattern, handler = mux.HandleArgsForCall(1)
			Expect(pattern).To(Equal("initial-domain2"))
			Expect(handler).To(BeAssignableToTypeOf(handlers.RequestLoggerHandler{}))
			Expect(handler.(handlers.RequestLoggerHandler).Handler).To(Equal(childHandler))
		})

		Context("when the repo fails", func() {
			var getErr error

			BeforeEach(func() {
				getErr = errors.New("no potato for you")
				recordsRepo.GetReturns(records.RecordSet{}, getErr)
			})

			It("logs and retries", func() {
				invoke(handlerRegistrar.Run)
				defer close(shutdown)

				clock.WaitForWatcherAndIncrement(handlers.RegisterInterval)
				Eventually(logger.ErrorCallCount).Should(Equal(1))
				tag, msg, stuff := logger.ErrorArgsForCall(0)
				Expect(tag).To(Equal("handler-registrar"))
				Expect(msg).To(Equal("cannot get record set"))
				Expect(stuff).To(HaveLen(1))
				Expect(stuff[0]).To(Equal(getErr))

				recordsRepo.GetReturns(records.RecordSet{
					Domains: []string{"some-domain"},
				}, nil)

				clock.WaitForWatcherAndIncrement(handlers.RegisterInterval)
				Eventually(mux.HandleCallCount).Should(Equal(1))
			})
		})

		Context("new domain added", func() {
			It("registers the new domain", func() {
				invoke(handlerRegistrar.Run)
				defer close(shutdown)

				clock.WaitForWatcherAndIncrement(handlers.RegisterInterval)
				Eventually(mux.HandleCallCount).Should(Equal(2))

				recordsRepo.GetReturns(records.RecordSet{
					Domains: []string{"initial-domain1", "initial-domain2", "new-domain"},
				}, nil)

				clock.WaitForWatcherAndIncrement(handlers.RegisterInterval)
				Eventually(mux.HandleCallCount).Should(Equal(3))

				pattern, handler := mux.HandleArgsForCall(2)
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
				Eventually(mux.HandleCallCount).Should(Equal(2))

				recordsRepo.GetReturns(records.RecordSet{
					Domains: []string{"initial-domain2"},
				}, nil)

				clock.WaitForWatcherAndIncrement(handlers.RegisterInterval)

				Eventually(mux.HandleRemoveCallCount).Should(Equal(1))

				Expect(mux.HandleRemoveArgsForCall(0)).To(Equal("initial-domain1"))
			})
		})
	})
})
