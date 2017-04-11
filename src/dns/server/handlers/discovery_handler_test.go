package handlers_test

import (
	"errors"
	"github.com/cloudfoundry/bosh-utils/logger/loggerfakes"
	"github.com/cloudfoundry/dns-release/src/dns/server/handlers"
	"github.com/cloudfoundry/dns-release/src/dns/server/handlers/handlersfakes"
	"github.com/cloudfoundry/dns-release/src/dns/server/handlers/internal/internalfakes"
	"github.com/miekg/dns"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/ginkgo/extensions/table"
)

var _ = Describe("DiscoveryHandler", func() {
	Context("ServeDNS", func() {
		var (
			discoveryHandler handlers.DiscoveryHandler
			fakeWriter       *internalfakes.FakeResponseWriter
			fakeLogger       *loggerfakes.FakeLogger
			fakeIPGetter     *handlersfakes.FakeIPGetter
		)

		BeforeEach(func() {
			fakeWriter = &internalfakes.FakeResponseWriter{}
			fakeLogger = &loggerfakes.FakeLogger{}
			fakeIPGetter = &handlersfakes.FakeIPGetter{}

			discoveryHandler = handlers.NewDiscoveryHandler(fakeLogger, fakeIPGetter)
		})

		Context("when there are no questions", func() {
			It("returns rcode success", func() {
				discoveryHandler.ServeDNS(fakeWriter, &dns.Msg{})
				message := fakeWriter.WriteMsgArgsForCall(0)
				Expect(message.Rcode).To(Equal(dns.RcodeSuccess))
				Expect(message.Authoritative).To(Equal(true))
				Expect(message.RecursionAvailable).To(Equal(false))
			})
		})

		Context("when there are questions", func() {
			It("returns rcode success for mx questions", func() {
				m := &dns.Msg{}
				m.SetQuestion("my-instance.my-network.my-deployment.bosh.", dns.TypeMX)

				discoveryHandler.ServeDNS(fakeWriter, m)
				message := fakeWriter.WriteMsgArgsForCall(0)
				Expect(message.Rcode).To(Equal(dns.RcodeSuccess))
				Expect(message.Authoritative).To(Equal(true))
				Expect(message.RecursionAvailable).To(Equal(false))
			})

			It("returns rcode success for aaaa questions", func() {
				m := &dns.Msg{}
				m.SetQuestion("my-instance.my-network.my-deployment.bosh.", dns.TypeAAAA)

				discoveryHandler.ServeDNS(fakeWriter, m)
				message := fakeWriter.WriteMsgArgsForCall(0)
				Expect(message.Rcode).To(Equal(dns.RcodeSuccess))
				Expect(message.Authoritative).To(Equal(true))
				Expect(message.RecursionAvailable).To(Equal(false))
			})

			It("returns rcode server failure for all other questions", func() {
				m := &dns.Msg{}
				m.SetQuestion("my-instance.my-network.my-deployment.bosh.", dns.TypePTR)

				discoveryHandler.ServeDNS(fakeWriter, m)
				message := fakeWriter.WriteMsgArgsForCall(0)
				Expect(message.Rcode).To(Equal(dns.RcodeServerFailure))
				Expect(message.Authoritative).To(Equal(true))
				Expect(message.RecursionAvailable).To(Equal(false))
			})

			Context("when the question is an A or ANY record", func() {
				DescribeTable("returns an A record based off of the records data",
					func(queryType uint16) {
						fakeIPGetter.GetIPsReturns([]string{"123.123.123.123"}, nil)

						m := &dns.Msg{}
						m.SetQuestion("my-instance.my-network.my-deployment.bosh.", queryType)

						discoveryHandler.ServeDNS(fakeWriter, m)
						message := fakeWriter.WriteMsgArgsForCall(0)

						Expect(message.Rcode).To(Equal(dns.RcodeSuccess))
						Expect(message.Authoritative).To(Equal(true))
						Expect(message.RecursionAvailable).To(Equal(false))

						Expect(message.Answer).To(HaveLen(1))

						answer := message.Answer[0]
						header := answer.Header()

						Expect(header.Rrtype).To(Equal(dns.TypeA))
						Expect(header.Class).To(Equal(uint16(dns.ClassINET)))
						Expect(header.Ttl).To(Equal(uint32(0)))

						Expect(answer).To(BeAssignableToTypeOf(&dns.A{}))
						Expect(answer.(*dns.A).A.String()).To(Equal("123.123.123.123"))

						Expect(fakeLogger.InfoCallCount()).To(Equal(0))
					},
					Entry("when the question is an A query", dns.TypeA),
					Entry("when the question is an ANY query", dns.TypeANY),
				)

				It("logs a message if multiple records were found for the FQDN", func() {
					fakeIPGetter.GetIPsReturns([]string{"123.123.123.123", "127.0.0.1"}, nil)

					m := &dns.Msg{}

					m.SetQuestion("my-instance.my-network.my-deployment.bosh.", dns.TypeA)
					discoveryHandler.ServeDNS(fakeWriter, m)

					Expect(fakeLogger.InfoCallCount()).To(Equal(1))
					tag, msg, args := fakeLogger.InfoArgsForCall(0)
					Expect(tag).To(Equal("DiscoveryHandler"))
					Expect(msg).To(Equal("got multiple ip addresses for %s: %v"))
					Expect(args[0]).To(Equal("my-instance.my-network.my-deployment.bosh."))
					Expect(args[1]).To(Equal([]string{"123.123.123.123", "127.0.0.1"}))
				})
			})

			Context("when fetching ips returns an error", func() {
				BeforeEach(func() {
					fakeIPGetter.GetIPsReturns([]string{}, errors.New("i screwed up"))
				})

				It("returns rcode server failure", func() {
					m := &dns.Msg{}
					m.SetQuestion("my-instance.my-network.my-deployment.bosh.", dns.TypeA)

					discoveryHandler.ServeDNS(fakeWriter, m)
					message := fakeWriter.WriteMsgArgsForCall(0)
					Expect(message.Rcode).To(Equal(dns.RcodeServerFailure))
					Expect(message.Authoritative).To(Equal(true))
					Expect(message.RecursionAvailable).To(Equal(false))
				})

				It("logs the error", func() {
					m := &dns.Msg{}
					m.SetQuestion("my-instance.my-network.my-deployment.bosh.", dns.TypeA)
					discoveryHandler.ServeDNS(fakeWriter, m)

					Expect(fakeLogger.ErrorCallCount()).To(Equal(1))
					tag, msg, args := fakeLogger.ErrorArgsForCall(0)
					Expect(tag).To(Equal("DiscoveryHandler"))
					Expect(msg).To(Equal("failed to get ip addresses: %v"))
					Expect(args[0]).To(MatchError("i screwed up"))
				})
			})
		})

		Context("logging", func() {
			It("logs an error if the response fails to write", func() {
				fakeWriter.WriteMsgReturns(errors.New("failed to write message"))

				discoveryHandler.ServeDNS(fakeWriter, &dns.Msg{})

				Expect(fakeLogger.ErrorCallCount()).To(Equal(1))
				tag, msg, _ := fakeLogger.ErrorArgsForCall(0)
				Expect(tag).To(Equal("DiscoveryHandler"))
				Expect(msg).To(Equal("failed to write message"))
			})
		})
	})
})
