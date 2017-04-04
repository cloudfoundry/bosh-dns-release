package server

import (
	"errors"
	"sync"
	"time"
)

//go:generate counterfeiter . ListenAndServer
type ListenAndServer interface {
	ListenAndServe() error
}

type Server struct {
	servers      []ListenAndServer
	healthchecks []HealthCheck
	timeout      time.Duration
}

func (s Server) ListenAndServe() error {
	err := make(chan error)

	for _, server := range s.servers {
		go func(server ListenAndServer) {
			err <- server.ListenAndServe()
		}(server)
	}

	done := make(chan struct{})
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

	select {
	case e := <-err:
		return e
	case <-time.After(s.timeout):
		return errors.New("timed out waiting for server to bind")
	case <-done:
	}

	select {}
}

func New(servers []ListenAndServer, healthchecks []HealthCheck, timeout time.Duration) Server {
	return Server{
		servers:      servers,
		healthchecks: healthchecks,
		timeout:      timeout,
	}
}
