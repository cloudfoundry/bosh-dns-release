package server_test

import (
	"fmt"
	"strconv"

	"bosh-dns/dns/server"

	"github.com/cloudfoundry/bosh-utils/logger/fakes"

	"errors"

	"net"
	"time"

	"sync"

	"bosh-dns/dns/server/internal/internalfakes"
	"bosh-dns/dns/server/serverfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func getFreePort() (int, error) {
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0, err
	}
	l.Close()

	_, port, err := net.SplitHostPort(l.Addr().String())
	if err != nil {
		return 0, err
	}

	intPort, err := strconv.Atoi(port)
	if err != nil {
		return 0, err
	}

	return intPort, nil
}

func tcpServerStub(bindAddress string, stop chan struct{}) func() error {
	return func() error {
		_, err := net.Listen("tcp", bindAddress)
		if err != nil {
			return err
		}

		select {
		case <-stop:
		}

		return nil
	}
}

func udpServerStub(bindAddress string, timeout time.Duration, stop chan struct{}) func() error {
	return func() error {
		listener, err := net.ListenPacket("udp", bindAddress)

		if err != nil {
			return err
		}
		listener.SetDeadline(time.Now().Add(timeout + (10 * time.Second)))

		buf := make([]byte, 1)
		for {
			n, addr, err := listener.ReadFrom(buf)
			if err != nil {
				return err
			}

			if n == 0 {
				return errors.New("not enough bytes read")
			}

			if _, err := listener.WriteTo(buf, addr); err != nil {
				return err
			}

			select {
			case <-stop:
			default:
			}
		}
	}
}

func notListeningStub(stop chan struct{}) func() error {
	return func() error {
		select {
		case <-stop:
		}

		return nil
	}
}

func passthroughCheck(reactiveAnswerChan chan error) *serverfakes.FakeUpcheck {
	return &serverfakes.FakeUpcheck{
		IsUpStub: func() error {
			return <-reactiveAnswerChan
		},
	}
}

func upCheck() *serverfakes.FakeUpcheck {
	return &serverfakes.FakeUpcheck{
		IsUpStub: func() error {
			return nil
		},
	}
}

func downCheck() *serverfakes.FakeUpcheck {
	return &serverfakes.FakeUpcheck{
		IsUpStub: func() error {
			return errors.New("fake down")
		},
	}
}

func shutdownStub(err error) func() error {
	return func() error {
		return err
	}
}

