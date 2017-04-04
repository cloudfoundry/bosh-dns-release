package server

import (
	"errors"
	"sync"
	"time"
)

//go:generate counterfeiter . DNSServer
type DNSServer interface {
	ListenAndServe() error
	Shutdown() error
}

type Server struct {
	servers      []DNSServer
	healthchecks []HealthCheck
	timeout      time.Duration
	shutdownChan chan struct{}
}

func New(servers []DNSServer, healthchecks []HealthCheck, timeout time.Duration, shutdownChan chan struct{}) Server {
	return Server{
		servers:      servers,
		healthchecks: healthchecks,
		timeout:      timeout,
		shutdownChan: shutdownChan,
	}
}

func (s Server) Run() error {
	err := make(chan error)
	s.listenAndServe(err)

	done := make(chan struct{})
	s.doHealthChecks(done)

	select {
	case e := <-err:
		return e
	case <-time.After(s.timeout):
		return errors.New("timed out waiting for server to bind")
	case <-done:
	}

	select {
	case <-s.shutdownChan:
		return s.shutdown()
	}
}

func (s Server) doHealthChecks(done chan struct{}) {
	wg := &sync.WaitGroup{}
	wg.Add(len(s.healthchecks))

	go func() {
		wg.Wait()
		close(done)
	}()

	for _, healthcheck := range s.healthchecks {
		go func(healthcheck HealthCheck) {
			for {
				if err := healthcheck.IsHealthy(); err == nil {
					break
				}
			}

			wg.Done()
		}(healthcheck)
	}
}

func (s Server) listenAndServe(err chan error) {
	for _, server := range s.servers {
		go func(server DNSServer) {
			err <- server.ListenAndServe()
		}(server)
	}
}

func (s Server) shutdown() error {
	err := make(chan error, len(s.servers))

	wg := &sync.WaitGroup{}
	wg.Add(len(s.servers))

	for _, server := range s.servers {
		go func(server DNSServer) {
			err <- server.Shutdown()

			wg.Done()
		}(server)
	}

	wg.Wait()
	close(err)

	for e := range err {
		if e != nil {
			return e
		}
	}

	return nil
}
