package dnsresolver_test

import (
	"errors"
	"fmt"
	"net"

	"github.com/cloudfoundry/bosh-utils/logger/loggerfakes"
	"github.com/miekg/dns"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "bosh-dns/dns/internal/testhelpers/question_case_helpers"
	"bosh-dns/dns/server/internal/internalfakes"
	"bosh-dns/dns/server/records"
	. "bosh-dns/dns/server/records/dnsresolver"
	"bosh-dns/dns/server/records/dnsresolver/dnsresolverfakes"
)

var _ = Describe("LocalDomain", func() {
	Describe("Resolve", func() {
		var (
			fakeLogger    *loggerfakes.FakeLogger
			fakeWriter    *internalfakes.FakeResponseWriter
			fakeRecordSet *dnsresolverfakes.FakeRecordSet
			localDomain   LocalDomain
			fakeTruncater *dnsresolverfakes.FakeResponseTruncater
		)

		BeforeEach(func() {
			fakeLogger = &loggerfakes.FakeLogger{}
			fakeWriter = &internalfakes.FakeResponseWriter{}
			fakeRecordSet = &dnsresolverfakes.FakeRecordSet{}
			fakeTruncater = &dnsresolverfakes.FakeResponseTruncater{}

			fakeWriter.RemoteAddrReturns(&net.UDPAddr{})
			localDomain = NewLocalDomain(fakeLogger, fakeRecordSet, fakeTruncater)
		})

		It("returns responses from the question domain", func() {
			originalResolutionList := []string{"123.123.123.123", "123.123.123.124"}
			fakeRecordSet.ResolveStub = func(domain string) ([]string, error) {
				switch domain {
				case "*.group-1.network-name.deployment-name.bosh.":
					return originalResolutionList, nil
				case "instance-2.group-2.network-name.deployment-name.bosh.":
					return []string{"123.123.123.246"}, nil
				}

				return nil, errors.New("nope")
			}

			var casedQname string
			req := &dns.Msg{}
			SetQuestion(req, &casedQname, "*.group-1.network-name.deployment-name.bosh.", dns.TypeA)
			responseMsg := localDomain.Resolve(
				fakeWriter,
				req,
			)

			var answerStrings []string

			for _, a := range responseMsg.Answer {
				answerStrings = append(answerStrings, a.(*dns.A).A.String())
				Expect(a).To(BeAssignableToTypeOf(&dns.A{}))
				header := a.Header()
				Expect(header.Name).To(Equal(casedQname))
				Expect(header.Rrtype).To(Equal(dns.TypeA))
				Expect(header.Class).To(Equal(uint16(dns.ClassINET)))
				Expect(header.Ttl).To(Equal(uint32(0)))
			}
			Expect(answerStrings).To(ConsistOf(originalResolutionList))

			Expect(responseMsg.RecursionAvailable).To(BeTrue())
			Expect(responseMsg.Authoritative).To(BeTrue())
			Expect(responseMsg.Rcode).To(Equal(dns.RcodeSuccess))
		})

		It("shuffles the answers", func() {
			originalResolutionList := []string{"1.1.1.1", "2.2.2.2", "3.3.3.3", "4.4.4.4", "5.5.5.5"}

			fakeRecordSet.ResolveStub = func(domain string) ([]string, error) {
				switch domain {
				case "*.group-1.network-name.deployment-name.bosh.":
					return originalResolutionList, nil
				case "instance-2.group-2.network-name.deployment-name.bosh.":
					return []string{"123.123.123.246"}, nil
				}

				return nil, errors.New("nope")
			}

			localDomain = NewLocalDomain(fakeLogger, fakeRecordSet, fakeTruncater)

			req := &dns.Msg{}
			SetQuestion(req, nil, "*.group-1.network-name.deployment-name.bosh.", dns.TypeA)
			responseMsg := localDomain.Resolve(
				fakeWriter,
				req,
			)

			var answerStrings []string

			for _, a := range responseMsg.Answer {
				answerStrings = append(answerStrings, a.(*dns.A).A.String())
			}
			Expect(answerStrings).To(ConsistOf(originalResolutionList))
			Expect(responseMsg.Rcode).To(Equal(dns.RcodeSuccess))
		})

		Context("when there are too many records to fit into 512 bytes", func() {
			var (
				request *dns.Msg
			)

			BeforeEach(func() {
				fakeRecordSet.ResolveStub = func(domain string) ([]string, error) {
					Expect(domain).To(Equal("my-instance.my-group.my-network.my-deployment.bosh."))

					return []string{"123.123.123.123"}, nil
				}
				request = &dns.Msg{}
				SetQuestion(request, nil, "my-instance.my-group.my-network.my-deployment.bosh.", dns.TypeA)
			})

			It("truncates the response", func() {
				response := localDomain.Resolve(
					fakeWriter,
					request,
				)

				Expect(fakeTruncater.TruncateIfNeededCallCount()).To(Equal(1))
				writer, req, resp := fakeTruncater.TruncateIfNeededArgsForCall(0)
				Expect(writer).To(Equal(fakeWriter))
				Expect(req).To(Equal(request))
				Expect(resp).To(Equal(response))
			})
		})

		It("returns only A records (no AAAA records) when the queried for A records", func() {
			ipv6ResolutionList := []string{"2601:0646:0102:0095:0000:0000:0000:0025"}
			ipv4ResolutionList := []string{"123.123.123.123", "123.123.123.246"}
			fakeRecordSet.ResolveReturns(append(ipv6ResolutionList, ipv4ResolutionList...), nil)

			var casedQname string
			req := &dns.Msg{}
			SetQuestion(req, &casedQname, "instance-id-answer.group-1.network-name.deployment-name.bosh.", dns.TypeA)
			responseMsg := localDomain.Resolve(
				fakeWriter,
				req,
			)

			var answerStrings []string

			for _, a := range responseMsg.Answer {
				answerStrings = append(answerStrings, a.(*dns.A).A.String())

				Expect(a).To(BeAssignableToTypeOf(&dns.A{}))
				header := a.Header()
				Expect(header.Name).To(Equal(casedQname))
				Expect(header.Rrtype).To(Equal(dns.TypeA))
				Expect(header.Class).To(Equal(uint16(dns.ClassINET)))
				Expect(header.Ttl).To(Equal(uint32(0)))
			}
			Expect(answerStrings).To(ConsistOf(ipv4ResolutionList))

			Expect(responseMsg.Rcode).To(Equal(dns.RcodeSuccess))
		})

		It("returns only AAAA records (no A records) when the queried for AAAA records", func() {
			ipv6ResolutionList := []string{
				"2601:0646:0102:0095:0000:0000:0000:0026",
				"2601:0646:0102:0095:0000:0000:0000:0024",
			}
			ipv4ResolutionList := []string{"123.123.123.246"}

			fakeRecordSet.ResolveReturns(append(ipv6ResolutionList, ipv4ResolutionList...), nil)

			var casedQname string
			req := &dns.Msg{}
			SetQuestion(req, &casedQname, "instance-id-answer.group-1.network-name.deployment-name.bosh.", dns.TypeAAAA)
			responseMsg := localDomain.Resolve(
				fakeWriter,
				req,
			)

			var ipv6AnswerStrings []string
			for _, a := range responseMsg.Answer {
				ipv6AnswerStrings = append(ipv6AnswerStrings, a.(*dns.AAAA).AAAA.String())

				Expect(a).To(BeAssignableToTypeOf(&dns.AAAA{}))
				header := a.Header()
				Expect(header.Name).To(Equal(casedQname))
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

			Expect(responseMsg.Rcode).To(Equal(dns.RcodeSuccess))
		})

		It("returns both A and AAAA records when the queried for ANY records", func() {
			ipv6ResolutionList := []string{
				"2601:0646:0102:0095:0000:0000:0000:0026",
				"2601:0646:0102:0095:0000:0000:0000:0024",
			}
			ipv4ResolutionList := []string{"123.123.123.246"}

			fakeRecordSet.ResolveReturns(append(ipv6ResolutionList, ipv4ResolutionList...), nil)

			var casedQname string
			req := &dns.Msg{}
			SetQuestion(req, &casedQname, "instance-id-answer.group-1.network-name.deployment-name.bosh.", dns.TypeANY)
			responseMsg := localDomain.Resolve(
				fakeWriter,
				req,
			)

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
				Expect(header.Name).To(Equal(casedQname))
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
				Expect(header.Name).To(Equal(casedQname))
				Expect(header.Rrtype).To(Equal(dns.TypeA))
				Expect(header.Class).To(Equal(uint16(dns.ClassINET)))
				Expect(header.Ttl).To(Equal(uint32(0)))
			}
			Expect(ipv4AnswerStrings).To(ConsistOf(ipv4ResolutionList))

			Expect(responseMsg.Rcode).To(Equal(dns.RcodeSuccess))
		})

		Context("when loading the records returns criteria error", func() {
			var dnsReturnCode int

			BeforeEach(func() {
				fakeRecordSet.ResolveReturns(nil, records.CriteriaError)

				req := &dns.Msg{}
				SetQuestion(req, nil, "instance-id-answer.group-1.network-name.deployment-name.bosh.", dns.TypeA)
				responseMsg := localDomain.Resolve(
					fakeWriter,
					req,
				)
				dnsReturnCode = responseMsg.Rcode
			})

			It("returns rcode format error", func() {
				Expect(dnsReturnCode).To(Equal(dns.RcodeFormatError))
			})

			It("logs the error", func() {
				Expect(fakeLogger.DebugCallCount()).To(Equal(2))
				tag, msg, args := fakeLogger.DebugArgsForCall(1)
				Expect(tag).To(Equal("LocalDomain"))
				Expect(msg).To(Equal("failed to get ip addresses: %v"))
				Expect(args[0]).To(MatchError(records.CriteriaError))
			})
		})

		Context("when loading the records returns domain error", func() {
			var dnsReturnCode int

			BeforeEach(func() {
				fakeRecordSet.ResolveReturns(nil, records.DomainError)

				req := &dns.Msg{}
				SetQuestion(req, nil, "instance-id-answer.group-1.network-name.deployment-name.bosh.", dns.TypeA)
				responseMsg := localDomain.Resolve(
					fakeWriter,
					req,
				)
				dnsReturnCode = responseMsg.Rcode
			})

			It("returns rcode name error", func() {
				Expect(dnsReturnCode).To(Equal(dns.RcodeNameError))
			})

			It("logs the error", func() {
				Expect(fakeLogger.DebugCallCount()).To(Equal(2))
				tag, msg, args := fakeLogger.DebugArgsForCall(1)
				Expect(tag).To(Equal("LocalDomain"))
				Expect(msg).To(Equal("failed to get ip addresses: %v"))
				Expect(args[0]).To(MatchError(records.DomainError))
			})
		})

		Context("when loading the records returns unexpected error", func() {
			var dnsReturnCode int

			BeforeEach(func() {
				fakeRecordSet.ResolveReturns(nil, errors.New("i screwed up"))

				req := &dns.Msg{}
				SetQuestion(req, nil, "instance-id-answer.group-1.network-name.deployment-name.bosh.", dns.TypeA)
				responseMsg := localDomain.Resolve(
					fakeWriter,
					req,
				)
				dnsReturnCode = responseMsg.Rcode
			})

			It("returns rcode server failure", func() {
				Expect(dnsReturnCode).To(Equal(dns.RcodeServerFailure))
			})

			It("logs the error", func() {
				Expect(fakeLogger.DebugCallCount()).To(Equal(2))
				tag, msg, args := fakeLogger.DebugArgsForCall(1)
				Expect(tag).To(Equal("LocalDomain"))
				Expect(msg).To(Equal("failed to get ip addresses: %v"))
				Expect(args[0]).To(MatchError("i screwed up"))
			})
		})
	})
})
