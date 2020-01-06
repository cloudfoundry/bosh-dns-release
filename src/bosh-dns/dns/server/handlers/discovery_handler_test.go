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
			fakeTruncater    *dnsresolverfakes.FakeResponseTruncater
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
			fakeTruncater = &dnsresolverfakes.FakeResponseTruncater{}
			discoveryHandler = handlers.NewDiscoveryHandler(fakeLogger, dnsresolver.NewLocalDomain(fakeLogger, fakeRecordSet, fakeShuffler, fakeTruncater))
		})

		Context("when there are no questions", func() {
			It("returns rcode success", func() {
				discoveryHandler.ServeDNS(fakeWriter, &dns.Msg{})
				message := fakeWriter.WriteMsgArgsForCall(0)
				Expect(message.Rcode).To(Equal(dns.RcodeSuccess))
				Expect(message.Authoritative).To(BeTrue())
				Expect(message.RecursionAvailable).To(BeTrue())
			})
		})

		Context("when there are questions", func() {
			It("returns rcode success for MX questions", func() {
				m := &dns.Msg{}
				m.SetQuestion("my-instance.my-network.my-deployment.bosh.", dns.TypeMX)

				discoveryHandler.ServeDNS(fakeWriter, m)
				message := fakeWriter.WriteMsgArgsForCall(0)
				Expect(message.Rcode).To(Equal(dns.RcodeSuccess))
				Expect(message.Authoritative).To(BeTrue())
				Expect(message.RecursionAvailable).To(BeTrue())
			})

			It("returns rcode success for A questions when there are no matching records", func() {
				m := &dns.Msg{}
				m.SetQuestion("my-instance.my-network.my-deployment.bosh.", dns.TypeA)

				discoveryHandler.ServeDNS(fakeWriter, m)
				message := fakeWriter.WriteMsgArgsForCall(0)
				Expect(message.Rcode).To(Equal(dns.RcodeSuccess))
				Expect(message.Authoritative).To(BeTrue())
				Expect(message.RecursionAvailable).To(BeTrue())
			})

			It("returns rcode success for AAAA questions when there are no matching records", func() {
				m := &dns.Msg{}
				m.SetQuestion("my-instance.my-network.my-deployment.bosh.", dns.TypeAAAA)

				discoveryHandler.ServeDNS(fakeWriter, m)
				message := fakeWriter.WriteMsgArgsForCall(0)
				Expect(message.Rcode).To(Equal(dns.RcodeSuccess))
				Expect(message.Authoritative).To(BeTrue())
				Expect(message.RecursionAvailable).To(BeTrue())
			})

			It("returns rcode not implements for srv questions", func() {
				m := &dns.Msg{}
				m.SetQuestion("my-instance.my-network.my-deployment.bosh.", dns.TypeSRV)

				discoveryHandler.ServeDNS(fakeWriter, m)
				message := fakeWriter.WriteMsgArgsForCall(0)
				Expect(message.Rcode).To(Equal(dns.RcodeNotImplemented))
				Expect(message.Authoritative).To(BeTrue())
				Expect(message.RecursionAvailable).To(BeTrue())
			})

			It("returns rcode server failure for all other questions", func() {
				m := &dns.Msg{}
				m.SetQuestion("my-instance.my-network.my-deployment.bosh.", dns.TypePTR)

				discoveryHandler.ServeDNS(fakeWriter, m)
				message := fakeWriter.WriteMsgArgsForCall(0)
				Expect(message.Rcode).To(Equal(dns.RcodeServerFailure))
				Expect(message.Authoritative).To(BeTrue())
				Expect(message.RecursionAvailable).To(BeTrue())
			})

			// q: A -> only A even if AAAA
			// q: AAAA -> only AAAA even if A
			// q: ANY -> both A and AAAA

			It("returns only A records (no AAAA records) when the queried for A records", func() {
				fakeRecordSet.ResolveReturns([]string{"2601:0646:0102:0095:0000:0000:0000:0025", "123.123.123.123"}, nil)

				m := &dns.Msg{}
				m.SetQuestion("my-instance.my-group.my-network.my-deployment.bosh.", dns.TypeA)

				discoveryHandler.ServeDNS(fakeWriter, m)
				responseMsg := fakeWriter.WriteMsgArgsForCall(0)

				Expect(responseMsg.Rcode).To(Equal(dns.RcodeSuccess))
				Expect(responseMsg.Authoritative).To(BeTrue())
				Expect(responseMsg.RecursionAvailable).To(BeTrue())
				Expect(responseMsg.Truncated).To(BeFalse())

				Expect(responseMsg.Answer).To(HaveLen(1))

				answer := responseMsg.Answer[0]
				header := answer.Header()

				Expect(header.Rrtype).To(Equal(dns.TypeA))
				Expect(header.Class).To(Equal(uint16(dns.ClassINET)))
				Expect(header.Ttl).To(Equal(uint32(0)))

				Expect(answer).To(BeAssignableToTypeOf(&dns.A{}))
				Expect(answer.(*dns.A).A.String()).To(Equal("123.123.123.123"))

				Expect(fakeLogger.InfoCallCount()).To(Equal(0))
			})

			It("returns only AAAA records (no A records) when the queried for AAAA records", func() {
				fakeRecordSet.ResolveReturns([]string{"2601:0646:0102:0095:0000:0000:0000:0025", "4.2.2.2"}, nil)

				m := &dns.Msg{}
				m.SetQuestion("my-instance.my-group.my-network.my-deployment.bosh.", dns.TypeAAAA)

				discoveryHandler.ServeDNS(fakeWriter, m)
				responseMsg := fakeWriter.WriteMsgArgsForCall(0)

				Expect(responseMsg.Rcode).To(Equal(dns.RcodeSuccess))
				Expect(responseMsg.Authoritative).To(BeTrue())
				Expect(responseMsg.RecursionAvailable).To(BeTrue())
				Expect(responseMsg.Truncated).To(BeFalse())

				Expect(responseMsg.Answer).To(HaveLen(1))

				answer := responseMsg.Answer[0]
				header := answer.Header()

				Expect(header.Rrtype).To(Equal(dns.TypeAAAA))
				Expect(header.Class).To(Equal(uint16(dns.ClassINET)))
				Expect(header.Ttl).To(Equal(uint32(0)))

				Expect(answer).To(BeAssignableToTypeOf(&dns.AAAA{}))
				Expect(answer.(*dns.AAAA).AAAA.String()).To(Equal("2601:646:102:95::25"))

				Expect(fakeLogger.InfoCallCount()).To(Equal(0))
			})

			It("returns both A and AAAA records when the queried for ANY records", func() {
				fakeRecordSet.ResolveReturns([]string{"2601:0646:0102:0095:0000:0000:0000:0025", "4.2.2.2"}, nil)

				m := &dns.Msg{}
				m.SetQuestion("my-instance.my-group.my-network.my-deployment.bosh.", dns.TypeANY)

				discoveryHandler.ServeDNS(fakeWriter, m)
				responseMsg := fakeWriter.WriteMsgArgsForCall(0)

				Expect(responseMsg.Rcode).To(Equal(dns.RcodeSuccess))
				Expect(responseMsg.Authoritative).To(BeTrue())
				Expect(responseMsg.RecursionAvailable).To(BeTrue())
				Expect(responseMsg.Truncated).To(BeFalse())

				Expect(responseMsg.Answer).To(HaveLen(2))

				{
					answer := responseMsg.Answer[0]
					header := answer.Header()

					Expect(header.Rrtype).To(Equal(dns.TypeAAAA))
					Expect(header.Class).To(Equal(uint16(dns.ClassINET)))
					Expect(header.Ttl).To(Equal(uint32(0)))

					Expect(answer).To(BeAssignableToTypeOf(&dns.AAAA{}))
					Expect(answer.(*dns.AAAA).AAAA.String()).To(Equal("2601:646:102:95::25"))
				}

				{
					answer := responseMsg.Answer[1]
					header := answer.Header()

					Expect(header.Rrtype).To(Equal(dns.TypeA))
					Expect(header.Class).To(Equal(uint16(dns.ClassINET)))
					Expect(header.Ttl).To(Equal(uint32(0)))

					Expect(answer).To(BeAssignableToTypeOf(&dns.A{}))
					Expect(answer.(*dns.A).A.String()).To(Equal("4.2.2.2"))
				}

				Expect(fakeLogger.InfoCallCount()).To(Equal(0))
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
