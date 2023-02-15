package handlers_test

import (
	"errors"

	"bosh-dns/dns/server/handlers"
	"bosh-dns/dns/server/internal/internalfakes"

	"github.com/miekg/dns"

	"net"

	. "bosh-dns/dns/internal/testhelpers/question_case_helpers"

	"github.com/cloudfoundry/bosh-utils/logger/loggerfakes"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("UpcheckHandler", func() {
	var (
		fakeLogger     *loggerfakes.FakeLogger
		upcheckHandler handlers.UpcheckHandler
		fakeWriter     *internalfakes.FakeResponseWriter
	)

	BeforeEach(func() {
		fakeLogger = &loggerfakes.FakeLogger{}
		upcheckHandler = handlers.NewUpcheckHandler(fakeLogger)
		fakeWriter = &internalfakes.FakeResponseWriter{}
	})

	Describe("ServeDNS", func() {
		Context("when ANY record", func() {
			It("returns success rcode", func() {
				var casedQname string
				m := &dns.Msg{}
				SetQuestion(m, &casedQname, "upcheck.bosh-dns.", dns.TypeANY)

				upcheckHandler.ServeDNS(fakeWriter, m)
				message := fakeWriter.WriteMsgArgsForCall(0)
				Expect(message.Rcode).To(Equal(dns.RcodeSuccess))
				Expect(message.Authoritative).To(Equal(true))
				Expect(message.RecursionAvailable).To(Equal(true))
				Expect(len(message.Answer)).To(Equal(2))
				Expect(message.Answer[0]).To(Equal(&dns.A{
					Hdr: dns.RR_Header{
						Name:   casedQname,
						Rrtype: dns.TypeA,
						Class:  dns.ClassINET,
						Ttl:    0,
					},
					A: net.IPv4(127, 0, 0, 1),
				}))
				Expect(message.Answer[1]).To(Equal(&dns.AAAA{
					Hdr: dns.RR_Header{
						Name:   casedQname,
						Rrtype: dns.TypeAAAA,
						Class:  dns.ClassINET,
						Ttl:    0,
					},
					AAAA: net.ParseIP("::1"),
				}))
			})
		})

		Context("when A record", func() {
			It("returns success rcode", func() {
				var casedQname string
				m := &dns.Msg{}
				SetQuestion(m, &casedQname, "upcheck.bosh-dns.", dns.TypeA)

				upcheckHandler.ServeDNS(fakeWriter, m)
				message := fakeWriter.WriteMsgArgsForCall(0)
				Expect(message.Rcode).To(Equal(dns.RcodeSuccess))
				Expect(message.Authoritative).To(Equal(true))
				Expect(message.RecursionAvailable).To(Equal(true))
				Expect(len(message.Answer)).To(Equal(1))
				Expect(message.Answer[0]).To(Equal(&dns.A{
					Hdr: dns.RR_Header{
						Name:   casedQname,
						Rrtype: dns.TypeA,
						Class:  dns.ClassINET,
						Ttl:    0,
					},
					A: net.IPv4(127, 0, 0, 1),
				}))
			})
		})

		Context("when AAAA record", func() {
			It("returns success rcode", func() {
				var casedQname string
				m := &dns.Msg{}
				SetQuestion(m, &casedQname, "upcheck.bosh-dns.", dns.TypeAAAA)

				upcheckHandler.ServeDNS(fakeWriter, m)
				message := fakeWriter.WriteMsgArgsForCall(0)
				Expect(message.Rcode).To(Equal(dns.RcodeSuccess))
				Expect(message.Authoritative).To(Equal(true))
				Expect(message.RecursionAvailable).To(Equal(true))
				Expect(len(message.Answer)).To(Equal(1))
				Expect(message.Answer[0]).To(Equal(&dns.AAAA{
					Hdr: dns.RR_Header{
						Name:   casedQname,
						Rrtype: dns.TypeAAAA,
						Class:  dns.ClassINET,
						Ttl:    0,
					},
					AAAA: net.ParseIP("::1"),
				}))
			})
		})

		Context("when not A, AAAA, or ANY record", func() {
			It("returns success rcode", func() {
				m := &dns.Msg{}
				SetQuestion(m, nil, "upcheck.bosh-dns.", dns.TypeMX)

				upcheckHandler.ServeDNS(fakeWriter, m)
				message := fakeWriter.WriteMsgArgsForCall(0)
				Expect(message.Rcode).To(Equal(dns.RcodeSuccess))
				Expect(message.Authoritative).To(Equal(true))
				Expect(message.RecursionAvailable).To(Equal(true))
				Expect(len(message.Answer)).To(Equal(0))
			})
		})

		Context("when no question", func() {
			It("returns something", func() {
				m := &dns.Msg{}
				m.Id = 3000

				upcheckHandler.ServeDNS(fakeWriter, m)
				message := fakeWriter.WriteMsgArgsForCall(0)
				Expect(message.Id).To(Equal(m.Id))
				Expect(message.Rcode).To(Equal(dns.RcodeSuccess))
				Expect(message.Authoritative).To(Equal(true))
				Expect(message.RecursionAvailable).To(Equal(true))
				Expect(len(message.Answer)).To(Equal(0))
			})
		})

		Context("when the message fails to write", func() {
			It("logs the error", func() {
				fakeWriter.WriteMsgReturns(errors.New("failed to write message"))

				m := &dns.Msg{}
				SetQuestion(m, nil, "upcheck.bosh-dns.", dns.TypeANY)
				upcheckHandler.ServeDNS(fakeWriter, m)

				Expect(fakeLogger.ErrorCallCount()).To(Equal(1))
				tag, msg, _ := fakeLogger.ErrorArgsForCall(0)
				Expect(tag).To(Equal("UpcheckHandler"))
				Expect(msg).To(Equal("failed to write message"))
			})
		})
	})
})
