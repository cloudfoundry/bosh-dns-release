package handlers_test

import (
	"errors"
	"net"

	"github.com/cloudfoundry/dns-release/src/dns/server/handlers"
	"github.com/cloudfoundry/dns-release/src/dns/server/handlers/internal/internalfakes"
	"github.com/miekg/dns"

	"time"

	"github.com/cloudfoundry/dns-release/src/dns/server/handlers/handlersfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("Recursion", func() {
	Describe("ServeDNS", func() {
		var (
			fakeWriter       *internalfakes.FakeResponseWriter
			recursionHandler handlers.Recursion
			fakeExchanger    *handlersfakes.FakeExchanger
		)

		BeforeEach(func() {
			fakeWriter = &internalfakes.FakeResponseWriter{}
			fakeExchanger = &handlersfakes.FakeExchanger{}
			fakeExchangerFactory := func(net string) handlers.Exchanger {
				return fakeExchanger
			}
			recursionHandler = handlers.NewRecursion([]string{"127.0.0.1", "10.244.5.4"}, fakeExchangerFactory)
		})

		Context("when no recursors are configured", func() {
			It("sets a failure rcode", func() {
				recursionHandler := handlers.NewRecursion([]string{}, func(string) handlers.Exchanger { return nil })

				m := &dns.Msg{}
				m.SetQuestion("example.com.", dns.TypeANY)
				recursionHandler.ServeDNS(fakeWriter, m)
				Expect(fakeWriter.WriteMsgCallCount()).To(Equal(1))
				message := fakeWriter.WriteMsgArgsForCall(0)
				Expect(message.Question).To(Equal(m.Question))
				Expect(message.Rcode).To(Equal(dns.RcodeServerFailure))
			})
		})

		Context("when request contains no questions", func() {
			It("set a success rcode and authorative", func() {
				recursionHandler.ServeDNS(fakeWriter, &dns.Msg{})
				message := fakeWriter.WriteMsgArgsForCall(0)
				Expect(message.Rcode).To(Equal(dns.RcodeSuccess))
				Expect(message.Authoritative).To(Equal(true))
				Expect(message.RecursionAvailable).To(Equal(false))
			})
		})

		Context("when request contains questions", func() {
			DescribeTable("it responds to DNS requests",
				func(protocol string, remoteAddrReturns net.Addr) {
					exchangeMsg := &dns.Msg{
						Answer: []dns.RR{&dns.A{A: net.ParseIP("99.99.99.99")}},
					}
					fakeExchanger := &handlersfakes.FakeExchanger{}
					fakeExchanger.ExchangeReturns(exchangeMsg, 0, nil)

					fakeExchangerFactory := func(net string) handlers.Exchanger {
						if net == protocol {
							return fakeExchanger
						}

						return &handlersfakes.FakeExchanger{}
					}

					fakeWriter.RemoteAddrReturns(remoteAddrReturns)
					recursionHandler := handlers.NewRecursion([]string{"127.0.0.1"}, fakeExchangerFactory)

					m := &dns.Msg{}
					m.SetQuestion("example.com.", dns.TypeANY)

					recursionHandler.ServeDNS(fakeWriter, m)
					message := fakeWriter.WriteMsgArgsForCall(0)
					Expect(message.Rcode).To(Equal(dns.RcodeSuccess))
					Expect(message.Answer).To(Equal(exchangeMsg.Answer))

					Expect(fakeExchanger.ExchangeCallCount()).To(Equal(1))
					msg, recursor := fakeExchanger.ExchangeArgsForCall(0)
					Expect(recursor).To(Equal("127.0.0.1"))
					Expect(msg).To(Equal(m))
				},
				Entry("forwards query to recursor via udp for udp clients", "udp", nil),
				Entry("forwards query to recursor via tcp for tcp clients", "tcp", &net.TCPAddr{}),
			)

			It("returns with the first recursor response", func() {
				exchangeMsg := &dns.Msg{
					Answer: []dns.RR{&dns.A{A: net.ParseIP("99.99.99.99")}},
				}

				fakeExchanger.ExchangeStub = func(msg *dns.Msg, address string) (*dns.Msg, time.Duration, error) {
					if address == "10.244.5.4" {
						return exchangeMsg, 0, nil
					}
					return nil, 0, errors.New("recursor failed to reply")
				}

				m := &dns.Msg{}
				m.SetQuestion("example.com.", dns.TypeANY)

				recursionHandler.ServeDNS(fakeWriter, m)
				message := fakeWriter.WriteMsgArgsForCall(0)
				Expect(message.Rcode).To(Equal(dns.RcodeSuccess))
				Expect(message.Answer).To(Equal(exchangeMsg.Answer))

				Expect(fakeExchanger.ExchangeCallCount()).To(Equal(2))
				msg, recursor := fakeExchanger.ExchangeArgsForCall(1)
				Expect(recursor).To(Equal("10.244.5.4"))
				Expect(msg).To(Equal(m))
			})
		})
	})
})
