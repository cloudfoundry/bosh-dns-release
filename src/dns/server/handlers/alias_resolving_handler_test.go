package handlers_test

import (
	. "github.com/cloudfoundry/dns-release/src/dns/server/handlers"

	"github.com/cloudfoundry/dns-release/src/dns/server/aliases"
	"github.com/cloudfoundry/dns-release/src/dns/server/handlers/internal/internalfakes"
	"github.com/miekg/dns"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("AliasResolvingHandler", func() {
	var (
		handler           AliasResolvingHandler
		childHandler      dns.Handler
		dispatchedRequest dns.Msg
		fakeWriter        *internalfakes.FakeResponseWriter
	)

	BeforeEach(func() {
		fakeWriter = &internalfakes.FakeResponseWriter{}

		childHandler = dns.HandlerFunc(func(resp dns.ResponseWriter, req *dns.Msg) {
			dispatchedRequest = *req

			m := &dns.Msg{}
			m.Authoritative = true
			m.RecursionAvailable = false
			m.SetRcode(req, dns.RcodeServerFailure)

			Expect(resp.WriteMsg(m)).To(Succeed())
		})

		config := aliases.Config{
			"alias1": {"a1_domain1", "a1_domain2"},
			"alias2": {"a2_domain1"},
		}

		var err error
		handler, err = NewAliasResolvingHandler(childHandler, config)
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("ServeDNS", func() {
		Context("when the message contains no aliased names", func() {
			It("passes the message through as-is", func() {
				m := dns.Msg{}
				m.SetQuestion("anything", dns.TypeA)

				handler.ServeDNS(fakeWriter, &m)

				Expect(dispatchedRequest).To(Equal(m))

				message := fakeWriter.WriteMsgArgsForCall(0)
				Expect(message.Rcode).To(Equal(dns.RcodeServerFailure))
				Expect(message.Authoritative).To(Equal(true))
				Expect(message.RecursionAvailable).To(Equal(false))
			})
		})

		Context("when the message contains an alias", func() {
			It("resolves the alias before delegating", func() {
				m := dns.Msg{}
				originalQuestions := []dns.Question{
					{
						Name:   "alias2",
						Qtype:  dns.TypeAAAA,
						Qclass: 1,
					},
				}
				m.Question = originalQuestions

				handler.ServeDNS(fakeWriter, &m)

				expectedResolution := dns.Msg{
					Question: []dns.Question{{
						Name:   "a2_domain1",
						Qtype:  m.Question[0].Qtype,
						Qclass: m.Question[0].Qclass,
					}},
					MsgHdr: m.MsgHdr,
				}

				Expect(dispatchedRequest).To(Equal(expectedResolution))

				message := fakeWriter.WriteMsgArgsForCall(0)
				Expect(message.Rcode).To(Equal(dns.RcodeServerFailure))
				Expect(message.Authoritative).To(Equal(true))
				Expect(message.RecursionAvailable).To(Equal(false))
				Expect(message.Question).To(Equal(originalQuestions))
			})

			Context("when the alias refers to multiple resolvable addresses", func() {
				It("merges all the relevant questions into one message", func() {
					m := dns.Msg{}
					m.SetQuestion("alias1", dns.TypeANY)

					handler.ServeDNS(fakeWriter, &m)

					expectedResolution := dns.Msg{
						MsgHdr: m.MsgHdr,
						Question: []dns.Question{
							{Name: "a1_domain1", Qtype: dns.TypeANY, Qclass: m.Question[0].Qclass},
							{Name: "a1_domain2", Qtype: dns.TypeANY, Qclass: m.Question[0].Qclass},
						},
					}

					Expect(dispatchedRequest).To(Equal(expectedResolution))

					message := fakeWriter.WriteMsgArgsForCall(0)
					Expect(message.Rcode).To(Equal(dns.RcodeServerFailure))
					Expect(message.Authoritative).To(Equal(true))
					Expect(message.RecursionAvailable).To(Equal(false))
				})
			})
		})
	})

	Describe("NewAliasResolvingHandler", func() {
		It("errors if given a config with recursing aliases", func() {
			config := aliases.Config{
				"alias1": {"a1_domain1", "alias2"},
				"alias2": {"a2_domain1"},
			}

			_, err := NewAliasResolvingHandler(childHandler, config)
			Expect(err).To(HaveOccurred())
		})
	})
})
