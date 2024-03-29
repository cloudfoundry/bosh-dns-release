package handlers_test

import (
	"errors"
	"fmt"
	"net"

	"github.com/cloudfoundry/bosh-utils/logger/loggerfakes"
	"github.com/miekg/dns"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "bosh-dns/dns/internal/testhelpers/question_case_helpers"
	"bosh-dns/dns/server/handlers"
	"bosh-dns/dns/server/internal/internalfakes"
	"bosh-dns/dns/server/records"
	"bosh-dns/dns/server/records/dnsresolver"
	"bosh-dns/dns/server/records/dnsresolver/dnsresolverfakes"
)

var _ = Describe("DiscoveryHandler", func() {
	Context("ServeDNS", func() {
		var (
			discoveryHandler handlers.DiscoveryHandler
			fakeWriter       *internalfakes.FakeResponseWriter
			fakeLogger       *loggerfakes.FakeLogger
			fakeRecordSet    *dnsresolverfakes.FakeRecordSet
			fakeTruncater    *dnsresolverfakes.FakeResponseTruncater
		)

		BeforeEach(func() {
			fakeWriter = &internalfakes.FakeResponseWriter{}
			fakeLogger = &loggerfakes.FakeLogger{}
			fakeRecordSet = &dnsresolverfakes.FakeRecordSet{}

			fakeWriter.RemoteAddrReturns(&net.UDPAddr{})
			fakeTruncater = &dnsresolverfakes.FakeResponseTruncater{}
			discoveryHandler = handlers.NewDiscoveryHandler(fakeLogger, dnsresolver.NewLocalDomain(fakeLogger, fakeRecordSet, fakeTruncater))
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

			It("returns rcode success for A questions when there are no matching records", func() {
				m := &dns.Msg{}
				SetQuestion(m, nil, "my-instance.my-network.my-deployment.bosh.", dns.TypeA)

				discoveryHandler.ServeDNS(fakeWriter, m)
				message := fakeWriter.WriteMsgArgsForCall(0)
				Expect(message.Rcode).To(Equal(dns.RcodeSuccess))
				Expect(message.Authoritative).To(BeTrue())
				Expect(message.RecursionAvailable).To(BeTrue())
			})

			It("returns rcode success for AAAA questions when there are no matching records", func() {
				m := &dns.Msg{}
				SetQuestion(m, nil, "my-instance.my-network.my-deployment.bosh.", dns.TypeAAAA)

				discoveryHandler.ServeDNS(fakeWriter, m)
				message := fakeWriter.WriteMsgArgsForCall(0)
				Expect(message.Rcode).To(Equal(dns.RcodeSuccess))
				Expect(message.Authoritative).To(BeTrue())
				Expect(message.RecursionAvailable).To(BeTrue())
			})

			It("returns success with no data for all other types if host lookup succeeds", func() {
				fakeRecordSet.ResolveReturns([]string{"2601:0646:0102:0095:0000:0000:0000:0025", "123.123.123.123"}, nil)
				m := &dns.Msg{}
				SetQuestion(m, nil, "my-instance.my-network.my-deployment.bosh.", dns.TypePTR)

				discoveryHandler.ServeDNS(fakeWriter, m)
				message := fakeWriter.WriteMsgArgsForCall(0)
				Expect(message.Rcode).To(Equal(dns.RcodeSuccess))
				Expect(message.Authoritative).To(BeTrue())
				Expect(message.RecursionAvailable).To(BeTrue())
				Expect(message.Answer).To(BeEmpty())
			})

			It("returns name error for all other types if host lookup returns name error", func() {
				fakeRecordSet.ResolveReturns(nil, records.DomainError)
				m := &dns.Msg{}
				SetQuestion(m, nil, "my-instance.my-network.my-deployment.bosh.", dns.TypePTR)

				discoveryHandler.ServeDNS(fakeWriter, m)
				message := fakeWriter.WriteMsgArgsForCall(0)
				Expect(message.Rcode).To(Equal(dns.RcodeNameError))
				Expect(message.Authoritative).To(BeTrue())
				Expect(message.RecursionAvailable).To(BeTrue())
				Expect(message.Answer).To(BeEmpty())
			})

			It("returns success with no data for all other types if host lookup returns criteria error", func() {
				fakeRecordSet.ResolveReturns(nil, records.CriteriaError)
				m := &dns.Msg{}
				SetQuestion(m, nil, "my-instance.my-network.my-deployment.bosh.", dns.TypePTR)

				discoveryHandler.ServeDNS(fakeWriter, m)
				message := fakeWriter.WriteMsgArgsForCall(0)
				Expect(message.Rcode).To(Equal(dns.RcodeSuccess))
				Expect(message.Authoritative).To(BeTrue())
				Expect(message.RecursionAvailable).To(BeTrue())
				Expect(message.Answer).To(BeEmpty())
			})

			// q: A -> only A even if AAAA
			// q: AAAA -> only AAAA even if A
			// q: ANY -> both A and AAAA

			It("returns only A records (no AAAA records) when the queried for A records", func() {
				fakeRecordSet.ResolveReturns([]string{"2601:0646:0102:0095:0000:0000:0000:0025", "123.123.123.123"}, nil)

				m := &dns.Msg{}
				SetQuestion(m, nil, "my-instance.my-group.my-network.my-deployment.bosh.", dns.TypeA)

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
				SetQuestion(m, nil, "my-instance.my-group.my-network.my-deployment.bosh.", dns.TypeAAAA)

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
				ipv6ResolutionList := []string{"2601:0646:0102:0095:0000:0000:0000:0025"}
				ipv4ResolutionList := []string{"4.2.2.2"}

				fakeRecordSet.ResolveReturns(append(ipv6ResolutionList, ipv4ResolutionList...), nil)

				m := &dns.Msg{}
				SetQuestion(m, nil, "my-instance.my-group.my-network.my-deployment.bosh.", dns.TypeANY)

				discoveryHandler.ServeDNS(fakeWriter, m)
				responseMsg := fakeWriter.WriteMsgArgsForCall(0)

				Expect(responseMsg.Rcode).To(Equal(dns.RcodeSuccess))
				Expect(responseMsg.Authoritative).To(BeTrue())
				Expect(responseMsg.RecursionAvailable).To(BeTrue())
				Expect(responseMsg.Truncated).To(BeFalse())

				Expect(responseMsg.Answer).To(HaveLen(2))

				var ipv4Responses []dns.RR
				var ipv6Responses []dns.RR
				for _, a := range responseMsg.Answer {
					if a.Header().Rrtype == dns.TypeAAAA {
						ipv6Responses = append(ipv6Responses, a)
					} else if a.Header().Rrtype == dns.TypeA {
						ipv4Responses = append(ipv4Responses, a)
					} else {
						Fail(fmt.Sprintf("unexpected response type: %v", a))
					}
				}

				var ipv6AnswerStrings []string
				for _, a := range ipv6Responses {
					ipv6AnswerStrings = append(ipv6AnswerStrings, a.(*dns.AAAA).AAAA.String())

					Expect(a).To(BeAssignableToTypeOf(&dns.AAAA{}))
					header := a.Header()
					Expect(header.Rrtype).To(Equal(dns.TypeAAAA))
					Expect(header.Class).To(Equal(uint16(dns.ClassINET)))
					Expect(header.Ttl).To(Equal(uint32(0)))
				}
				Expect(len(ipv6AnswerStrings)).To(Equal(len(ipv6ResolutionList)))

				var canonicalIPv6ResolutionList []string
				for _, s := range ipv6AnswerStrings {
					canonicalIPv6ResolutionList = append(canonicalIPv6ResolutionList, net.ParseIP(s).String())
				}
				Expect(ipv6AnswerStrings).To(ConsistOf(canonicalIPv6ResolutionList))

				var ipv4AnswerStrings []string
				for _, a := range ipv4Responses {
					ipv4AnswerStrings = append(ipv4AnswerStrings, a.(*dns.A).A.String())

					Expect(a).To(BeAssignableToTypeOf(&dns.A{}))
					header := a.Header()
					Expect(header.Rrtype).To(Equal(dns.TypeA))
					Expect(header.Class).To(Equal(uint16(dns.ClassINET)))
					Expect(header.Ttl).To(Equal(uint32(0)))
				}
				Expect(ipv4AnswerStrings).To(ConsistOf(ipv4ResolutionList))

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
