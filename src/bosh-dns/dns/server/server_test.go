package server_test

import (
	"fmt"

	"bosh-dns/dns/server"

	"github.com/cloudfoundry/bosh-utils/logger/fakes"

	"errors"

	"net"
	"time"

	"bosh-dns/dns/server/serverfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"bosh-dns/dns/internal/testhelpers"
)

func tcpServerStub(bindAddress string, stop chan struct{}) func() error {
	return func() error {
		listener, err := net.Listen("tcp", bindAddress)
		if err != nil {
			return err
		}
		defer func() {
			err := listener.Close()
			Expect(err).NotTo(HaveOccurred())
		}()

		<-stop

		return nil
	}
}

func udpServerStub(bindAddress string, timeout time.Duration, stop chan struct{}) func() error {
	return func() error {
		listener, err := net.ListenPacket("udp", bindAddress)
		if err != nil {
			return err
		}
		go func() {
			defer GinkgoRecover()
			<-stop
			err := listener.Close()
			Expect(err).NotTo(HaveOccurred())
		}()

		listener.SetDeadline(time.Now().Add(timeout + (1 * time.Second)))

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
		timeout         time.Duration
		bindAddress     string
		pollingInterval time.Duration
		shutdownChannel chan struct{}
		stopFakeServer  chan struct{}
		logger          *fakes.FakeLogger
	)

	BeforeEach(func() {
		stopFakeServer = make(chan struct{})
		shutdownChannel = make(chan struct{})
		timeout = 1 * time.Second

		port, err := testhelpers.GetFreePort()
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
		if shutdownChannel != nil {
			close(shutdownChannel)
		}
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
			Context("when both servers are up", func() {
				It("proceeds through upchecks", func() {
					dnsServerFinished := make(chan error)
					go func() {
						dnsServerFinished <- dnsServer.Run()
					}()

					Eventually(logger.DebugCallCount).Should(BeNumerically(">", 0))
					tag, msg, _ := logger.DebugArgsForCall(0)
					Expect(tag).To(Equal("server"))
					Expect(msg).To(Equal("done with upchecks"))
				})
			})

			Describe("self-correcting runtime upchecks", func() {
				triggerNFailures := func(out chan error, N chan int, notifyDone chan int) {
					out <- nil
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

						startFailing <- 5
						Eventually(numFailuresSent).Should(Receive(Equal(5)))
						Eventually(dnsServerFinished).Should(Receive())
						Eventually(shutdownChannel).Should(BeClosed())
						shutdownChannel = nil
					})

					It("recovers if the failures are not consistent", func() {
						go triggerNFailures(upChan, startFailing, numFailuresSent)

						dnsServerFinished := make(chan error)
						go func() {
							dnsServerFinished <- dnsServer.Run()
						}()

						startFailing <- 3
						Eventually(numFailuresSent).Should(Receive(Equal(3)))

						Consistently(dnsServerFinished, 3*pollingInterval).ShouldNot(Receive())

						startFailing <- 3
						Eventually(numFailuresSent).Should(Receive(Equal(3)))

						Consistently(dnsServerFinished, 3*pollingInterval).ShouldNot(Receive())
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

						startFailing <- 5
						Eventually(numFailuresSent).Should(Receive(Equal(5)))
						Eventually(dnsServerFinished).Should(Receive())
						Eventually(shutdownChannel).Should(BeClosed())
						shutdownChannel = nil
					})

					It("recovers if the failures are not consistent", func() {
						go triggerNFailures(upChan, startFailing, numFailuresSent)

						dnsServerFinished := make(chan error)
						go func() {
							dnsServerFinished <- dnsServer.Run()
						}()

						startFailing <- 3
						Eventually(numFailuresSent).Should(Receive(Equal(3)))

						Consistently(dnsServerFinished, 3*pollingInterval).ShouldNot(Receive())

						startFailing <- 3
						Eventually(numFailuresSent).Should(Receive(Equal(3)))
						Consistently(dnsServerFinished, 3*pollingInterval).ShouldNot(Receive())
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

					Eventually(logger.WarnCallCount).Should(Equal(1))
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

					close(shutdownChannel)
					shutdownChannel = nil
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

					close(shutdownChannel)
					shutdownChannel = nil
					Eventually(dnsServerFinished).Should(Receive(Equal(err)))

					Expect(fakeTCPServer.ShutdownCallCount()).To(Equal(1))
					Expect(fakeUDPServer.ShutdownCallCount()).To(Equal(1))
				})
			})
		})
	})
})
