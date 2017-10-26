package dnsresolver_test

import (
	. "bosh-dns/dns/server/records/dnsresolver"

	"github.com/cloudfoundry/bosh-utils/logger/loggerfakes"
	"github.com/miekg/dns"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"errors"
	"net"

	"bosh-dns/dns/server/internal/internalfakes"
	"bosh-dns/dns/server/records/dnsresolver/dnsresolverfakes"
)

var _ = Describe("LocalDomain", func() {
	Describe("Resolve", func() {
		var (
			fakeLogger    *loggerfakes.FakeLogger
			fakeWriter    *internalfakes.FakeResponseWriter
			fakeRecordSet *dnsresolverfakes.FakeRecordSet
			localDomain   LocalDomain
			fakeShuffler  *dnsresolverfakes.FakeAnswerShuffler
		)

		BeforeEach(func() {
			fakeLogger = &loggerfakes.FakeLogger{}
			fakeWriter = &internalfakes.FakeResponseWriter{}
			fakeRecordSet = &dnsresolverfakes.FakeRecordSet{}
			fakeShuffler = &dnsresolverfakes.FakeAnswerShuffler{}
			fakeShuffler.ShuffleStub = func(input []dns.RR) []dns.RR {
				return input
			}

			fakeWriter.RemoteAddrReturns(&net.UDPAddr{})
			localDomain = NewLocalDomain(fakeLogger, fakeRecordSet, fakeShuffler)
		})

		It("returns responses from all the question domains", func() {
			fakeRecordSet.ResolveStub = func(domain string) ([]string, bool, error) {
				switch domain {
				case "instance-1.group-1.network-name.deployment-name.bosh.":
					return []string{"123.123.123.123"}, true, nil
				case "instance-2.group-2.network-name.deployment-name.bosh.":
					return []string{"123.123.123.246"}, true, nil
				}

				return nil, false, errors.New("nope")
			}

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

			Expect(responseMsg.RecursionAvailable).To(BeTrue())
			Expect(responseMsg.Authoritative).To(BeTrue())
			Expect(responseMsg.Rcode).To(Equal(dns.RcodeSuccess))
		})

		It("shuffles the answers", func() {
			fakeRecordSet.ResolveStub = func(domain string) ([]string, bool, error) {
				switch domain {
				case "instance-1.group-1.network-name.deployment-name.bosh.":
					return []string{"123.123.123.123"}, true, nil
				case "instance-2.group-1.network-name.deployment-name.bosh.":
					return []string{"123.123.123.124"}, true, nil
				}

				return nil, false, errors.New("nope")
			}

			fakeShuffler.ShuffleStub = func(input []dns.RR) []dns.RR {
				return []dns.RR{input[1], input[0]}
			}
			localDomain = NewLocalDomain(fakeLogger, fakeRecordSet, fakeShuffler)

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

		Context("when one domain is healthy and the other is unhealthy", func() {
			It("only returns responses from the healthy domain", func() {
				fakeRecordSet.ResolveStub = func(domain string) ([]string, bool, error) {
					switch domain {
					case "instance-1.group-1.network-name.deployment-name.bosh.":
						return []string{"123.123.123.123"}, true, nil
					case "instance-2.group-2.network-name.deployment-name.bosh.":
						return []string{"123.123.123.246"}, false, nil
					}

					return nil, false, errors.New("nope")
				}

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
				Expect(answers).To(HaveLen(1))

				answer := answers[0]
				header := answer.Header()
				Expect(header.Name).To(Equal("answer.bosh."))
				Expect(header.Rrtype).To(Equal(dns.TypeA))
				Expect(header.Class).To(Equal(uint16(dns.ClassINET)))
				Expect(header.Ttl).To(Equal(uint32(0)))
				Expect(answer).To(BeAssignableToTypeOf(&dns.A{}))
				Expect(answer.(*dns.A).A.String()).To(Equal("123.123.123.123"))

				Expect(responseMsg.RecursionAvailable).To(BeTrue())
				Expect(responseMsg.Authoritative).To(BeTrue())
				Expect(responseMsg.Rcode).To(Equal(dns.RcodeSuccess))
			})
		})

		Context("when all domains are unhealthy", func() {
			It("returns responses from all domains", func() {
				fakeRecordSet.ResolveStub = func(domain string) ([]string, bool, error) {
					switch domain {
					case "instance-1.group-1.network-name.deployment-name.bosh.":
						return []string{"123.123.123.123"}, false, nil
					case "instance-2.group-2.network-name.deployment-name.bosh.":
						return []string{"123.123.123.246"}, false, nil
					}

					return nil, false, errors.New("nope")
				}

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

				Expect(responseMsg.RecursionAvailable).To(BeTrue())
				Expect(responseMsg.Authoritative).To(BeTrue())
				Expect(responseMsg.Rcode).To(Equal(dns.RcodeSuccess))
			})
		})

		Context("when there are too many records to fit into 512 bytes", func() {
			var (
				req *dns.Msg
			)

			BeforeEach(func() {
				fakeRecordSet.ResolveStub = func(domain string) ([]string, bool, error) {
					Expect(domain).To(Equal("my-instance.my-group.my-network.my-deployment.bosh."))

					return []string{"123.123.123.123", "127.0.0.1", "127.0.0.2", "127.0.0.3", "127.0.0.4", "127.0.0.5", "127.0.0.6"}, true, nil
				}
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
						hugeSlice := make([]string, 1000)
						for i := 0; i < 1000; i += 1 {
							hugeSlice = append(hugeSlice, "123.123.123.123")
						}
						fakeRecordSet.ResolveReturns(hugeSlice, true, nil)
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
			fakeRecordSet.ResolveReturns([]string{"123.123.123.123", "123.123.123.246"}, true, nil)

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

		Context("when loading the records returns an error", func() {
			var dnsReturnCode int

			BeforeEach(func() {
				fakeRecordSet.ResolveReturns(nil, false, errors.New("i screwed up"))

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
				Expect(dnsReturnCode).To(Equal(dns.RcodeFormatError))
			})

			It("logs the error", func() {
				Expect(fakeLogger.ErrorCallCount()).To(Equal(1))
				tag, msg, args := fakeLogger.ErrorArgsForCall(0)
				Expect(tag).To(Equal("LocalDomain"))
				Expect(msg).To(Equal("failed to get ip addresses: %v"))
				Expect(args[0]).To(MatchError("i screwed up"))
			})
		})
	})
})
