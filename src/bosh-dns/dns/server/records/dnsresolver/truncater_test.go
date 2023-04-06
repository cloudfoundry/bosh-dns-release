package dnsresolver_test

import (
	"net"

	"github.com/miekg/dns"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "bosh-dns/dns/internal/testhelpers/question_case_helpers"
	"bosh-dns/dns/server/internal/internalfakes"
	"bosh-dns/dns/server/records/dnsresolver"
)

var _ = Describe("TruncateIfNeeded", func() {

	var (
		subject  dnsresolver.ResponseTruncater
		request  *dns.Msg
		response *dns.Msg
		writer   *internalfakes.FakeResponseWriter
	)

	BeforeEach(func() {
		subject = dnsresolver.NewResponseTruncater()
		writer = &internalfakes.FakeResponseWriter{}
	})

	Context("Request is TCP", func() {
		BeforeEach(func() {
			request = createRequest(0)
			writer.RemoteAddrReturns(&net.TCPAddr{})
		})
		Context("Response is larger than 512 bytes", func() {
			BeforeEach(func() {
				response = createResponse(513)
			})
			It("Should not truncate", func() {
				subject.TruncateIfNeeded(writer, request, response)

				Expect(response.Truncated).To(BeFalse())
				Expect(response.IsEdns0()).To(BeNil())
				Expect(response.Len()).To(BeNumerically(">=", 513))
			})
		})
	})

	Context("Request is UDP", func() {
		BeforeEach(func() {
			writer.RemoteAddrReturns(&net.UDPAddr{})
		})
		Context("Request has EDNS record", func() {
			BeforeEach(func() {
				request = createRequest(1024)
			})
			Context("Response is smaller than EDNS buffer", func() {
				BeforeEach(func() {
					response = createResponse(600)
				})
				It("Should not truncate", func() {
					subject.TruncateIfNeeded(writer, request, response)

					Expect(response.Truncated).To(BeFalse())
					Expect(response.IsEdns0()).ToNot(BeNil())
					Expect(response.Len()).To(BeNumerically(">=", 600))
				})
			})
			Context("Response is larger than EDNS buffer", func() {
				BeforeEach(func() {
					response = createResponse(1025)
				})
				It("Should truncate", func() {
					subject.TruncateIfNeeded(writer, request, response)

					Expect(response.Truncated).To(BeTrue())
					Expect(response.Compress).To(BeTrue())
					Expect(response.IsEdns0()).ToNot(BeNil())
					Expect(response.Len()).To(BeNumerically("<=", 1024))
				})
			})
		})

		Context("Request does not have EDNS record", func() {
			BeforeEach(func() {
				request = createRequest(0)
			})
			Context("Response is smaller than 512 bytes", func() {
				BeforeEach(func() {
					response = createResponse(100)
				})
				It("Should not truncate", func() {
					subject.TruncateIfNeeded(writer, request, response)

					Expect(response.Truncated).To(BeFalse())
					Expect(response.IsEdns0()).To(BeNil())
					Expect(response.Len()).To(BeNumerically(">=", 100))
				})
			})
			Context("Response is larger than 512 bytes", func() {
				BeforeEach(func() {
					response = createResponse(513)
				})
				It("Should truncate", func() {
					subject.TruncateIfNeeded(writer, request, response)

					Expect(response.Truncated).To(BeTrue())
					Expect(response.Compress).To(BeTrue())
					Expect(response.IsEdns0()).To(BeNil())
					Expect(response.Len()).To(BeNumerically("<=", 512))
				})
			})
			Context("Response is already truncated", func() {
				BeforeEach(func() {
					response = createResponse(256)
					response.Truncated = true
					response.Compress = true
				})
				It("Should not modify response", func() {
					subject.TruncateIfNeeded(writer, request, response)

					expected := createResponse(256)
					Expect(response.Truncated).To(BeTrue())
					Expect(response.Compress).To(BeTrue())
					Expect(response.IsEdns0()).To(BeNil())
					Expect(response.Len()).To(Equal(expected.Len()))
				})
			})
			Context("Response is already truncated but client has smaller buffer", func() {
				BeforeEach(func() {
					response = createResponse(1024)
					response.Truncated = true
					response.Compress = true
				})
				It("Should truncate further", func() {
					subject.TruncateIfNeeded(writer, request, response)

					Expect(response.Truncated).To(BeTrue())
					Expect(response.Compress).To(BeTrue())
					Expect(response.IsEdns0()).To(BeNil())
					Expect(response.Len()).To(BeNumerically("<=", 512))
				})
			})
		})
	})
})

func createRequest(ednsBuffer uint16) *dns.Msg {
	m := &dns.Msg{}
	SetQuestion(m, nil, "my-instance.my-group.my-network.my-deployment.bosh.", dns.TypeANY)
	if ednsBuffer > 0 {
		m.SetEdns0(ednsBuffer, false)
	}
	return m
}

func createResponse(minSize int) *dns.Msg {
	m := &dns.Msg{
		Answer: []dns.RR{},
	}
	SetQuestion(m, nil, "my-instance.my-group.my-network.my-deployment.bosh.", dns.TypeANY)
	m.SetRcode(m, dns.RcodeSuccess)

	m.Compress = true // ensure compression won't avoid truncation
	for m.Len() < minSize {
		m.Answer = append(m.Answer, &dns.AAAA{AAAA: net.ParseIP("1111:2222:3333:4444:5555:6666:7777:8888")})
	}
	m.Compress = false // test subject should set compress when necessary
	return m
}
