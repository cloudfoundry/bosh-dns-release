package handlers_test

import (
	"errors"
	"net"

	"github.com/cloudfoundry/bosh-utils/logger/loggerfakes"
	"github.com/cloudfoundry/dns-release/src/dns/server/handlers"
	"github.com/cloudfoundry/dns-release/src/dns/server/handlers/internal/internalfakes"
	"github.com/cloudfoundry/dns-release/src/dns/server/records"
	"github.com/miekg/dns"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/cloudfoundry/dns-release/src/dns/server/records/dnsresolver/dnsresolverfakes"
	"github.com/cloudfoundry/dns-release/src/dns/server/records/dnsresolver"
)

var _ = Describe("DiscoveryHandler", func() {
	Context("ServeDNS", func() {
		var (
			discoveryHandler  handlers.DiscoveryHandler
			fakeWriter        *internalfakes.FakeResponseWriter
			fakeLogger        *loggerfakes.FakeLogger
			fakeRecordSetRepo *dnsresolverfakes.FakeRecordSetRepo
			fakeShuffler      *dnsresolverfakes.FakeAnswerShuffler
		)

		BeforeEach(func() {
			fakeWriter = &internalfakes.FakeResponseWriter{}
			fakeLogger = &loggerfakes.FakeLogger{}
			fakeRecordSetRepo = &dnsresolverfakes.FakeRecordSetRepo{}
			fakeShuffler = &dnsresolverfakes.FakeAnswerShuffler{}
			fakeShuffler.ShuffleStub = func(input []dns.RR) []dns.RR {
				return input
			}

			discoveryHandler = handlers.NewDiscoveryHandler(fakeLogger, dnsresolver.NewLocalDomain(fakeLogger, fakeRecordSetRepo, fakeShuffler))
		})

		Context("when there are no questions", func() {
			It("returns rcode success", func() {
				discoveryHandler.ServeDNS(fakeWriter, &dns.Msg{})
				message := fakeWriter.WriteMsgArgsForCall(0)
				Expect(message.Rcode).To(Equal(dns.RcodeSuccess))
				Expect(message.Authoritative).To(Equal(true))
				Expect(message.RecursionAvailable).To(Equal(false))
			})
		})

		Context("when there are questions", func() {
			It("returns rcode success for mx questions", func() {
				m := &dns.Msg{}
				m.SetQuestion("my-instance.my-network.my-deployment.bosh.", dns.TypeMX)

				discoveryHandler.ServeDNS(fakeWriter, m)
				message := fakeWriter.WriteMsgArgsForCall(0)
				Expect(message.Rcode).To(Equal(dns.RcodeSuccess))
				Expect(message.Authoritative).To(Equal(true))
				Expect(message.RecursionAvailable).To(Equal(false))
			})

			It("returns rcode success for aaaa questions", func() {
				m := &dns.Msg{}
				m.SetQuestion("my-instance.my-network.my-deployment.bosh.", dns.TypeAAAA)

				discoveryHandler.ServeDNS(fakeWriter, m)
				message := fakeWriter.WriteMsgArgsForCall(0)
				Expect(message.Rcode).To(Equal(dns.RcodeSuccess))
				Expect(message.Authoritative).To(Equal(true))
				Expect(message.RecursionAvailable).To(Equal(false))
			})

			It("returns rcode server failure for all other questions", func() {
				m := &dns.Msg{}
				m.SetQuestion("my-instance.my-network.my-deployment.bosh.", dns.TypePTR)

				discoveryHandler.ServeDNS(fakeWriter, m)
				message := fakeWriter.WriteMsgArgsForCall(0)
				Expect(message.Rcode).To(Equal(dns.RcodeServerFailure))
				Expect(message.Authoritative).To(Equal(true))
				Expect(message.RecursionAvailable).To(Equal(false))
			})

			Context("when the question is an A or ANY record", func() {
				DescribeTable("returns an A record based off of the records data",
					func(queryType uint16) {
						recordSet := records.RecordSet{
							Records: []records.Record{
								{
									Id:         "my-instance",
									Group:      "my-group",
									Network:    "my-network",
									Deployment: "my-deployment",
									Ip:         "123.123.123.123",
								},
							},
						}
						fakeRecordSetRepo.GetReturns(recordSet, nil)

						m := &dns.Msg{}
						m.SetQuestion("my-instance.my-group.my-network.my-deployment.bosh.", queryType)

						discoveryHandler.ServeDNS(fakeWriter, m)
						responseMsg := fakeWriter.WriteMsgArgsForCall(0)

						Expect(responseMsg.Rcode).To(Equal(dns.RcodeSuccess))
						Expect(responseMsg.Authoritative).To(Equal(true))
						Expect(responseMsg.RecursionAvailable).To(Equal(false))
						Expect(responseMsg.Truncated).To(Equal(false))

						Expect(responseMsg.Answer).To(HaveLen(1))

						answer := responseMsg.Answer[0]
						header := answer.Header()

						Expect(header.Rrtype).To(Equal(dns.TypeA))
						Expect(header.Class).To(Equal(uint16(dns.ClassINET)))
						Expect(header.Ttl).To(Equal(uint32(0)))

						Expect(answer).To(BeAssignableToTypeOf(&dns.A{}))
						Expect(answer.(*dns.A).A.String()).To(Equal("123.123.123.123"))

						Expect(fakeLogger.InfoCallCount()).To(Equal(0))
					},
					Entry("when the question is an A query", dns.TypeA),
					Entry("when the question is an ANY query", dns.TypeANY),
				)

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
							discoveryHandler.ServeDNS(fakeWriter, req)

							responseMsg := fakeWriter.WriteMsgArgsForCall(0)
							Expect(responseMsg.Rcode).To(Equal(dns.RcodeSuccess))
							Expect(len(responseMsg.Answer)).To(Equal(6))
							Expect(responseMsg.Truncated).To(Equal(true))
							Expect(responseMsg.Len()).To(BeNumerically("<", 512))
						})
					})

					Context("when the request is tcp", func() {
						It("does not truncate", func() {
							fakeWriter.RemoteAddrReturns(&net.TCPAddr{})
							discoveryHandler.ServeDNS(fakeWriter, req)

							responseMsg := fakeWriter.WriteMsgArgsForCall(0)
							Expect(responseMsg.Rcode).To(Equal(dns.RcodeSuccess))
							Expect(responseMsg.Truncated).To(Equal(false))
							Expect(len(responseMsg.Answer)).To(Equal(7))
							Expect(responseMsg.Len()).To(BeNumerically(">", 512))
						})
					})
				})
			})

			Context("when loading the records returns an error", func() {
				BeforeEach(func() {
					recordSet := records.RecordSet{}
					fakeRecordSetRepo.GetReturns(recordSet, errors.New("i screwed up"))
				})

				It("returns rcode server failure", func() {
					m := &dns.Msg{}
					m.SetQuestion("my-instance.my-network.my-deployment.bosh.", dns.TypeA)

					discoveryHandler.ServeDNS(fakeWriter, m)
					message := fakeWriter.WriteMsgArgsForCall(0)
					Expect(message.Rcode).To(Equal(dns.RcodeServerFailure))
					Expect(message.Authoritative).To(Equal(true))
					Expect(message.RecursionAvailable).To(Equal(false))
				})

				It("logs the error", func() {
					m := &dns.Msg{}
					m.SetQuestion("my-instance.my-network.my-deployment.bosh.", dns.TypeA)
					discoveryHandler.ServeDNS(fakeWriter, m)

					Expect(fakeLogger.ErrorCallCount()).To(Equal(1))
					tag, msg, args := fakeLogger.ErrorArgsForCall(0)
					Expect(tag).To(Equal("LocalDomain"))
					Expect(msg).To(Equal("failed to get ip addresses: %v"))
					Expect(args[0]).To(MatchError("i screwed up"))
				})
			})

			Context("when parsing the query returns an error", func() {
				var question string
				BeforeEach(func() {
					recordSet := records.RecordSet{}
					fakeRecordSetRepo.GetReturns(recordSet, nil)
					question = "q-&^$*^*#^.my-group.my-network.my-deployment.bosh."
				})

				It("returns rcode format error", func() {
					m := &dns.Msg{}
					m.SetQuestion(question, dns.TypeA)

					discoveryHandler.ServeDNS(fakeWriter, m)
					message := fakeWriter.WriteMsgArgsForCall(0)
					Expect(message.Rcode).To(Equal(dns.RcodeFormatError))
					Expect(message.Authoritative).To(Equal(true))
					Expect(message.RecursionAvailable).To(Equal(false))
				})

				It("logs the error", func() {
					m := &dns.Msg{}
					m.SetQuestion(question, dns.TypeA)
					discoveryHandler.ServeDNS(fakeWriter, m)

					Expect(fakeLogger.ErrorCallCount()).To(Equal(1))
					tag, msg, _ := fakeLogger.ErrorArgsForCall(0)
					Expect(tag).To(Equal("LocalDomain"))
					Expect(msg).To(Equal("failed to decode query: %v"))
				})
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
