package handlers_test

import (
	"errors"
	"fmt"
	"net"
	"sync/atomic"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	"github.com/cloudfoundry/bosh-utils/logger/loggerfakes"
	"github.com/miekg/dns"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"bosh-dns/dns/config"
	. "bosh-dns/dns/internal/testhelpers/question_case_helpers"
	"bosh-dns/dns/server/handlers"
	"bosh-dns/dns/server/handlers/handlersfakes"
	"bosh-dns/dns/server/internal/internalfakes"
	"bosh-dns/dns/server/records/dnsresolver/dnsresolverfakes"
)

var _ = Describe("ForwardHandler", func() {
	Describe("ServeDNS", func() {
		var (
			fakeWriter           *internalfakes.FakeResponseWriter
			recursionHandler     handlers.ForwardHandler
			fakeExchangerFactory handlers.ExchangerFactory
			fakeExchanger        *handlersfakes.FakeExchanger
			fakeClock            *fakeclock.FakeClock
			fakeLogger           *loggerfakes.FakeLogger
			fakeRecursorPool     *handlersfakes.FakeRecursorPool
			fakeTruncater        *dnsresolverfakes.FakeResponseTruncater
		)

		BeforeEach(func() {
			fakeWriter = &internalfakes.FakeResponseWriter{}
			fakeExchanger = &handlersfakes.FakeExchanger{}
			fakeExchangerFactory = func(net string) handlers.Exchanger {
				return fakeExchanger
			}
			fakeLogger = &loggerfakes.FakeLogger{}
			fakeClock = fakeclock.NewFakeClock(time.Now())
			fakeRecursorPool = &handlersfakes.FakeRecursorPool{}
			recursors := []string{"127.0.0.1", "10.244.5.4"}
			fakeRecursorPool.PerformStrategicallyStub = func(f func(string) error) error {
				var err error
				for _, recursor := range recursors {
					err = f(recursor)
					if err == nil {
						return nil
					}
				}
				return err
			}
			fakeTruncater = &dnsresolverfakes.FakeResponseTruncater{}
			recursionHandler = handlers.NewForwardHandler(fakeRecursorPool, fakeExchangerFactory, fakeClock, fakeLogger, fakeTruncater)
		})

		Context("when there are no recursors configured", func() {
			var msg *dns.Msg
			var casedQname string
			BeforeEach(func() {
				fakeRecursorPool.PerformStrategicallyReturns(errors.New("no recursors configured"))
				msg = &dns.Msg{}
				SetQuestion(msg, &casedQname, "example.com.", dns.TypeANY)
			})

			It("indicates that there are no recursers available", func() {
				recursionHandler.ServeDNS(fakeWriter, msg)
				Expect(fakeWriter.WriteMsgCallCount()).To(Equal(1))

				Expect(fakeLogger.DebugCallCount()).To(Equal(2))
				tag, logMsg, _ := fakeLogger.DebugArgsForCall(1)
				Expect(tag).To(Equal("ForwardHandler"))
				Expect(logMsg).To(Equal(fmt.Sprintf("handlers.ForwardHandler Request id=%d qtype=[ANY] qname=["+casedQname+"] rcode=NXDOMAIN ancount=0 error=[no recursors configured] time=0ns", msg.Id)))

				message := fakeWriter.WriteMsgArgsForCall(0)
				Expect(message.Question).To(Equal(msg.Question))
				Expect(message.Rcode).To(Equal(dns.RcodeNameError))
				Expect(message.Authoritative).To(Equal(false))
				Expect(message.RecursionAvailable).To(Equal(false))
			})
		})

		Context("when first recursor returns a message", func() {

			DescribeTable("non-success codes (except SERVFAIL) are treated as errors",
				func(rcode int, expectedErr string) {
					fakeExchanger.ExchangeReturns(&dns.Msg{
						MsgHdr: dns.MsgHdr{
							Rcode: rcode,
						},
					}, 0, nil)

					fakeRecursorPool.PerformStrategicallyStub = func(f func(string) error) error {
						err := f("127.0.0.1")
						Expect(err).To(MatchError(expectedErr))
						return err
					}

					recursionHandler.ServeDNS(fakeWriter, &dns.Msg{
						Question: []dns.Question{
							{
								Name: "my.domain",
							},
						},
					})

					Expect(fakeWriter.WriteMsgCallCount()).To(Equal(1))
					response := fakeWriter.WriteMsgArgsForCall(0)
					Expect(response.Rcode).To(Equal(dns.RcodeNameError))
				},
				Entry("returns NXDOMAIN", dns.RcodeNameError, "received NXDOMAIN for my.domain from upstream (recursor: 127.0.0.1)"),
			)

			DescribeTable("SERVFAIL codes are treated as SERVFAIL",
				func(rcode int, expectedErr string) {
					fakeExchanger.ExchangeReturns(&dns.Msg{
						MsgHdr: dns.MsgHdr{
							Rcode: rcode,
						},
					}, 0, nil)

					fakeRecursorPool.PerformStrategicallyStub = func(f func(string) error) error {
						err := f("127.0.0.1")
						Expect(err).To(MatchError(expectedErr))
						return err
					}

					recursionHandler.ServeDNS(fakeWriter, &dns.Msg{
						Question: []dns.Question{
							{
								Name: "my.domain",
							},
						},
					})

					Expect(fakeWriter.WriteMsgCallCount()).To(Equal(1))
					response := fakeWriter.WriteMsgArgsForCall(0)
					Expect(response.Rcode).To(Equal(dns.RcodeServerFailure))
				},
				Entry("returns SERVFAIL", dns.RcodeServerFailure, "received SERVFAIL for my.domain from upstream (recursor: 127.0.0.1)"),
			)
		})

		Context("when no working recursors are configured", func() {
			var msg *dns.Msg
			var casedQname string

			BeforeEach(func() {
				fakeExchanger.ExchangeReturns(nil, 0, errors.New("first recursor failed to reply"))
				msg = &dns.Msg{}
				SetQuestion(msg, &casedQname, "example.com.", dns.TypeANY)
			})

			It("sets a failure rcode", func() {
				recursionHandler.ServeDNS(fakeWriter, msg)
				Expect(fakeWriter.WriteMsgCallCount()).To(Equal(1))

				Expect(fakeLogger.ErrorCallCount()).To(Equal(2))
				Expect(fakeLogger.DebugCallCount()).To(Equal(2))
				tag, logMsg, args := fakeLogger.ErrorArgsForCall(0)
				Expect(tag).To(Equal("ForwardHandler"))
				Expect(logMsg).To(Equal("error recursing for %s to %q: %s"))
				Expect(args[0]).To(Equal(casedQname))
				Expect(args[1]).To(Equal("127.0.0.1"))
				Expect(args[2]).To(Equal("first recursor failed to reply"))
				tag, logMsg, args = fakeLogger.ErrorArgsForCall(1)
				Expect(tag).To(Equal("ForwardHandler"))
				Expect(logMsg).To(Equal("error recursing for %s to %q: %s"))
				Expect(args[0]).To(Equal(casedQname))
				Expect(args[1]).To(Equal("10.244.5.4"))
				Expect(args[2]).To(Equal("first recursor failed to reply"))
				tag, logMsg, _ = fakeLogger.DebugArgsForCall(1)
				Expect(tag).To(Equal("ForwardHandler"))
				Expect(logMsg).To(Equal(fmt.Sprintf("handlers.ForwardHandler Request id=%d qtype=[ANY] qname=["+casedQname+"] rcode=NXDOMAIN ancount=0 error=[first recursor failed to reply] time=0ns", msg.Id)))

				message := fakeWriter.WriteMsgArgsForCall(0)
				Expect(message.Question).To(Equal(msg.Question))
				Expect(message.Rcode).To(Equal(dns.RcodeNameError))
				Expect(message.Authoritative).To(Equal(false))
				Expect(message.RecursionAvailable).To(Equal(false))
			})

			Context("when the message fails to write", func() {
				It("logs the error", func() {
					fakeWriter.WriteMsgReturns(errors.New("failed to write message"))

					recursionHandler.ServeDNS(fakeWriter, msg)

					Expect(fakeLogger.ErrorCallCount()).To(Equal(3))
					tag, msg, args := fakeLogger.ErrorArgsForCall(2)
					Expect(tag).To(Equal("ForwardHandler"))
					Expect(fmt.Sprintf(msg, args...)).To(Equal("error writing response: failed to write message"))
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

				Expect(fakeLogger.DebugCallCount()).To(Equal(2))
				tag, msg, _ := fakeLogger.DebugArgsForCall(1)
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
					Expect(fmt.Sprintf(msg, args...)).To(Equal("error writing response: failed to write message"))
				})
			})
		})

		Context("when request contains questions", func() {
			DescribeTable("responds to DNS requests",
				func(protocol string, remoteAddrReturns net.Addr) {
					recursorAnswer := &dns.Msg{
						Answer: []dns.RR{&dns.A{A: net.ParseIP("99.99.99.99")}},
					}
					fakeExchanger := &handlersfakes.FakeExchanger{}

					var err error
					fakeExchanger.ExchangeReturns(recursorAnswer, 0, err)

					fakeExchangerFactory := func(net string) handlers.Exchanger {
						if net == protocol {
							return fakeExchanger
						}

						return &handlersfakes.FakeExchanger{}
					}

					fakeWriter.RemoteAddrReturns(remoteAddrReturns)
					recursionHandler := handlers.NewForwardHandler(fakeRecursorPool, fakeExchangerFactory, fakeClock, fakeLogger, fakeTruncater)

					var casedQname string
					m := &dns.Msg{}
					SetQuestion(m, &casedQname, "example.com.", dns.TypeANY)

					recursionHandler.ServeDNS(fakeWriter, m)
					message := fakeWriter.WriteMsgArgsForCall(0)
					Expect(message.Rcode).To(Equal(dns.RcodeSuccess))
					Expect(message.Answer).To(Equal(recursorAnswer.Answer))

					Expect(fakeExchanger.ExchangeCallCount()).To(Equal(1))
					msg, recursor := fakeExchanger.ExchangeArgsForCall(0)
					Expect(recursor).To(Equal("127.0.0.1"))
					Expect(msg).To(Equal(m))

					Expect(fakeLogger.DebugCallCount()).To(Equal(2))

					logTag, logMessage, _ := fakeLogger.DebugArgsForCall(1)
					Expect(logTag).To(Equal("ForwardHandler"))
					Expect(logMessage).To(Equal(fmt.Sprintf("handlers.ForwardHandler Request id=%d qtype=[ANY] qname=["+casedQname+"] rcode=NOERROR ancount=1 recursor=127.0.0.1 time=0ns", m.Id)))
				},
				Entry("forwards query to recursor via udp for udp clients", "udp", nil),
				Entry("forwards query to recursor via tcp for tcp clients", "tcp", &net.TCPAddr{}),
			)

			Context("i/o timout without retry", func() {
				var (
					requestMessage *dns.Msg
				)

				BeforeEach(func() {
					fakeExchanger := &handlersfakes.FakeExchanger{}
					fakeExchangerFactory := func(net string) handlers.Exchanger { return fakeExchanger }
					recursionHandler = handlers.NewForwardHandler(fakeRecursorPool, fakeExchangerFactory, fakeClock, fakeLogger, fakeTruncater)
					requestMessage = &dns.Msg{}
					SetQuestion(requestMessage, nil, "example.com.", dns.TypeANY)
					o := &net.DNSError{
						IsTimeout: true,
					}
					fakeExchanger.ExchangeReturns(&dns.Msg{}, 0, o)
				})

				It("server failure response based on i/o timeout", func() {
					fakeWriter.RemoteAddrReturns(&net.TCPAddr{})

					recursionHandler.ServeDNS(fakeWriter, requestMessage)
					Expect(fakeTruncater.TruncateIfNeededCallCount()).To(Equal(0))
					message := fakeWriter.WriteMsgArgsForCall(0)
					Expect(message.Rcode).To(Equal(dns.RcodeServerFailure))
				})
			})

			Context("i/o timeout with retry", func() {
				var (
					requestMessage *dns.Msg
					dnsServer1     string
					dnsServer2     string
					protocol       string
					maxRetries     int
					initialCall    int
					recursors      []string
					factory        handlers.ExchangerFactory
				)

				BeforeEach(func() {
					suite, _ := GinkgoConfiguration()

					port1 := 62000 + suite.ParallelProcess
					port2 := port1 + suite.ParallelTotal

					dnsServer1 = fmt.Sprintf("127.0.0.1:%d", port1)
					dnsServer2 = fmt.Sprintf("127.0.0.1:%d", port2)
					protocol = "udp"
					maxRetries = 2
					initialCall = 1

					requestMessage = &dns.Msg{}
					SetQuestion(requestMessage, nil, "example.com.", dns.TypeANY)
					factory = handlers.NewExchangerFactory(1 * time.Second)
					recursors = []string{dnsServer1, dnsServer2}
				})

				It("smart recursors with retry", func() {
					pool := handlers.NewFailoverRecursorPool(recursors, config.SmartRecursorSelection, maxRetries, fakeLogger)
					recursionHandler = handlers.NewForwardHandler(pool, factory, fakeClock, fakeLogger, fakeTruncater)

					//create a fake dns endpoint that times out because of no response
					listen, err := net.ListenPacket(protocol, dnsServer1)
					Expect(err).ToNot(HaveOccurred())
					defer listen.Close()

					readBytes1 := make([]byte, 1024)
					var retryCalled int32 = 0
					quit := make(chan struct{})
					defer close(quit)

					go func() {
						for {
							select {
							case <-quit:
								return
							default:
								//ignore sent information, simply timeout
								listen.ReadFrom(readBytes1) //nolint:errcheck
								atomic.AddInt32(&retryCalled, 1)
							}
						}
					}()

					fineListener, err := net.ListenPacket(protocol, dnsServer2)
					Expect(err).ToNot(HaveOccurred())
					defer fineListener.Close()

					readBytes2 := make([]byte, 1024)

					go func() {
						for {
							select {
							case <-quit:
								return
							default:
								//ignore sent information, just respond
								bl, addr, _ := fineListener.ReadFrom(readBytes2)
								fineListener.WriteTo(readBytes2[:bl], addr) //nolint:errcheck
							}
						}
					}()

					recursionHandler.ServeDNS(fakeWriter, requestMessage)
					message := fakeWriter.WriteMsgArgsForCall(0)
					Expect(message.Rcode).To(Equal(dns.RcodeSuccess))
					Expect(atomic.LoadInt32(&retryCalled)).To(Equal(int32(maxRetries + initialCall)))
				})

				It("serial recursors with retry", func() {
					pool := handlers.NewFailoverRecursorPool(recursors, config.SerialRecursorSelection, maxRetries, fakeLogger)
					recursionHandler = handlers.NewForwardHandler(pool, factory, fakeClock, fakeLogger, fakeTruncater)

					//create a fake dns endpoint that times out because of no response
					listen, err := net.ListenPacket(protocol, dnsServer1)
					Expect(err).ToNot(HaveOccurred())
					defer listen.Close()

					readBytes1 := make([]byte, 1024)
					var retryCalled int32 = 0
					quit := make(chan struct{})
					defer close(quit)

					go func() {
						for {
							select {
							case <-quit:
								return
							default:
								//ignore sent information, simply timeout
								listen.ReadFrom(readBytes1) //nolint:errcheck
								atomic.AddInt32(&retryCalled, 1)
							}
						}
					}()

					fineListener, err := net.ListenPacket(protocol, dnsServer2)
					Expect(err).ToNot(HaveOccurred())
					defer fineListener.Close()

					Expect(fineListener.SetReadDeadline(time.Time{})).To(Succeed())
					readBytes2 := make([]byte, 1024)

					go func() {
						for {
							select {
							case <-quit:
								return
							default:
								//ignore sent information, just respond
								bl, addr, _ := fineListener.ReadFrom(readBytes2)
								fineListener.WriteTo(readBytes2[:bl], addr) //nolint:errcheck
							}
						}
					}()

					recursionHandler.ServeDNS(fakeWriter, requestMessage)
					message := fakeWriter.WriteMsgArgsForCall(0)
					Expect(message.Rcode).To(Equal(dns.RcodeSuccess))
					Expect(atomic.LoadInt32(&retryCalled)).To(Equal(int32(maxRetries + initialCall)))
				})
			})

			Context("truncation", func() {
				var (
					requestMessage *dns.Msg
					recursorAnswer *dns.Msg
				)

				BeforeEach(func() {
					recursorAnswer = &dns.Msg{
						Answer: []dns.RR{&dns.A{A: net.ParseIP("99.99.99.99")}},
					}
					fakeExchanger := &handlersfakes.FakeExchanger{}
					fakeExchangerFactory := func(net string) handlers.Exchanger { return fakeExchanger }
					recursionHandler = handlers.NewForwardHandler(fakeRecursorPool, fakeExchangerFactory, fakeClock, fakeLogger, fakeTruncater)
					requestMessage = &dns.Msg{}
					SetQuestion(requestMessage, nil, "example.com.", dns.TypeANY)
					fakeExchanger.ExchangeReturns(recursorAnswer, 0, nil)
				})

				It("passes the response to the truncater", func() {
					fakeWriter.RemoteAddrReturns(&net.TCPAddr{})

					recursionHandler.ServeDNS(fakeWriter, requestMessage)
					Expect(fakeTruncater.TruncateIfNeededCallCount()).To(Equal(1))
					writer, req, resp := fakeTruncater.TruncateIfNeededArgsForCall(0)
					Expect(writer).To(Equal(fakeWriter))
					Expect(req).To(Equal(requestMessage))
					Expect(resp).To(Equal(recursorAnswer))
				})
			})

			Context("when a recursor fails", func() {
				var (
					msg *dns.Msg
				)
				var casedQname string

				BeforeEach(func() {
					fakeExchanger.ExchangeReturns(&dns.Msg{}, 0, errors.New("failed to exchange"))

					msg = &dns.Msg{}
					SetQuestion(msg, &casedQname, "example.com.", dns.TypeANY)

					recursionHandler.ServeDNS(fakeWriter, msg)
				})

				It("writes a failure result", func() {
					Expect(fakeLogger.DebugCallCount()).To(Equal(2))
					Expect(fakeLogger.ErrorCallCount()).To(Equal(2))
					tag, msg, args := fakeLogger.ErrorArgsForCall(0)
					Expect(tag).To(Equal("ForwardHandler"))
					Expect(fmt.Sprintf(msg, args...)).To(Equal(`error recursing for ` + casedQname + ` to "127.0.0.1": failed to exchange`))

					tag, msg, args = fakeLogger.ErrorArgsForCall(1)
					Expect(tag).To(Equal("ForwardHandler"))
					Expect(fmt.Sprintf(msg, args...)).To(Equal(`error recursing for ` + casedQname + ` to "10.244.5.4": failed to exchange`))
				})

				Context("when all recursors fail", func() {
					It("returns a server failure", func() {
						Expect(fakeWriter.WriteMsgCallCount()).To(Equal(1))

						message := fakeWriter.WriteMsgArgsForCall(0)
						Expect(message.Question).To(Equal(msg.Question))
						Expect(message.Rcode).To(Equal(dns.RcodeNameError))
						Expect(message.Authoritative).To(Equal(false))
						Expect(message.RecursionAvailable).To(Equal(false))
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
				SetQuestion(m, nil, "example.com.", dns.TypeANY)

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
					SetQuestion(m, nil, "example.com.", dns.TypeANY)

					recursionHandler.ServeDNS(fakeWriter, m)

					Expect(fakeLogger.ErrorCallCount()).To(Equal(1))
					tag, msg, args := fakeLogger.ErrorArgsForCall(0)
					Expect(tag).To(Equal("ForwardHandler"))
					Expect(fmt.Sprintf(msg, args...)).To(Equal("error writing response: failed to write message"))
				})
			})
		})
	})
})
