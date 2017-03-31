package server_test

import (
	"fmt"

	"github.com/cloudfoundry/dns-release/src/dns/server"

	"errors"

	"net"
	"time"

	"github.com/cloudfoundry/dns-release/src/dns/server/internal/internalfakes"
	"github.com/cloudfoundry/dns-release/src/dns/server/serverfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func getFreePort() (string, error) {
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		return "", err
	}
	l.Close()

	_, port, err := net.SplitHostPort(l.Addr().String())
	if err != nil {
		return "", err
	}

	return port, nil
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

var _ = Describe("Server", func() {
	var (
		dnsServer      server.Server
		fakeTCPServer  *serverfakes.FakeListenAndServer
		fakeUDPServer  *serverfakes.FakeListenAndServer
		timeout        time.Duration
		bindAddress    string
		stopFakeServer chan struct{}
	)

	BeforeEach(func() {
		stopFakeServer = make(chan struct{})
		timeout = 1 * time.Second

		port, err := getFreePort()
		Expect(err).NotTo(HaveOccurred())

		bindAddress = fmt.Sprintf("127.0.0.1:%s", port)

		fakeTCPServer = &serverfakes.FakeListenAndServer{}
		fakeUDPServer = &serverfakes.FakeListenAndServer{}
		fakeTCPServer.ListenAndServeStub = tcpServerStub(bindAddress, stopFakeServer)
		fakeUDPServer.ListenAndServeStub = udpServerStub(bindAddress, timeout, stopFakeServer)

		dnsServer = server.New(fakeTCPServer, fakeUDPServer, net.Dial, timeout, bindAddress)
	})

	AfterEach(func() {
		close(stopFakeServer)
	})

	Context("when the timeout has been reached", func() {
		Context("and the servers are not up", func() {
			It("returns an error", func() {
				fakeTCPServer.ListenAndServeStub = notListeningStub(stopFakeServer)
				fakeUDPServer.ListenAndServeStub = notListeningStub(stopFakeServer)

				dnsServerFinished := make(chan error)
				go func() {
					dnsServerFinished <- dnsServer.ListenAndServe()
				}()

				err := errors.New("timed out waiting for server to bind")
				Eventually(dnsServerFinished, timeout+(2*time.Second)).Should(Receive(&err))
			})
		})

	})

	Context("when both servers are up", func() {
		It("blocks forever", func() {
			dnsServerFinished := make(chan error)
			go func() {
				dnsServerFinished <- dnsServer.ListenAndServe()
			}()

			Consistently(dnsServerFinished, timeout+(2*time.Second)).ShouldNot(Receive())
		})
	})

	Context("when the udp server never binds to a port", func() {
		It("returns an error", func() {
			fakeUDPServer.ListenAndServeStub = notListeningStub(stopFakeServer)

			dnsServerFinished := make(chan error)
			go func() {
				dnsServerFinished <- dnsServer.ListenAndServe()
			}()

			err := errors.New("timed out waiting for server to bind")
			Eventually(dnsServerFinished, timeout+(2*time.Second)).Should(Receive(&err))
		})
	})

	Context("when the tcp server never binds to a port", func() {
		It("returns an error", func() {
			fakeTCPServer.ListenAndServeStub = notListeningStub(stopFakeServer)

			dnsServerFinished := make(chan error)
			go func() {
				dnsServerFinished <- dnsServer.ListenAndServe()
			}()

			err := errors.New("timed out waiting for server to bind")
			Eventually(dnsServerFinished, timeout+(2*time.Second)).Should(Receive(&err))
		})
	})

	Context("when the udp health checking fails", func() {
		Context("dialing fails", func() {
			It("does not exit before the timeout is reached", func() {
				fakeDialer := func(protocol, address string) (net.Conn, error) {
					if protocol == "udp" {
						return nil, errors.New("failed to dial")
					}

					return net.Dial(protocol, address)
				}

				dnsServer = server.New(fakeTCPServer, fakeUDPServer, fakeDialer, timeout, bindAddress)
				dnsServerFinished := make(chan error)
				go func() {
					dnsServerFinished <- dnsServer.ListenAndServe()
				}()

				err := errors.New("timed out waiting for server to bind")
				Eventually(dnsServerFinished, timeout+(2*time.Second)).Should(Receive(&err))
			})
		})

		Context("writing payload fails", func() {
			It("does not exit before the timeout is reached", func() {
				fakeConn := &internalfakes.FakeConn{}
				fakeDialer := func(protocol, address string) (net.Conn, error) {
					if protocol == "udp" {
						return fakeConn, nil
					}

					return net.Dial(protocol, address)
				}
				fakeConn.WriteReturns(0, errors.New("fake write error"))

				dnsServer = server.New(fakeTCPServer, fakeUDPServer, fakeDialer, timeout, bindAddress)
				dnsServerFinished := make(chan error)
				go func() {
					dnsServerFinished <- dnsServer.ListenAndServe()
				}()

				err := errors.New("timed out waiting for server to bind")
				Eventually(dnsServerFinished, timeout+(2*time.Second)).Should(Receive(&err))
			})
		})
	})

	Context("when a provided tcp server cannot listen and serve", func() {
		It("should return an error", func() {
			fakeTCPServer.ListenAndServeReturns(errors.New("some-fake-tcp-error"))

			err := dnsServer.ListenAndServe()
			Expect(err).To(MatchError("some-fake-tcp-error"))
		})
	})

	Context("when a provided udp server cannot listen and serve", func() {
		It("should return an error", func() {
			fakeUDPServer.ListenAndServeReturns(errors.New("some-fake-udp-error"))

			err := dnsServer.ListenAndServe()
			Expect(err).To(MatchError("some-fake-udp-error"))
		})
	})
})
