package server

import (
	"errors"
	"net"
	"sync"
	"time"
)

type Dialer func(string, string) (net.Conn, error)

//go:generate counterfeiter . ListenAndServer
type ListenAndServer interface {
	ListenAndServe() error
}

type Server struct {
	udpServer   ListenAndServer
	tcpServer   ListenAndServer
	timeout     time.Duration
	dial        Dialer
	bindAddress string
}

func (s Server) ListenAndServe() error {
	err := make(chan error)
	go func() {
		err <- s.tcpServer.ListenAndServe()
	}()

	go func() {
		err <- s.udpServer.ListenAndServe()
	}()

	done := make(chan struct{})
	wg := &sync.WaitGroup{}
	wg.Add(2)

	go func() {
		wg.Wait()
		close(done)
	}()

	go func() {
		for {
			_, err := s.dial("tcp", s.bindAddress)
			if err == nil {
				break
			}
		}

		wg.Done()
	}()

	go func() {
		for {
			conn, err := s.dial("udp", s.bindAddress)
			if err != nil {
				continue
			}

			if _, err := conn.Write([]byte{}); err != nil {
				continue
			}

			_, err = conn.Read(make([]byte, 1))
			if err == nil {
				break
			}
		}

		wg.Done()
	}()

	select {
	case e := <-err:
		return e
	case <-time.After(s.timeout):
		return errors.New("timed out waiting for server to bind")
	case <-done:
	}

	select {}
}

func New(tcpServer ListenAndServer, udpServer ListenAndServer, dial Dialer, timeout time.Duration, bindAddress string) Server {
	return Server{
		tcpServer:   tcpServer,
		udpServer:   udpServer,
		timeout:     timeout,
		dial:        dial,
		bindAddress: bindAddress,
	}
}
