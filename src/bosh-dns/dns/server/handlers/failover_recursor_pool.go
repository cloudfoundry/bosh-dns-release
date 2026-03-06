package handlers

import (
	"errors"
	"fmt"
	"net"
	"sync/atomic"

	"github.com/cloudfoundry/bosh-utils/logger"
	"github.com/miekg/dns"

	"bosh-dns/dns/config"
	"bosh-dns/dns/server"
)

const (
	FailHistoryLength    = 25
	FailHistoryThreshold = 5
)

var ErrNoRecursorResponse = errors.New("no response from recursors")

//counterfeiter:generate . RecursorPool

type RecursorPool interface {
	PerformStrategically(func(string) error) error
}

// NewFailoverRecursorPool creates a failover recursor pool based on `recursorSelection`.
//
// When it is "serial", the recursor pool will go in order of the recursors
// list, always starting from the beginning. It does not track history around
// which recursors have failed.
//
// When it is "smart", the recursor pool will randomize the recursors list upon
// the server starting.  It does track history around which recursors have
// failed. This follows the standard DNS specification.
//
// When it is "race", the recursor pool will query all recursors simultaneously
// and return the first successful response.
//
// Each recursor will be queried until one succeeds or all recursors were tried

func NewFailoverRecursorPool(recursors []string, recursorSelection string, RecursorMaxRetries int, logger logger.Logger) RecursorPool {
	recursorSettings := recursorRetrySettings{
		maxRetries: RecursorMaxRetries,
	}

	switch recursorSelection {
	case config.SmartRecursorSelection:
		return newSmartFailoverRecursorPool(recursors, recursorSettings, logger)
	case config.RaceRecursorSelection:
		return newRaceRecursorPool(recursors, recursorSettings, logger)
	default: // serial
		return newSerialFailoverRecursorPool(recursors, recursorSettings, logger)
	}
}

type serialFailoverRecursorPool struct {
	recursors             []string
	logger                logger.Logger
	logTag                string
	recursorRetrySettings recursorRetrySettings
}

type smartFailoverRecursorPool struct {
	preferredRecursorIndex uint64

	logger                logger.Logger
	logTag                string
	recursors             []recursorWithHistory
	recursorRetrySettings recursorRetrySettings
}

type recursorRetrySettings struct {
	maxRetries int
}

type recursorWithHistory struct {
	name       string
	failBuffer chan bool
	failCount  int32
}

type raceRecursorPool struct {
	recursors             []string
	logger                logger.Logger
	logTag                string
	recursorRetrySettings recursorRetrySettings
}

type raceResult struct {
	recursor string
	err      error
	priority int // Lower is better: 0=success, 1=NXDOMAIN, 2=SERVFAIL, 3=network error
}

func newSerialFailoverRecursorPool(recursors []string, recursorSettings recursorRetrySettings, logger logger.Logger) RecursorPool {
	return &serialFailoverRecursorPool{
		recursors,
		logger,
		"SerialFailoverRecursor",
		recursorSettings,
	}

}

func newRaceRecursorPool(recursors []string, recursorSettings recursorRetrySettings, logger logger.Logger) RecursorPool {
	if recursors == nil {
		recursors = []string{}
	}

	return &raceRecursorPool{
		recursors:             recursors,
		logger:                logger,
		logTag:                "RaceRecursor",
		recursorRetrySettings: recursorSettings,
	}
}

func newSmartFailoverRecursorPool(recursors []string, recursorSettings recursorRetrySettings, logger logger.Logger) RecursorPool {
	recursorsWithHistory := []recursorWithHistory{}

	if recursors == nil {
		recursors = []string{}
	}

	for _, name := range recursors {
		failBuffer := make(chan bool, FailHistoryLength)
		for i := 0; i < FailHistoryLength; i++ {
			failBuffer <- false
		}

		recursorsWithHistory = append(recursorsWithHistory, recursorWithHistory{
			name:       name,
			failBuffer: failBuffer,
			failCount:  0,
		})
	}

	logTag := "FailoverRecursor"
	if len(recursorsWithHistory) > 0 {
		logger.Info(logTag, fmt.Sprintf("starting preference: %s\n", recursorsWithHistory[0].name))
	}
	return &smartFailoverRecursorPool{
		recursors:              recursorsWithHistory,
		preferredRecursorIndex: 0,
		logger:                 logger,
		logTag:                 logTag,
		recursorRetrySettings:  recursorSettings,
	}
}

func (q *serialFailoverRecursorPool) PerformStrategically(work func(string) error) error {
	for _, r := range q.recursors {
		if err := performWithRetryLogic(work, r, q.recursorRetrySettings.maxRetries, q.logTag, q.logger); err == nil {
			return nil
		}
	}
	return ErrNoRecursorResponse
}

