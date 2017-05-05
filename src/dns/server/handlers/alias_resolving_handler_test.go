package handlers_test

import (
	. "github.com/cloudfoundry/dns-release/src/dns/server/handlers"

	"errors"
	"github.com/cloudfoundry/bosh-utils/logger/loggerfakes"
	"github.com/cloudfoundry/dns-release/src/dns/server/aliases"
	"github.com/cloudfoundry/dns-release/src/dns/server/handlers/handlersfakes"
	"github.com/cloudfoundry/dns-release/src/dns/server/handlers/internal/internalfakes"
	"github.com/miekg/dns"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"net"
	"github.com/cloudfoundry/dns-release/src/dns/server/records/dnsresolver"
)

var _ = Describe("AliasResolvingHandler", func() {
	var (
		handler            AliasResolvingHandler
		childHandler       dns.Handler
		dispatchedRequest  dns.Msg
		fakeWriter         *internalfakes.FakeResponseWriter
		fakeDomainResolver *handlersfakes.FakeDomainResolver
		fakeLogger         *loggerfakes.FakeLogger
	)

	BeforeEach(func() {
		fakeDomainResolver = &handlersfakes.FakeDomainResolver{}
		fakeWriter = &internalfakes.FakeResponseWriter{}
		fakeLogger = &loggerfakes.FakeLogger{}

		childHandler = dns.HandlerFunc(func(resp dns.ResponseWriter, req *dns.Msg) {
			dispatchedRequest = *req

			m := &dns.Msg{}
			m.Authoritative = true
			m.RecursionAvailable = false
			m.SetRcode(req, dns.RcodeServerFailure)

			Expect(resp.WriteMsg(m)).To(Succeed())
		})

		config := aliases.MustNewConfigFromMap(map[string][]string{
			"alias1":   {"a1_domain1", "a1_domain2"},
			"alias2":   {"a2_domain1"},
			"_.alias2": {"_.a2_domain1", "_.b2_domain1"},
		})

		var err error
		handler, err = NewAliasResolvingHandler(childHandler, config, fakeDomainResolver, fakeLogger)
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("ServeDNS", func() {
		Context("when the message contains no aliased names", func() {
			It("passes the message through as-is", func() {
				m := dns.Msg{}
				m.SetQuestion("anything", dns.TypeA)
				m.SetEdns0(2048, false)

				handler.ServeDNS(fakeWriter, &m)

				Expect(dispatchedRequest).To(Equal(m))
				opt := dispatchedRequest.IsEdns0()
				Expect(opt.UDPSize()).To(Equal(uint16(2048)))

				response := fakeWriter.WriteMsgArgsForCall(0)
				Expect(response.Rcode).To(Equal(dns.RcodeServerFailure))
				Expect(response.Authoritative).To(Equal(true))
				Expect(response.RecursionAvailable).To(Equal(false))
			})
		})

		Context("when the message contains a underscore style alias", func() {
			It("translates the question preserving the capture", func() {
				fakeResponse := &dns.Msg{
					Answer: []dns.RR{
						&dns.A{A: net.IPv4(123, 123, 123, 123)},
						&dns.A{A: net.IPv4(123, 123, 123, 246)},
					},
				}
				requestMsg := &dns.Msg{
					Question: []dns.Question{
						{
							Name:   "5.alias2.",
							Qtype:  dns.TypeA,
							Qclass: 1,
						},
					},
				}
				fakeDomainResolver.ResolveAnswerStub = func(answerName string, resolutionNames []string, protocol dnsresolver.Protocol, actualRequestMsg *dns.Msg) *dns.Msg {
					Expect(resolutionNames).To(ConsistOf("5.a2_domain1.", "5.b2_domain1."))
					Expect(actualRequestMsg).To(Equal(requestMsg))
					fakeResponse.SetRcode(requestMsg, dns.RcodeSuccess)
					return fakeResponse
				}

				handler.ServeDNS(fakeWriter, requestMsg)

				message := fakeWriter.WriteMsgArgsForCall(0)
				Expect(message).To(Equal(fakeResponse))
			})

			It("returns a non successful return code when a resoution fails", func() {
				fakeResponse := &dns.Msg{}
				fakeDomainResolver.ResolveAnswerStub = func(answerName string, resolutionNames []string, protocol dnsresolver.Protocol, requestMsg *dns.Msg) *dns.Msg {
					Expect(resolutionNames).To(ConsistOf("5.a2_domain1.", "5.b2_domain1."))
					fakeResponse.SetRcode(requestMsg, dns.RcodeServerFailure)
					return fakeResponse
				}

				m := dns.Msg{}
				originalQuestions := []dns.Question{
					{
						Name:   "5.alias2.",
						Qtype:  dns.TypeA,
						Qclass: 1,
					},
				}
				m.Question = originalQuestions

				handler.ServeDNS(fakeWriter, &m)
				message := fakeWriter.WriteMsgArgsForCall(0)
				Expect(message).To(Equal(fakeResponse))
				Expect(message.Rcode).To(Equal(dns.RcodeServerFailure))
			})

			It("logs if the response cannot be written", func() {
				fakeWriter.WriteMsgReturns(errors.New("failed to write message"))

				m := &dns.Msg{}
				m.SetQuestion("5.alias2.", dns.TypeANY)

				handler.ServeDNS(fakeWriter, m)

				Expect(fakeLogger.ErrorCallCount()).To(Equal(1))
				tag, msg, args := fakeLogger.ErrorArgsForCall(0)
				Expect(tag).To(Equal("AliasResolvingHandler"))
				Expect(msg).To(Equal("error writing response %s"))
				Expect(args).To(ContainElement("failed to write message"))
			})
		})

		Context("when the message contains an alias", func() {
			It("resolves the alias before delegating", func() {
				fakeResponse := &dns.Msg{
					Answer: []dns.RR{
						&dns.A{A: net.IPv4(123, 123, 123, 123)},
					},
				}

				fakeDomainResolver.ResolveAnswerStub = func(answerDomain string, quetsionDomains []string, requestMsg *dns.Msg) *dns.Msg {
					fakeResponse.SetRcode(requestMsg, dns.RcodeSuccess)
					return fakeResponse
				}

				m := dns.Msg{
					Question: []dns.Question{
						{
							Name:   "alias2.",
							Qtype:  dns.TypeA,
							Qclass: 1,
						},
					},
				}

				handler.ServeDNS(fakeWriter, &m)

				message := fakeWriter.WriteMsgArgsForCall(0)
				Expect(message).To(Equal(fakeResponse))
			})
		})
	})

	Describe("NewAliasResolvingHandler", func() {
		It("errors if given a config with recursing aliases", func() {
			config := aliases.MustNewConfigFromMap(map[string][]string{
				"alias1": {"a1_domain1", "alias2"},
				"alias2": {"a2_domain1"},
			})

			_, err := NewAliasResolvingHandler(childHandler, config, fakeDomainResolver, fakeLogger)
			Expect(err).To(HaveOccurred())
		})
	})
})
