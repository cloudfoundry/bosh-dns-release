package handlers_test

import (
	"errors"
	"net"
	"time"

	"github.com/cloudfoundry/bosh-utils/logger/loggerfakes"
	"github.com/cloudfoundry/dns-release/src/dns/server/handlers"
	"github.com/cloudfoundry/dns-release/src/dns/server/handlers/handlersfakes"
	"github.com/cloudfoundry/dns-release/src/dns/server/handlers/internal/internalfakes"
	"github.com/miekg/dns"

	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("ForwardHandler", func() {
	Describe("ServeDNS", func() {
		var (
			fakeWriter           *internalfakes.FakeResponseWriter
			recursionHandler     handlers.ForwardHandler
			fakeExchangerFactory handlers.ExchangerFactory
			fakeExchanger        *handlersfakes.FakeExchanger
			fakeLogger           *loggerfakes.FakeLogger
		)

		BeforeEach(func() {
			fakeWriter = &internalfakes.FakeResponseWriter{}
			fakeExchanger = &handlersfakes.FakeExchanger{}
			fakeExchangerFactory = func(net string) handlers.Exchanger {
				return fakeExchanger
			}
			fakeLogger = &loggerfakes.FakeLogger{}

			recursionHandler = handlers.NewForwardHandler([]string{"127.0.0.1", "10.244.5.4"}, fakeExchangerFactory, fakeLogger)
		})

		Context("when no recursors are configured", func() {
			var recursionHandler handlers.ForwardHandler
			var msg *dns.Msg
			BeforeEach(func() {
				recursionHandler = handlers.NewForwardHandler([]string{}, fakeExchangerFactory, fakeLogger)
				msg = &dns.Msg{}
				msg.SetQuestion("example.com.", dns.TypeANY)
			})

			It("sets a failure rcode", func() {
				recursionHandler.ServeDNS(fakeWriter, msg)
				Expect(fakeWriter.WriteMsgCallCount()).To(Equal(1))

				Expect(fakeLogger.InfoCallCount()).To(Equal(2))
				tag, logMsg, _ := fakeLogger.InfoArgsForCall(1)
				Expect(tag).To(Equal("ForwardHandler"))
				Expect(logMsg).To(Equal("no response from recursors"))

				message := fakeWriter.WriteMsgArgsForCall(0)
				Expect(message.Question).To(Equal(msg.Question))
				Expect(message.Rcode).To(Equal(dns.RcodeServerFailure))
				Expect(message.Authoritative).To(Equal(false))
				Expect(message.RecursionAvailable).To(Equal(true))
			})

			Context("when the message fails to write", func() {
				It("logs the error", func() {
					fakeWriter.WriteMsgReturns(errors.New("failed to write message"))

					recursionHandler.ServeDNS(fakeWriter, msg)

					Expect(fakeLogger.ErrorCallCount()).To(Equal(1))
					tag, msg, args := fakeLogger.ErrorArgsForCall(0)
					Expect(tag).To(Equal("ForwardHandler"))
					Expect(msg).To(Equal("error writing response %s"))
					Expect(args).To(ContainElement("failed to write message"))
				})
			})
		})

		Context("when request contains no questions", func() {
			It("set a success rcode and authorative", func() {
				recursionHandler.ServeDNS(fakeWriter, &dns.Msg{})
				message := fakeWriter.WriteMsgArgsForCall(0)
				Expect(message.Rcode).To(Equal(dns.RcodeSuccess))
				Expect(message.Authoritative).To(Equal(true))
				Expect(message.RecursionAvailable).To(Equal(false))

				Expect(fakeLogger.InfoCallCount()).To(Equal(1))
				tag, msg, _ := fakeLogger.InfoArgsForCall(0)
				Expect(tag).To(Equal("ForwardHandler"))
				Expect(msg).To(Equal("received a request with no questions"))
			})

			Context("when the message fails to write", func() {
				It("logs an error", func() {
					fakeWriter.WriteMsgReturns(errors.New("failed to write message"))

					recursionHandler.ServeDNS(fakeWriter, &dns.Msg{})

					Expect(fakeLogger.ErrorCallCount()).To(Equal(1))
					tag, msg, args := fakeLogger.ErrorArgsForCall(0)
					Expect(tag).To(Equal("ForwardHandler"))
					Expect(msg).To(Equal("error writing response %s"))
					Expect(args).To(ContainElement("failed to write message"))
				})
			})
		})

		Context("when request contains questions", func() {
			DescribeTable("it responds to DNS requests",
				func(protocol string, remoteAddrReturns net.Addr, truncatedResponse bool) {
					recursorAnswer := &dns.Msg{
						Answer: []dns.RR{&dns.A{A: net.ParseIP("99.99.99.99")}},
					}
					fakeExchanger := &handlersfakes.FakeExchanger{}

					var err error
					if truncatedResponse {
						err = dns.ErrTruncated
					}
					fakeExchanger.ExchangeReturns(recursorAnswer, 0, err)

					fakeExchangerFactory := func(net string) handlers.Exchanger {
						if net == protocol {
							return fakeExchanger
						}

						return &handlersfakes.FakeExchanger{}
					}

					fakeWriter.RemoteAddrReturns(remoteAddrReturns)
					recursionHandler := handlers.NewForwardHandler([]string{"127.0.0.1"}, fakeExchangerFactory, fakeLogger)

					m := &dns.Msg{}
					m.SetQuestion("example.com.", dns.TypeANY)

					recursionHandler.ServeDNS(fakeWriter, m)
					message := fakeWriter.WriteMsgArgsForCall(0)
					Expect(message.Rcode).To(Equal(dns.RcodeSuccess))
					Expect(message.Answer).To(Equal(recursorAnswer.Answer))

					Expect(fakeExchanger.ExchangeCallCount()).To(Equal(1))
					msg, recursor := fakeExchanger.ExchangeArgsForCall(0)
					Expect(recursor).To(Equal("127.0.0.1"))
					Expect(msg).To(Equal(m))

					Expect(fakeLogger.InfoCallCount()).To(Equal(1))
					tag, logMsg, _ := fakeLogger.InfoArgsForCall(0)
					Expect(tag).To(Equal("ForwardHandler"))
					Expect(logMsg).To(Equal("attempting recursors"))
				},
				Entry("forwards query to recursor via udp for udp clients", "udp", nil, false),
				Entry("forwards query to recursor via udp for udp clients when the response is truncated", "udp", nil, true),
				Entry("forwards query to recursor via tcp for tcp clients", "tcp", &net.TCPAddr{}, false),
			)

			Context("compression", func() {
				var (
					requestMessage   *dns.Msg
					recursorAnswer   *dns.Msg
					recursionHandler dns.Handler
				)

				appendAnswersUntilSize := func(requestedResponseLen int) {
					for i := 0; recursorAnswer.Len() < requestedResponseLen; i++ {
						aRec := &dns.A{
							Hdr: dns.RR_Header{
								Name:   "foo.bar.",
								Rrtype: dns.TypeA,
								Class:  dns.ClassINET,
								Ttl:    0,
							},
							A: net.ParseIP(fmt.Sprintf("127.0.0.%d", i+1)).To4(),
						}
						recursorAnswer.Answer = append(recursorAnswer.Answer, aRec)
					}
				}

				BeforeEach(func() {
					recursorAnswer = &dns.Msg{
						Answer: []dns.RR{&dns.A{A: net.ParseIP("99.99.99.99")}},
					}
					fakeExchanger := &handlersfakes.FakeExchanger{}
					fakeExchangerFactory := func(net string) handlers.Exchanger { return fakeExchanger }
					recursionHandler = handlers.NewForwardHandler([]string{"127.0.0.1"}, fakeExchangerFactory, fakeLogger)
					requestMessage = &dns.Msg{}
					requestMessage.SetQuestion("example.com.", dns.TypeANY)
					fakeExchanger.ExchangeReturns(recursorAnswer, 0, nil)
				})

				Context("when the request is tcp and is large", func() {
					It("does not compress the response", func() {
						appendAnswersUntilSize(1024)
						fakeWriter.RemoteAddrReturns(&net.TCPAddr{})
						Expect(recursorAnswer.Len()).To(BeNumerically(">", 1024))

						recursionHandler.ServeDNS(fakeWriter, requestMessage)
						responseMessage := fakeWriter.WriteMsgArgsForCall(0)
						Expect(responseMessage.Answer).To(HaveLen(len(recursorAnswer.Answer)))
						Expect(responseMessage.Compress).To(BeFalse())
						Expect(responseMessage.Len()).To(Equal(recursorAnswer.Len()))
					})
				})

				Context("when the request is udp and the response is less than 512", func() {
					It("does not compress the response", func() {
						appendAnswersUntilSize(400)
						fakeWriter.RemoteAddrReturns(&net.UDPAddr{})
						Expect(recursorAnswer.Len()).To(BeNumerically("<", 512))

						recursionHandler.ServeDNS(fakeWriter, requestMessage)
						responseMessage := fakeWriter.WriteMsgArgsForCall(0)
						Expect(responseMessage.Compress).To(BeFalse())
						Expect(responseMessage.Answer).To(HaveLen(len(recursorAnswer.Answer)))
						Expect(responseMessage.Len()).To(Equal(recursorAnswer.Len()))
					})
				})

				Context("when the request is udp and the response is greater than 512", func() {
					It("compresses the response", func() {
						appendAnswersUntilSize(512)
						fakeWriter.RemoteAddrReturns(&net.UDPAddr{})
						Expect(recursorAnswer.Len()).To(BeNumerically(">", 512))

						recursionHandler.ServeDNS(fakeWriter, requestMessage)
						responseMessage := fakeWriter.WriteMsgArgsForCall(0)
						Expect(responseMessage.Compress).To(BeTrue())
						Expect(responseMessage.Answer).To(HaveLen(len(recursorAnswer.Answer)))
						Expect(responseMessage.Len()).To(BeNumerically("<", recursorAnswer.Len()))
					})

					Context("when the request contains an Edns0 UDPSize that is greater than the response size", func() {
						It("does not compress the response", func() {
							appendAnswersUntilSize(512)
							requestMessage.SetEdns0(1024, true)
							fakeWriter.RemoteAddrReturns(&net.UDPAddr{})

							Expect(recursorAnswer.Len()).To(BeNumerically(">", 512))
							Expect(recursorAnswer.Len()).To(BeNumerically("<", 1024))

							recursionHandler.ServeDNS(fakeWriter, requestMessage)
							responseMessage := fakeWriter.WriteMsgArgsForCall(0)
							Expect(responseMessage.Compress).To(BeFalse())
							Expect(responseMessage.Answer).To(HaveLen(len(recursorAnswer.Answer)))
							Expect(responseMessage.Len()).To(Equal(recursorAnswer.Len()))
						})
					})

					Context("when the request contains an Edns0 UDPSize that is smaller than the response size", func() {
						It("compresses the response", func() {
							appendAnswersUntilSize(1024)
							fakeWriter.RemoteAddrReturns(&net.UDPAddr{})
							requestMessage.SetEdns0(1024, true)

							Expect(recursorAnswer.Len()).To(BeNumerically(">", 1024))

							recursionHandler.ServeDNS(fakeWriter, requestMessage)
							responseMessage := fakeWriter.WriteMsgArgsForCall(0)
							Expect(responseMessage.Compress).To(BeTrue())
							Expect(responseMessage.Answer).To(HaveLen(len(recursorAnswer.Answer)))
							Expect(responseMessage.Len()).To(BeNumerically("<", recursorAnswer.Len()))
						})
					})
				})
			})

			Context("when a recursor fails", func() {
				var (
					msg *dns.Msg
				)

				BeforeEach(func() {
					fakeExchanger := &handlersfakes.FakeExchanger{}
					fakeExchanger.ExchangeReturns(&dns.Msg{}, 0, errors.New("failed to exchange"))

					fakeExchangerFactory := func(net string) handlers.Exchanger {
						return fakeExchanger
					}

					recursionHandler := handlers.NewForwardHandler([]string{"127.0.0.1", "127.0.0.2"}, fakeExchangerFactory, fakeLogger)

					msg = &dns.Msg{}
					msg.SetQuestion("example.com.", dns.TypeANY)

					recursionHandler.ServeDNS(fakeWriter, msg)
				})

				It("writes a failure result", func() {
					Expect(fakeLogger.InfoCallCount()).To(Equal(4))
					tag, msg, args := fakeLogger.InfoArgsForCall(1)
					Expect(tag).To(Equal("ForwardHandler"))
					Expect(msg).To(Equal("error recursing to %s %s"))
					Expect(args[0]).To(Equal("127.0.0.1"))
					Expect(args[1]).To(Equal("failed to exchange"))

					tag, msg, args = fakeLogger.InfoArgsForCall(2)
					Expect(tag).To(Equal("ForwardHandler"))
					Expect(msg).To(Equal("error recursing to %s %s"))
					Expect(args[0]).To(Equal("127.0.0.2"))
					Expect(args[1]).To(Equal("failed to exchange"))
				})

				Context("when all recursors fail", func() {
					It("returns a server failure", func() {
						Expect(fakeWriter.WriteMsgCallCount()).To(Equal(1))

						message := fakeWriter.WriteMsgArgsForCall(0)
						Expect(message.Question).To(Equal(msg.Question))
						Expect(message.Rcode).To(Equal(dns.RcodeServerFailure))
						Expect(message.Authoritative).To(Equal(false))
						Expect(message.RecursionAvailable).To(Equal(true))
					})
				})
			})

			It("returns with the first recursor response", func() {
				exchangeMsg := &dns.Msg{
					Answer: []dns.RR{&dns.A{A: net.ParseIP("99.99.99.99")}},
				}

				fakeExchanger.ExchangeStub = func(msg *dns.Msg, address string) (*dns.Msg, time.Duration, error) {
					if address == "10.244.5.4" {
						return exchangeMsg, 0, nil
					}
					return nil, 0, errors.New("recursor failed to reply")
				}

				m := &dns.Msg{}
				m.SetQuestion("example.com.", dns.TypeANY)

				recursionHandler.ServeDNS(fakeWriter, m)
				message := fakeWriter.WriteMsgArgsForCall(0)
				Expect(message.Rcode).To(Equal(dns.RcodeSuccess))
				Expect(message.Answer).To(Equal(exchangeMsg.Answer))

				Expect(fakeExchanger.ExchangeCallCount()).To(Equal(2))
				msg, recursor := fakeExchanger.ExchangeArgsForCall(1)
				Expect(recursor).To(Equal("10.244.5.4"))
				Expect(msg).To(Equal(m))
			})

			Context("when the message fails to write", func() {
				It("logs the error", func() {
					fakeWriter.WriteMsgReturns(errors.New("failed to write message"))

					m := &dns.Msg{}
					m.SetQuestion("example.com.", dns.TypeANY)

					recursionHandler.ServeDNS(fakeWriter, m)

					Expect(fakeLogger.ErrorCallCount()).To(Equal(1))
					tag, msg, args := fakeLogger.ErrorArgsForCall(0)
					Expect(tag).To(Equal("ForwardHandler"))
					Expect(msg).To(Equal("error writing response %s"))
					Expect(args).To(ContainElement("failed to write message"))
				})
			})
		})
	})
})
