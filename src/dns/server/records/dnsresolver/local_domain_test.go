package dnsresolver_test

import (
	. "github.com/cloudfoundry/dns-release/src/dns/server/records/dnsresolver"

	"github.com/cloudfoundry/bosh-utils/logger/loggerfakes"
	"github.com/cloudfoundry/dns-release/src/dns/server/records"
	"github.com/miekg/dns"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"errors"
	"github.com/cloudfoundry/dns-release/src/dns/server/records/dnsresolver/dnsresolverfakes"
)

var _ = Describe("LocalDomain", func() {
	Describe("Resolve", func() {
		var (
			fakeLogger        *loggerfakes.FakeLogger
			fakeRecordSetRepo *dnsresolverfakes.FakeRecordSetRepo
			localDomain       LocalDomain
			fakeShuffler      *dnsresolverfakes.FakeAnswerShuffler
		)

		BeforeEach(func() {
			fakeLogger = &loggerfakes.FakeLogger{}
			fakeRecordSetRepo = &dnsresolverfakes.FakeRecordSetRepo{}
			fakeShuffler = &dnsresolverfakes.FakeAnswerShuffler{}
			fakeShuffler.ShuffleStub = func(input []dns.RR) []dns.RR {
				return input
			}
			localDomain = NewLocalDomain(fakeLogger, fakeRecordSetRepo, fakeShuffler)
		})

		It("returns responses from all the question domains", func() {
			recordSet := records.RecordSet{
				Records: []records.Record{
					{
						Id:         "instance-1",
						Group:      "group-1",
						Network:    "network-name",
						Deployment: "deployment-name",
						Ip:         "123.123.123.123",
					},
					{
						Id:         "instance-2",
						Group:      "group-2",
						Network:    "network-name",
						Deployment: "deployment-name",
						Ip:         "123.123.123.246",
					},
				},
			}
			fakeRecordSetRepo.GetReturns(recordSet, nil)

			req := &dns.Msg{}
			req.SetQuestion("answer.bosh.", dns.TypeA)
			responseMsg := localDomain.ResolveAnswer(
				"answer.bosh.",
				[]string{
					"instance-1.group-1.network-name.deployment-name.bosh.",
					"instance-2.group-2.network-name.deployment-name.bosh.",
				},
				UDP,
				req,
			)

			answers := responseMsg.Answer
			Expect(answers).To(HaveLen(2))

			answer := answers[0]
			header := answer.Header()
			Expect(header.Name).To(Equal("answer.bosh."))
			Expect(header.Rrtype).To(Equal(dns.TypeA))
			Expect(header.Class).To(Equal(uint16(dns.ClassINET)))
			Expect(header.Ttl).To(Equal(uint32(0)))
			Expect(answer).To(BeAssignableToTypeOf(&dns.A{}))
			Expect(answer.(*dns.A).A.String()).To(Equal("123.123.123.123"))

			answer = answers[1]
			header = answer.Header()
			Expect(header.Name).To(Equal("answer.bosh."))
			Expect(header.Rrtype).To(Equal(dns.TypeA))
			Expect(header.Class).To(Equal(uint16(dns.ClassINET)))
			Expect(header.Ttl).To(Equal(uint32(0)))
			Expect(answer).To(BeAssignableToTypeOf(&dns.A{}))
			Expect(answer.(*dns.A).A.String()).To(Equal("123.123.123.246"))

			Expect(responseMsg.Rcode).To(Equal(dns.RcodeSuccess))
		})

		It("shuffles the answers", func() {
			recordSet := records.RecordSet{
				Records: []records.Record{
					{
						Id:         "instance-1",
						Group:      "group-1",
						Network:    "network-name",
						Deployment: "deployment-name",
						Ip:         "123.123.123.123",
					},
					{
						Id:         "instance-2",
						Group:      "group-1",
						Network:    "network-name",
						Deployment: "deployment-name",
						Ip:         "123.123.123.124",
					},
				},
			}
			fakeRecordSetRepo.GetReturns(recordSet, nil)
			fakeShuffler.ShuffleStub = func(input []dns.RR) []dns.RR {
				return []dns.RR{input[1], input[0]}
			}
			localDomain = NewLocalDomain(fakeLogger, fakeRecordSetRepo, fakeShuffler)

			req := &dns.Msg{}
			req.SetQuestion("ignored", dns.TypeA)
			responseMsg := localDomain.ResolveAnswer(
				"answer.bosh.",
				[]string{
					"instance-1.group-1.network-name.deployment-name.bosh.",
					"instance-2.group-1.network-name.deployment-name.bosh.",
				},
				UDP,
				req,
			)

			answers := responseMsg.Answer
			Expect(answers[0].(*dns.A).A.String()).To(Equal("123.123.123.124"))
			Expect(answers[1].(*dns.A).A.String()).To(Equal("123.123.123.123"))
			Expect(responseMsg.Rcode).To(Equal(dns.RcodeSuccess))

		})

		Context("when there are too many records to fit into 512 bytes", func() {
			var (
				recordSet records.RecordSet
				req       *dns.Msg
			)

			BeforeEach(func() {
				recordSet = records.RecordSet{
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
							Ip:         "127.0.0.1",
						},
						{
							Id:         "my-instance",
							Group:      "my-group",
							Network:    "my-network",
							Deployment: "my-deployment",
							Ip:         "127.0.0.2",
						},
						{
							Id:         "my-instance",
							Group:      "my-group",
							Network:    "my-network",
							Deployment: "my-deployment",
							Ip:         "127.0.0.3",
						},
						{
							Id:         "my-instance",
							Group:      "my-group",
							Network:    "my-network",
							Deployment: "my-deployment",
							Ip:         "127.0.0.4",
						},
						{
							Id:         "my-instance",
							Group:      "my-group",
							Network:    "my-network",
							Deployment: "my-deployment",
							Ip:         "127.0.0.5",
						},
						{
							Id:         "my-instance",
							Group:      "my-group",
							Network:    "my-network",
							Deployment: "my-deployment",
							Ip:         "127.0.0.6",
						},
					},
				}

				fakeRecordSetRepo.GetReturns(recordSet, nil)
				req = &dns.Msg{}
				req.SetQuestion("my-instance.my-group.my-network.my-deployment.bosh.", dns.TypeA)
			})

			Context("when the request is udp", func() {
				It("truncates the response", func() {
					responseMsg := localDomain.ResolveAnswer(
						"my-instance.my-group.my-network.my-deployment.bosh.",
						[]string{"my-instance.my-group.my-network.my-deployment.bosh."},
						UDP,
						req,
					)

					Expect(responseMsg.Rcode).To(Equal(dns.RcodeSuccess))
					Expect(len(responseMsg.Answer)).To(Equal(6))
					Expect(responseMsg.Truncated).To(Equal(true))
					Expect(responseMsg.Len()).To(BeNumerically("<", 512))
				})
			})

			Context("when the request is tcp", func() {
				It("does not truncate", func() {
					responseMsg := localDomain.ResolveAnswer(
						"my-instance.my-group.my-network.my-deployment.bosh.",
						[]string{"my-instance.my-group.my-network.my-deployment.bosh."},
						TCP,
						req,
					)

					Expect(responseMsg.Rcode).To(Equal(dns.RcodeSuccess))
					Expect(responseMsg.Truncated).To(Equal(false))
					Expect(len(responseMsg.Answer)).To(Equal(7))
					Expect(responseMsg.Len()).To(BeNumerically(">", 512))
				})
			})
		})

		It("returns A records based off of the records data", func() {
			recordSet := records.RecordSet{
				Records: []records.Record{
					{
						Id:         "instance-id",
						Group:      "group-1",
						Network:    "network-name",
						Deployment: "deployment-name",
						Ip:         "123.123.123.123",
					},
					{
						Id:         "instance-id",
						Group:      "group-1",
						Network:    "network-name",
						Deployment: "deployment-name",
						Ip:         "123.123.123.246",
					},
				},
			}
			fakeRecordSetRepo.GetReturns(recordSet, nil)

			req := &dns.Msg{}
			req.SetQuestion("instance-id-answer.group-1.network-name.deployment-name.bosh.", dns.TypeA)
			responseMsg := localDomain.ResolveAnswer(
				"instance-id-answer.group-1.network-name.deployment-name.bosh.",
				[]string{"instance-id.group-1.network-name.deployment-name.bosh."},
				UDP,
				req,
			)

			answers := responseMsg.Answer
			Expect(answers).To(HaveLen(2))

			answer := answers[0]
			header := answer.Header()
			Expect(header.Name).To(Equal("instance-id-answer.group-1.network-name.deployment-name.bosh."))
			Expect(header.Rrtype).To(Equal(dns.TypeA))
			Expect(header.Class).To(Equal(uint16(dns.ClassINET)))
			Expect(header.Ttl).To(Equal(uint32(0)))
			Expect(answer).To(BeAssignableToTypeOf(&dns.A{}))
			Expect(answer.(*dns.A).A.String()).To(Equal("123.123.123.123"))

			answer = answers[1]
			header = answer.Header()
			Expect(header.Name).To(Equal("instance-id-answer.group-1.network-name.deployment-name.bosh."))
			Expect(header.Rrtype).To(Equal(dns.TypeA))
			Expect(header.Class).To(Equal(uint16(dns.ClassINET)))
			Expect(header.Ttl).To(Equal(uint32(0)))
			Expect(answer).To(BeAssignableToTypeOf(&dns.A{}))
			Expect(answer.(*dns.A).A.String()).To(Equal("123.123.123.246"))

			Expect(responseMsg.Rcode).To(Equal(dns.RcodeSuccess))
		})

		Context("when loading the records returns an error", func() {
			var dnsReturnCode int

			BeforeEach(func() {
				recordSet := records.RecordSet{}
				fakeRecordSetRepo.GetReturns(recordSet, errors.New("i screwed up"))

				req := &dns.Msg{}
				req.SetQuestion("ignored", dns.TypeA)
				responseMsg := localDomain.ResolveAnswer(
					"instance-id-answer.group-1.network-name.deployment-name.bosh.",
					[]string{"instance-id.group-1.network-name.deployment-name.bosh."},
					UDP,
					req,
				)
				dnsReturnCode = responseMsg.Rcode
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

				req := &dns.Msg{}
				req.SetQuestion("q-&^$*^*#^.group-1.network-name.deployment-name.bosh.", dns.TypeA)
				responseMsg := localDomain.ResolveAnswer(
					"q-&^$*^*#^.group-1.network-name.deployment-name.bosh.",
					[]string{"q-&^$*^*#^.group-1.network-name.deployment-name.bosh."},
					UDP,
					req,
				)
				dnsReturnCode = responseMsg.Rcode
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
