package server

import "net"

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

	_, err = conn.Read(make([]byte, 1))
	if err != nil {
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
