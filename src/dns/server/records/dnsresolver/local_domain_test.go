package dnsresolver_test

import (
	. "github.com/cloudfoundry/dns-release/src/dns/server/records/dnsresolver"

	"github.com/cloudfoundry/bosh-utils/logger/loggerfakes"
	"github.com/cloudfoundry/dns-release/src/dns/server/records"
	"github.com/cloudfoundry/dns-release/src/dns/server/records/dnsresolver/dnsresolverfakes"
	"github.com/miekg/dns"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"errors"
)

var _ = Describe("LocalDomain", func() {
	Describe("Resolve", func() {
		var (
			fakeLogger        *loggerfakes.FakeLogger
			fakeRecordSetRepo *dnsresolverfakes.FakeRecordSetRepo
			localDomain       LocalDomain
		)

		BeforeEach(func() {
			fakeLogger = &loggerfakes.FakeLogger{}
			fakeRecordSetRepo = &dnsresolverfakes.FakeRecordSetRepo{}
			localDomain = NewLocalDomain(fakeLogger, fakeRecordSetRepo)
		})

		It("returns A records based off of the records data", func() {
			recordSet := records.RecordSet{
				Records: []records.Record{
					{
						Id:         "my-instance",
						Group:      "my-group",
						Network:    "my-network",
						Deployment: "my-deployment",
						Ip:         "123.123.123.123",
					},
					{
						Id:         "my-instance",
						Group:      "my-group",
						Network:    "my-network",
						Deployment: "my-deployment",
						Ip:         "123.123.123.246",
					},
				},
			}
			fakeRecordSetRepo.GetReturns(recordSet, nil)

			answers, dnsReturnCode := localDomain.Resolve(
				"my-instance-answer.my-group.my-network.my-deployment.bosh.",
				"my-instance.my-group.my-network.my-deployment.bosh.",
			)

			Expect(answers).To(HaveLen(2))

			answer := answers[0]
			header := answer.Header()
			Expect(header.Name).To(Equal("my-instance-answer.my-group.my-network.my-deployment.bosh."))
			Expect(header.Rrtype).To(Equal(dns.TypeA))
			Expect(header.Class).To(Equal(uint16(dns.ClassINET)))
			Expect(header.Ttl).To(Equal(uint32(0)))
			Expect(answer).To(BeAssignableToTypeOf(&dns.A{}))
			Expect(answer.(*dns.A).A.String()).To(Equal("123.123.123.123"))

			answer = answers[1]
			header = answer.Header()
			Expect(header.Name).To(Equal("my-instance-answer.my-group.my-network.my-deployment.bosh."))
			Expect(header.Rrtype).To(Equal(dns.TypeA))
			Expect(header.Class).To(Equal(uint16(dns.ClassINET)))
			Expect(header.Ttl).To(Equal(uint32(0)))
			Expect(answer).To(BeAssignableToTypeOf(&dns.A{}))
			Expect(answer.(*dns.A).A.String()).To(Equal("123.123.123.246"))

			Expect(dnsReturnCode).To(Equal(dns.RcodeSuccess))
		})

		Context("when loading the records returns an error", func() {
			var dnsReturnCode int

			BeforeEach(func() {
				recordSet := records.RecordSet{}
				fakeRecordSetRepo.GetReturns(recordSet, errors.New("i screwed up"))
				_, dnsReturnCode = localDomain.Resolve(
					"my-instance-answer.my-group.my-network.my-deployment.bosh.",
					"my-instance.my-group.my-network.my-deployment.bosh.",
				)
			})

			It("returns rcode server failure", func() {
				Expect(dnsReturnCode).To(Equal(dns.RcodeServerFailure))
			})

			It("logs the error", func() {
				Expect(fakeLogger.ErrorCallCount()).To(Equal(1))
				tag, msg, args := fakeLogger.ErrorArgsForCall(0)
				Expect(tag).To(Equal("LocalDomain"))
				Expect(msg).To(Equal("failed to get ip addresses: %v"))
				Expect(args[0]).To(MatchError("i screwed up"))
			})
		})

		Context("when parsing the query returns an error", func() {
			var dnsReturnCode int

			BeforeEach(func() {
				recordSet := records.RecordSet{}
				fakeRecordSetRepo.GetReturns(recordSet, nil)
				question := "q-&^$*^*#^.my-group.my-network.my-deployment.bosh."
				_, dnsReturnCode = localDomain.Resolve(question, question)
			})

			It("returns rcode format error", func() {
				Expect(dnsReturnCode).To(Equal(dns.RcodeFormatError))
			})

			It("logs the error", func() {
				Expect(fakeLogger.ErrorCallCount()).To(Equal(1))
				tag, msg, _ := fakeLogger.ErrorArgsForCall(0)
				Expect(tag).To(Equal("LocalDomain"))
				Expect(msg).To(Equal("failed to decode query: %v"))
			})
		})
	})
})
