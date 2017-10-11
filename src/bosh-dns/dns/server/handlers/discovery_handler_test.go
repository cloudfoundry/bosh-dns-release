package handlers_test

import (
	"errors"
	"net"

	"bosh-dns/dns/server/handlers"
	"bosh-dns/dns/server/internal/internalfakes"
	"bosh-dns/dns/server/records/dnsresolver"
	"bosh-dns/dns/server/records/dnsresolver/dnsresolverfakes"

	"github.com/cloudfoundry/bosh-utils/logger/loggerfakes"
	"github.com/miekg/dns"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("DiscoveryHandler", func() {
	Context("ServeDNS", func() {
		var (
			discoveryHandler handlers.DiscoveryHandler
			fakeWriter       *internalfakes.FakeResponseWriter
			fakeLogger       *loggerfakes.FakeLogger
			fakeRecordSet    *dnsresolverfakes.FakeRecordSet
			fakeShuffler     *dnsresolverfakes.FakeAnswerShuffler
		)

		BeforeEach(func() {
			fakeWriter = &internalfakes.FakeResponseWriter{}
			fakeLogger = &loggerfakes.FakeLogger{}
			fakeRecordSet = &dnsresolverfakes.FakeRecordSet{}
			fakeShuffler = &dnsresolverfakes.FakeAnswerShuffler{}
			fakeShuffler.ShuffleStub = func(input []dns.RR) []dns.RR {
				return input
			}

			fakeWriter.RemoteAddrReturns(&net.UDPAddr{})
			discoveryHandler = handlers.NewDiscoveryHandler(fakeLogger, dnsresolver.NewLocalDomain(fakeLogger, fakeRecordSet, fakeShuffler))
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
						fakeRecordSet.ResolveReturns([]string{"123.123.123.123"}, nil)

						m := &dns.Msg{}
						m.SetQuestion("my-instance.my-group.my-network.my-deployment.bosh.", queryType)

						discoveryHandler.ServeDNS(fakeWriter, m)
						responseMsg := fakeWriter.WriteMsgArgsForCall(0)

						Expect(responseMsg.Rcode).To(Equal(dns.RcodeSuccess))
						Expect(responseMsg.Authoritative).To(Equal(true))
						Expect(responseMsg.RecursionAvailable).To(Equal(true))
						Expect(responseMsg.Truncated).To(Equal(false))

						Expect(responseMsg.Answer).To(HaveLen(1))

						answer := responseMsg.Answer[0]
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