var _ = Describe("Server", func() {
	var (
		dnsServer       server.Server
		fakeTCPServer   *serverfakes.FakeDNSServer
		fakeUDPServer   *serverfakes.FakeDNSServer
		tcpUpcheck      *serverfakes.FakeUpcheck
		udpUpcheck      *serverfakes.FakeUpcheck
		fakeDialer      server.Dialer
		timeout         time.Duration
		bindAddress     string
		lock            sync.Mutex
		pollingInterval time.Duration
		shutdownChannel chan struct{}
		stopFakeServer  chan struct{}
		logger          *fakes.FakeLogger
	)

	BeforeEach(func() {
		stopFakeServer = make(chan struct{})
		shutdownChannel = make(chan struct{})
		timeout = 1 * time.Second

		port, err := getFreePort()
		Expect(err).NotTo(HaveOccurred())

		bindAddress = fmt.Sprintf("127.0.0.1:%d", port)

		fakeTCPServer = &serverfakes.FakeDNSServer{}
		fakeUDPServer = &serverfakes.FakeDNSServer{}
		fakeTCPServer.ListenAndServeStub = tcpServerStub(bindAddress, stopFakeServer)
		fakeUDPServer.ListenAndServeStub = udpServerStub(bindAddress, timeout, stopFakeServer)

		logger = &fakes.FakeLogger{}

		tcpUpcheck = upCheck()
		udpUpcheck = upCheck()

		SetDefaultEventuallyTimeout(timeout + 2*time.Second)
		SetDefaultConsistentlyDuration(timeout + 2*time.Second)

		pollingInterval = 5 * time.Second

		fakeDialer = net.Dial
	})

	JustBeforeEach(func() {
		dnsServer = server.New(
			[]server.DNSServer{fakeTCPServer, fakeUDPServer},
			[]server.Upcheck{tcpUpcheck, udpUpcheck},
			timeout,
			pollingInterval,
			shutdownChannel,
			logger,
		)
	})

	AfterEach(func() {
		close(stopFakeServer)
	})

	Context("Run", func() {
		Context("when the timeout has been reached", func() {
			Context("and the servers are not up", func() {
				BeforeEach(func() {
					tcpUpcheck = downCheck()
					udpUpcheck = downCheck()
				})
				It("returns an error", func() {
					fakeTCPServer.ListenAndServeStub = notListeningStub(stopFakeServer)
					fakeUDPServer.ListenAndServeStub = notListeningStub(stopFakeServer)

					dnsServerFinished := make(chan error)
					go func() {
						dnsServerFinished <- dnsServer.Run()
					}()

					Eventually(dnsServerFinished).Should(Receive(Equal(errors.New("timed out waiting for server to bind"))))
				})
			})
		})

		Context("when a provided upcheck server cannot listen and serve", func() {
			BeforeEach(func() {
				tcpUpcheck = downCheck()
				udpUpcheck = downCheck()
			})

			It("should return an error when the tcp server cannot listen and serve", func() {
				fakeTCPServer.ListenAndServeReturns(errors.New("some-fake-tcp-error"))

				err := dnsServer.Run()
				Expect(err).To(MatchError("some-fake-tcp-error"))
			})

			It("should return an error when the udp server cannot listen and serve", func() {
				fakeUDPServer.ListenAndServeReturns(errors.New("some-fake-udp-error"))

				err := dnsServer.Run()
				Expect(err).To(MatchError("some-fake-udp-error"))
			})
		})

		Context("upchecking", func() {
			var fakeConn *internalfakes.FakeConn

			BeforeEach(func() {
				fakeConn = &internalfakes.FakeConn{}
			})

			Context("when both servers are up", func() {
				var fakeProtocolConn map[string]*internalfakes.FakeConn
				var fakeProtocolDialConn map[string]int

				BeforeEach(func() {
					fakeProtocolConn = map[string]*internalfakes.FakeConn{
						"tcp": {},
						"udp": {},
					}

					fakeProtocolDialConn = map[string]int{
						"tcp": 0,
						"udp": 0,
					}

					fakeDialer = func(protocol, address string) (net.Conn, error) {
						lock.Lock()
						fakeProtocolDialConn[protocol]++
						lock.Unlock()

						return fakeProtocolConn[protocol], nil
					}
				})

				It("blocks forever", func() {
					dnsServerFinished := make(chan error)
					go func() {
						dnsServerFinished <- dnsServer.Run()
					}()

					Consistently(dnsServerFinished).ShouldNot(Receive())

					Expect(fakeProtocolConn["tcp"].CloseCallCount()).To(Equal(fakeProtocolDialConn["tcp"]))
					Expect(fakeProtocolConn["udp"].CloseCallCount()).To(Equal(fakeProtocolDialConn["udp"]))
					Expect(logger.DebugCallCount()).To(BeNumerically(">", 0))
					tag, msg, _ := logger.DebugArgsForCall(0)
					Expect(tag).To(Equal("server"))
					Expect(msg).To(Equal("done with upchecks"))
				})
			})

			Describe("self-correcting runtime upchecks", func() {
				triggerNFailures := func(out chan error, N chan int, notifyDone chan int) {
					for {
						select {
						case numFailures := <-N:
							var i int
							for i = 0; i < numFailures; i++ {
								out <- errors.New("deadbeef")
							}
							notifyDone <- i
						default:
							out <- nil
						}
					}
				}

				var (
					startFailing    chan int
					numFailuresSent chan int
					upChan          chan error
				)

				BeforeEach(func() {
					pollingInterval = 50 * time.Millisecond
					startFailing = make(chan int)
					numFailuresSent = make(chan int)
					upChan = make(chan error)
				})

				Context("when the TCP server suddenly stops working", func() {
					BeforeEach(func() {
						udpUpcheck = upCheck()
						tcpUpcheck = passthroughCheck(upChan)
					})

					It("kills itself after five failures in a row", func() {
						go triggerNFailures(upChan, startFailing, numFailuresSent)

						dnsServerFinished := make(chan error)
						go func() {
							dnsServerFinished <- dnsServer.Run()
						}()

						Consistently(dnsServerFinished).ShouldNot(Receive())
						startFailing <- 5
						Expect(<-numFailuresSent).To(Equal(5))
						Eventually(dnsServerFinished).Should(Receive())
					})

					It("recovers if the failures are not consistent", func() {
						go triggerNFailures(upChan, startFailing, numFailuresSent)

						dnsServerFinished := make(chan error)
						go func() {
							dnsServerFinished <- dnsServer.Run()
						}()

						Consistently(dnsServerFinished).ShouldNot(Receive())
						startFailing <- 3
						Expect(<-numFailuresSent).To(Equal(3))
						Consistently(dnsServerFinished).ShouldNot(Receive())
						startFailing <- 3
						Expect(<-numFailuresSent).To(Equal(3))
						Consistently(dnsServerFinished).ShouldNot(Receive())
					})
				})

				Context("when the UDP server suddenly stops working", func() {
					BeforeEach(func() {
						udpUpcheck = passthroughCheck(upChan)
						tcpUpcheck = upCheck()
					})

					It("kills itself after five failures in a row", func() {
						go triggerNFailures(upChan, startFailing, numFailuresSent)

						dnsServerFinished := make(chan error)
						go func() {
							dnsServerFinished <- dnsServer.Run()
						}()

						Consistently(dnsServerFinished).ShouldNot(Receive())
						startFailing <- 5
						Expect(<-numFailuresSent).To(Equal(5))
						Eventually(dnsServerFinished).Should(Receive())
					})

					It("recovers if the failures are not consistent", func() {
						go triggerNFailures(upChan, startFailing, numFailuresSent)

						dnsServerFinished := make(chan error)
						go func() {
							dnsServerFinished <- dnsServer.Run()
						}()

						Consistently(dnsServerFinished).ShouldNot(Receive())
						startFailing <- 3
						Expect(<-numFailuresSent).To(Equal(3))
						Consistently(dnsServerFinished).ShouldNot(Receive())
						startFailing <- 3
						Expect(<-numFailuresSent).To(Equal(3))
						Consistently(dnsServerFinished).ShouldNot(Receive())
					})
				})
			})

			Context("when no upchecks are configured", func() {
				JustBeforeEach(func() {
					dnsServer = server.New(
						[]server.DNSServer{fakeTCPServer, fakeUDPServer},
						[]server.Upcheck{},
						timeout,
						pollingInterval,
						shutdownChannel,
						logger,
					)
				})

				It("logs a message to that effect", func() {
					dnsServerFinished := make(chan error)
					go func() {
						dnsServerFinished <- dnsServer.Run()
					}()

					Consistently(dnsServerFinished).ShouldNot(Receive())

					Expect(logger.WarnCallCount()).To(Equal(1))
					tag, msg, _ := logger.WarnArgsForCall(0)
					Expect(tag).To(Equal("server"))
					Expect(msg).To(Equal("proceeding immediately: no upchecks configured"))
				})
			})

			Context("when the udp server never binds to a port", func() {
				BeforeEach(func() {
					udpUpcheck = downCheck()
				})

				It("returns an error", func() {
					fakeUDPServer.ListenAndServeStub = notListeningStub(stopFakeServer)

					dnsServerFinished := make(chan error)
					go func() {
						dnsServerFinished <- dnsServer.Run()
					}()

					Eventually(dnsServerFinished).Should(Receive(Equal(errors.New("timed out waiting for server to bind"))))
					Expect(logger.DebugCallCount()).To(BeNumerically(">", 1))
					Expect(logger.DebugCallCount()).To(BeNumerically("<", 100))
					_, msg, _ := logger.DebugArgsForCall(1)
					Expect(msg).To(ContainSubstring("waiting for server to come up"))
				})
			})

			Context("when the tcp server never binds to a port", func() {
				BeforeEach(func() {
					tcpUpcheck = downCheck()
				})

				It("returns an error", func() {
					fakeTCPServer.ListenAndServeStub = notListeningStub(stopFakeServer)

					dnsServerFinished := make(chan error)
					go func() {
						dnsServerFinished <- dnsServer.Run()
					}()

					Eventually(dnsServerFinished).Should(Receive(Equal(errors.New("timed out waiting for server to bind"))))
					Expect(logger.DebugCallCount()).To(BeNumerically(">", 1))
					Expect(logger.DebugCallCount()).To(BeNumerically("<", 100))
				})
			})

			Context("shutdown signal", func() {
				It("gracefully terminates the servers when the shutdown signal has been fired", func() {
					dnsServerFinished := make(chan error)
					go func() {
						dnsServerFinished <- dnsServer.Run()
					}()

					Consistently(dnsServerFinished).ShouldNot(Receive())

					close(shutdownChannel)
					Eventually(dnsServerFinished).Should(Receive(nil))

					Expect(fakeTCPServer.ShutdownCallCount()).To(Equal(1))
					Expect(fakeUDPServer.ShutdownCallCount()).To(Equal(1))
				})

				It("returns an error if the servers were unable to shutdown cleanly", func() {
					err := errors.New("failed to shutdown tcp server")
					fakeTCPServer.ShutdownReturns(err)
					dnsServerFinished := make(chan error)
					go func() {
						dnsServerFinished <- dnsServer.Run()
					}()

					Consistently(dnsServerFinished).ShouldNot(Receive())

					close(shutdownChannel)
					Eventually(dnsServerFinished).Should(Receive(Equal(err)))

					Expect(fakeTCPServer.ShutdownCallCount()).To(Equal(1))
					Expect(fakeUDPServer.ShutdownCallCount()).To(Equal(1))
				})
			})
		})
	})
})
