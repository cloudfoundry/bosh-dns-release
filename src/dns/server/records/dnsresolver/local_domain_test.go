package dnsresolver_test

import (
	. "dns/server/records/dnsresolver"

	"github.com/cloudfoundry/bosh-utils/logger/loggerfakes"
	"dns/server/records"
	"github.com/miekg/dns"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	"errors"
	"net"

	"dns/server/internal/internalfakes"
	"dns/server/records/dnsresolver/dnsresolverfakes"
)

var _ = Describe("LocalDomain", func() {
	Describe("Resolve", func() {
		var (
			fakeLogger        *loggerfakes.FakeLogger
			fakeWriter        *internalfakes.FakeResponseWriter
			fakeRecordSetRepo *dnsresolverfakes.FakeRecordSetRepo
			localDomain       LocalDomain
			fakeShuffler      *dnsresolverfakes.FakeAnswerShuffler
			fakeHealthLookup  *dnsresolverfakes.FakeHealthLookup
		)

		BeforeEach(func() {
			fakeLogger = &loggerfakes.FakeLogger{}
			fakeWriter = &internalfakes.FakeResponseWriter{}
			fakeRecordSetRepo = &dnsresolverfakes.FakeRecordSetRepo{}
			fakeShuffler = &dnsresolverfakes.FakeAnswerShuffler{}
			fakeShuffler.ShuffleStub = func(input []dns.RR) []dns.RR {
				return input
			}
			fakeHealthLookup = &dnsresolverfakes.FakeHealthLookup{}
			fakeHealthLookup.IsHealthyReturns(true)

			fakeWriter.RemoteAddrReturns(&net.UDPAddr{})
			localDomain = NewLocalDomain(fakeLogger, fakeRecordSetRepo, fakeShuffler, fakeHealthLookup)
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
						Domain:     "bosh.",
					},
					{
						Id:         "instance-2",
						Group:      "group-2",
						Network:    "network-name",
						Deployment: "deployment-name",
						Ip:         "123.123.123.246",
						Domain:     "bosh.",
					},
				},
			}
			fakeRecordSetRepo.GetReturns(recordSet, nil)

			req := &dns.Msg{}
			req.SetQuestion("answer.bosh.", dns.TypeA)
			responseMsg := localDomain.Resolve(
				[]string{
					"instance-1.group-1.network-name.deployment-name.bosh.",
					"instance-2.group-2.network-name.deployment-name.bosh.",
				},
				fakeWriter,
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
						Domain:     "bosh.",
					},
					{
						Id:         "instance-2",
						Group:      "group-1",
						Network:    "network-name",
						Deployment: "deployment-name",
						Ip:         "123.123.123.124",
						Domain:     "bosh.",
					},
				},
			}
			fakeRecordSetRepo.GetReturns(recordSet, nil)
			fakeShuffler.ShuffleStub = func(input []dns.RR) []dns.RR {
				return []dns.RR{input[1], input[0]}
			}
			localDomain = NewLocalDomain(fakeLogger, fakeRecordSetRepo, fakeShuffler, fakeHealthLookup)

			req := &dns.Msg{}
			req.SetQuestion("ignored", dns.TypeA)
			responseMsg := localDomain.Resolve(
				[]string{
					"instance-1.group-1.network-name.deployment-name.bosh.",
					"instance-2.group-1.network-name.deployment-name.bosh.",
				},
				fakeWriter,
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
							Domain:     "bosh.",
						},
						{
							Id:         "my-instance",
							Group:      "my-group",
							Network:    "my-network",
							Deployment: "my-deployment",
							Ip:         "127.0.0.1",
							Domain:     "bosh.",
						},
						{
							Id:         "my-instance",
							Group:      "my-group",
							Network:    "my-network",
							Deployment: "my-deployment",
							Ip:         "127.0.0.2",
							Domain:     "bosh.",
						},
						{
							Id:         "my-instance",
							Group:      "my-group",
							Network:    "my-network",
							Deployment: "my-deployment",
							Ip:         "127.0.0.3",
							Domain:     "bosh.",
						},
						{
							Id:         "my-instance",
							Group:      "my-group",
							Network:    "my-network",
							Deployment: "my-deployment",
							Ip:         "127.0.0.4",
							Domain:     "bosh.",
						},
						{
							Id:         "my-instance",
							Group:      "my-group",
							Network:    "my-network",
							Deployment: "my-deployment",
							Ip:         "127.0.0.5",
							Domain:     "bosh.",
						},
						{
							Id:         "my-instance",
							Group:      "my-group",
							Network:    "my-network",
							Deployment: "my-deployment",
							Ip:         "127.0.0.6",
							Domain:     "bosh.",
						},
					},
				}

				fakeRecordSetRepo.GetReturns(recordSet, nil)
				req = &dns.Msg{}
				req.SetQuestion("my-instance.my-group.my-network.my-deployment.bosh.", dns.TypeA)
			})

			Context("when the request is udp", func() {
				It("truncates the response", func() {
					responseMsg := localDomain.Resolve(
						[]string{"my-instance.my-group.my-network.my-deployment.bosh."},
						fakeWriter,
						req,
					)

					Expect(responseMsg.Rcode).To(Equal(dns.RcodeSuccess))
					Expect(len(responseMsg.Answer)).To(Equal(6))
					Expect(responseMsg.Truncated).To(Equal(true))
					Expect(responseMsg.Len()).To(BeNumerically("<", 512))
				})
			})

			Context("when the request is tcp", func() {
				Context("and the message is longer than MaxMsgSize", func() {
					BeforeEach(func() {
						recordSet = records.RecordSet{
							Records: []records.Record{},
						}

						for i := 0; i < 1000; i += 1 {
							recordSet.Records = append(recordSet.Records, records.Record{
								Id:         "my-instance",
								Group:      "my-group",
								Network:    "my-network",
								Deployment: "my-deployment",
								Ip:         "123.123.123.123",
								Domain:     "bosh.",
							})
						}

						fakeRecordSetRepo.GetReturns(recordSet, nil)
					})

					It("truncates the answers", func() {
						fakeWriter.RemoteAddrReturns(&net.TCPAddr{})

						responseMsg := localDomain.Resolve(
							[]string{"my-instance.my-group.my-network.my-deployment.bosh."},
							fakeWriter,
							req,
						)

						Expect(responseMsg.Rcode).To(Equal(dns.RcodeSuccess))
						// https://tools.ietf.org/html/rfc2181#page-11
						// should not be marked as truncated because we don't want clients to ignore this response
						Expect(responseMsg.Truncated).To(Equal(false))
						Expect(responseMsg.Len()).To(BeNumerically("<", dns.MaxMsgSize))
					})
				})

				It("does not truncate", func() {
					fakeWriter.RemoteAddrReturns(&net.TCPAddr{})

					responseMsg := localDomain.Resolve(
						[]string{"my-instance.my-group.my-network.my-deployment.bosh."},
						fakeWriter,
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
						Domain:     "bosh.",
					},
					{
						Id:         "instance-id",
						Group:      "group-1",
						Network:    "network-name",
						Deployment: "deployment-name",
						Ip:         "123.123.123.246",
						Domain:     "bosh.",
					},
				},
			}
			fakeRecordSetRepo.GetReturns(recordSet, nil)

			req := &dns.Msg{}
			req.SetQuestion("instance-id-answer.group-1.network-name.deployment-name.bosh.", dns.TypeA)
			responseMsg := localDomain.Resolve(
				[]string{"instance-id.group-1.network-name.deployment-name.bosh."},
				fakeWriter,
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

		DescribeTable("health checking",
			func(firstHealthiness, secondHealthiness bool, expectedIPs []string) {
				recordSet := records.RecordSet{
					Records: []records.Record{
						{
							Id:         "instance-id",
							Group:      "group-1",
							Network:    "network-name",
							Deployment: "deployment-name",
							Ip:         "127.0.0.1",
							Domain:     "bosh.",
						},
						{
							Id:         "instance-id",
							Group:      "group-1",
							Network:    "network-name",
							Deployment: "deployment-name",
							Ip:         "127.0.0.2",
							Domain:     "bosh.",
						},
					},
				}
				fakeRecordSetRepo.GetReturns(recordSet, nil)
				fakeHealthLookup.IsHealthyStub = func(ip string) bool {
					switch ip {
					case "127.0.0.1":
						return firstHealthiness
					case "127.0.0.2":
						return secondHealthiness
					}
					return true
				}

				req := &dns.Msg{}
				req.SetQuestion("instance-id-answer.group-1.network-name.deployment-name.bosh.", dns.TypeA)

				response := localDomain.Resolve(
					[]string{"instance-id.group-1.network-name.deployment-name.bosh."},
					fakeWriter,
					req,
				)

				actualIPs := []string{}
				for _, answer := range response.Answer {
					Expect(answer).To(BeAssignableToTypeOf(&dns.A{}))
					actualIPs = append(actualIPs, answer.(*dns.A).A.String())
				}

				Expect(actualIPs).To(ConsistOf(expectedIPs))
			},
			Entry("all healthy IPs, returns all IPs", true, true, []string{"127.0.0.1", "127.0.0.2"}),
			Entry("some unhealthy IPs, returns only healthy", true, false, []string{"127.0.0.1"}),
			Entry("all unhealthy IPs, returns all IPs", false, false, []string{"127.0.0.1", "127.0.0.2"}),
		)

		Context("when loading the records returns an error", func() {
			var dnsReturnCode int

			BeforeEach(func() {
				recordSet := records.RecordSet{}
				fakeRecordSetRepo.GetReturns(recordSet, errors.New("i screwed up"))

				req := &dns.Msg{}
				req.SetQuestion("instance-id-answer.group-1.network-name.deployment-name.bosh.", dns.TypeA)
				responseMsg := localDomain.Resolve(
					[]string{"instance-id.group-1.network-name.deployment-name.bosh."},
					fakeWriter,
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
				responseMsg := localDomain.Resolve(
					[]string{"q-&^$*^*#^.group-1.network-name.deployment-name.bosh."},
					fakeWriter,
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
