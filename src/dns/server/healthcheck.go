package server

import (
	"net"
	"time"
)

type Dialer func(string, string) (net.Conn, error)

//go:generate counterfeiter . HealthCheck
type HealthCheck interface {
	IsHealthy() error
}

type UDPHealthCheck struct {
	dial   Dialer
	target string
}

func NewUDPHealthCheck(dial Dialer, target string) UDPHealthCheck {
	return UDPHealthCheck{
		dial:   dial,
		target: target,
	}
}

func (hc UDPHealthCheck) IsHealthy() error {
	conn, err := hc.dial("udp", hc.target)
	if err != nil {
		return err
	}

	defer conn.Close()

	if _, err := conn.Write([]byte{0x00}); err != nil {
		return err
	}

	conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))

	// This value needs to have a length of at least 12, otherwise Windows will fail with an error like:
	//
	// A message sent on a datagram socket was larger than the internal message buffer or some other network
	// limit, or the buffer used to receive a datagram into was smaller than the datagram itself.
	//
	// This is likely due to the fact that Windows requires a buffer that is large enough to at least hold
	// a UDP header
	if _, err := conn.Read(make([]byte, 12)); err != nil {
		return err
	}

	return nil
}

type TCPHealthCheck struct {
	dial   Dialer
	target string
}

func NewTCPHealthCheck(dial Dialer, target string) TCPHealthCheck {
	return TCPHealthCheck{
		dial:   dial,
		target: target,
	}
}

func (hc TCPHealthCheck) IsHealthy() error {
	conn, err := hc.dial("tcp", hc.target)
	if err != nil {
		return err
	}

	conn.Close()

	return nil
}