func performWithRetryLogic(work func(string) error, recursor string, maxRetries int, logTag string, log logger.Logger) (err error) {
	for ret := 0; ret <= maxRetries; ret++ {
		err = work(recursor)
		if err == nil {
			return err
		}
		if _, ok := err.(net.Error); !ok {
			return err
		}
		log.Debug(logTag, fmt.Sprintf("dns request network error %s retry [%d/%d] - request count [%d] for recursor %s \n", err.(net.Error), ret, maxRetries, ret+1, recursor))
	}

	//retry count reached
	log.Error(logTag, fmt.Sprintf("write error response to client after retry count reached [%d/%d] with rcode=%d - %s \n", maxRetries, maxRetries, dns.RcodeServerFailure, err.Error()))
	return err
}

func (q *smartFailoverRecursorPool) PerformStrategically(work func(string) error) error {
	offset := atomic.LoadUint64(&q.preferredRecursorIndex)
	uintRecursorCount := uint64(len(q.recursors))

	for i := uint64(0); i < uintRecursorCount; i++ {
		index := int((i + offset) % uintRecursorCount)

		err := performWithRetryLogic(work, q.recursors[index].name, q.recursorRetrySettings.maxRetries, q.logTag, q.logger)
		if err == nil {
			q.registerResult(index, false)
			return nil
		}

		failures := q.registerResult(index, true)
		if i == 0 && failures >= FailHistoryThreshold {
			q.shiftPreference()
		}
	}

	return ErrNoRecursorResponse
}

func (q *smartFailoverRecursorPool) shiftPreference() {
	pri := atomic.AddUint64(&q.preferredRecursorIndex, 1)
	index := pri % uint64(len(q.recursors))
	q.logger.Info(q.logTag, fmt.Sprintf("shifting recursor preference: %s\n", q.recursors[index].name))
}

func (q *smartFailoverRecursorPool) registerResult(index int, wasError bool) int32 {
	failingRecursor := &q.recursors[index]

	oldestResult := <-failingRecursor.failBuffer
	failingRecursor.failBuffer <- wasError

	change := int32(0)

	if oldestResult {
		change--
	}

	if wasError {
		change++
	}

	return atomic.AddInt32(&failingRecursor.failCount, change)
}

func (q *raceRecursorPool) PerformStrategically(work func(string) error) error {
	if len(q.recursors) == 0 {
		return ErrNoRecursorResponse
	}

	results := make(chan raceResult, len(q.recursors))

	for _, recursor := range q.recursors {
		go func(r string) {
			err := performWithRetryLogic(work, r, q.recursorRetrySettings.maxRetries, q.logTag, q.logger)

			priority := q.classifyError(err)

			results <- raceResult{
				recursor: r,
				err:      err,
				priority: priority,
			}
		}(recursor)
	}

	var bestResult *raceResult
	receivedCount := 0
	totalRecursors := len(q.recursors)

	for receivedCount < totalRecursors {
		res := <-results
		receivedCount++

		q.logger.Debug(q.logTag, fmt.Sprintf(
			"received response %d/%d from %s (priority=%d, err=%v)",
			receivedCount, totalRecursors, res.recursor, res.priority, res.err))

		if bestResult == nil || res.priority < bestResult.priority {
			bestResult = &res
		}

		if res.priority == 0 {
			q.logger.Info(q.logTag, fmt.Sprintf(
				"recursor %s returned successful response (received %d/%d responses)",
				res.recursor, receivedCount, totalRecursors))

			go q.drainResults(results, totalRecursors-receivedCount)

			return nil
		}
	}

	if bestResult == nil {
		return ErrNoRecursorResponse
	}

	if bestResult.err != nil {
		q.logger.Info(q.logTag, fmt.Sprintf(
			"all %d recursors responded, best result from %s (priority=%d): %s",
			totalRecursors, bestResult.recursor, bestResult.priority, bestResult.err.Error()))
	} else {
		q.logger.Info(q.logTag, fmt.Sprintf(
			"all %d recursors responded, using result from %s",
			totalRecursors, bestResult.recursor))
	}

	return bestResult.err
}

func (q *raceRecursorPool) classifyError(err error) int {
	if err == nil {
		return 0 // NOERROR - perfect response
	}

	if dnsErr, ok := err.(server.DnsError); ok {
		switch dnsErr.Rcode() {
		case dns.RcodeNameError: // NXDOMAIN
			return 1 // Name doesn't exist - could be correct, but prefer NOERROR to ignore temporary broken recursors
		case dns.RcodeServerFailure: // SERVFAIL
			return 2 // Server failure - prefer NXDOMAIN over this
		default:
			return 2 // Other DNS errors
		}
	}

	if _, ok := err.(net.Error); ok {
		return 3
	}

	return 3
}

func (q *raceRecursorPool) drainResults(results chan raceResult, remaining int) {
	for i := 0; i < remaining; i++ {
		<-results
	}
}
