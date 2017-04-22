package server_test

import (
	"errors"
	"net"

	"github.com/cloudfoundry/dns-release/src/dns/server"
	"github.com/cloudfoundry/dns-release/src/dns/server/internal/internalfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"time"
)

var _ = Describe("Healthcheck", func() {
	Context("UDPHealthCheck", func() {
		var subject server.UDPHealthCheck
		var fakeDialer server.Dialer
		var fakeConn *internalfakes.FakeConn

		BeforeEach(func() {
			fakeConn = &internalfakes.FakeConn{}
		})

		JustBeforeEach(func() {
			subject = server.NewUDPHealthCheck(fakeDialer, "127.0.0.1:53")
		})

		Context("when the udp health checking fails", func() {
			Context("dialing fails", func() {
				BeforeEach(func() {
					fakeDialer = func(protocol, address string) (net.Conn, error) {
						return nil, errors.New("failed to dial")
					}
				})

				It("returns with error", func() {
					err := subject.IsHealthy()

					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("failed to dial"))
				})
			})

			Context("writing udp payload fails", func() {
				BeforeEach(func() {
					fakeDialer = func(protocol, address string) (net.Conn, error) {
						return fakeConn, nil
					}
				})

				It("returns with error", func() {
					fakeConn.WriteReturns(0, errors.New("fake write error"))

					err := subject.IsHealthy()

					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("fake write error"))
					Expect(fakeConn.CloseCallCount()).To(BeNumerically(">", 0))
				})
			})

			Context("reading udp payload fails", func() {
				BeforeEach(func() {
					fakeDialer = func(protocol, address string) (net.Conn, error) {
						return fakeConn, nil
					}
				})

				It("returns with error", func() {
					fakeConn.ReadReturns(0, errors.New("fake read error"))

					err := subject.IsHealthy()

					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("fake read error"))
					Expect(fakeConn.CloseCallCount()).To(BeNumerically(">", 0))
				})
			})
		})

		Context("when the udp health checking succeeds", func() {
			BeforeEach(func() {
				fakeDialer = func(protocol, address string) (net.Conn, error) {
					return fakeConn, nil
				}
			})

			It("returns nil", func() {
				beforeTime := time.Now()

				err := subject.IsHealthy()
				Expect(err).NotTo(HaveOccurred())
				Expect(fakeConn.SetReadDeadlineArgsForCall(0)).To(BeTemporally(">=", beforeTime.Add(500*time.Millisecond)))
			})
		})
	})

	Context("TCPHealthCheck", func() {
		var subject server.TCPHealthCheck
		var fakeDialer server.Dialer
		var fakeConn *internalfakes.FakeConn

		BeforeEach(func() {
			fakeConn = &internalfakes.FakeConn{}
		})

		JustBeforeEach(func() {
			subject = server.NewTCPHealthCheck(fakeDialer, "127.0.0.1:53")
		})

		Context("when the tcp health checking fails", func() {
			Context("dialing fails", func() {
				BeforeEach(func() {
					fakeDialer = func(protocol, address string) (net.Conn, error) {
						return nil, errors.New("failed to dial")
					}
				})

				It("returns with error", func() {
					err := subject.IsHealthy()

					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("failed to dial"))
				})
			})
		})

		Context("when the tcp health checking succeeds", func() {
			BeforeEach(func() {
				fakeDialer = func(protocol, address string) (net.Conn, error) {
					return fakeConn, nil
				}
			})

			It("returns nil", func() {
				err := subject.IsHealthy()
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
})
