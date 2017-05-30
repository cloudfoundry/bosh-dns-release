package server

import (
	"errors"
	"sync"
	"time"

	"github.com/cloudfoundry/bosh-utils/logger"
)

//go:generate counterfeiter . DNSServer
type DNSServer interface {
	ListenAndServe() error
	Shutdown() error
}

type Server struct {
	servers             []DNSServer
	healthchecks        []HealthCheck
	timeout             time.Duration
	healthcheckInterval time.Duration
	shutdownChan        chan struct{}
	logger              logger.Logger
}

func New(servers []DNSServer, healthchecks []HealthCheck, timeout, healthcheckInterval time.Duration, shutdownChan chan struct{}, logger logger.Logger) Server {
	return Server{
		servers:             servers,
		healthchecks:        healthchecks,
		timeout:             timeout,
		shutdownChan:        shutdownChan,
		healthcheckInterval: healthcheckInterval,
		logger:              logger,
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
		s.logger.Debug("server", "done with health checks")
	}

	s.monitorHealthChecks()

	select {
	case <-s.shutdownChan:
		return s.shutdown()
	}
}

func (s Server) monitorHealthChecks() {
	for _, healthcheck := range s.healthchecks {
		go func(h HealthCheck, limit int) {
			danger := 0
			for {
				if err := h.IsHealthy(); err != nil {
					danger += 1
					if danger >= limit && s.shutdownChan != nil {
						close(s.shutdownChan)
						s.shutdownChan = nil
						return
					}
				} else {
					danger = 0
				}

				time.Sleep(s.healthcheckInterval)
			}
		}(healthcheck, 5)
	}
}

func (s Server) doHealthChecks(done chan struct{}) {
	wg := &sync.WaitGroup{}
	wg.Add(len(s.healthchecks))

	if len(s.healthchecks) == 0 {
		s.logger.Warn("server", "proceeding immediately: no healthchecks configured")
		close(done)
		return
	}

	go func() {
		wg.Wait()
		close(done)
	}()

	for _, healthcheck := range s.healthchecks {
		go func(healthcheck HealthCheck) {
			for {
				var err error
				if err = healthcheck.IsHealthy(); err == nil {
					break
				}
				s.logger.Debug("healthcheck", "waiting for server to come up", err)

				time.Sleep(50 * time.Millisecond)
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
