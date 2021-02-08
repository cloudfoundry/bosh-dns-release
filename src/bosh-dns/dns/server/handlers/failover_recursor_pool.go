package handlers

import (
	"bosh-dns/dns/config"
	"errors"
	"fmt"
	"github.com/miekg/dns"
	"net"
	"sync/atomic"

	"github.com/cloudfoundry/bosh-utils/logger"
)

const (
	FailHistoryLength    = 25
	FailHistoryThreshold = 5
)

var ErrNoRecursorResponse = errors.New("no response from recursors")

//go:generate counterfeiter . RecursorPool

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
// Each recursor will be queried until one succeeds or all recursors were tried

func NewFailoverRecursorPool(recursors []string, recursorSelection string, recursorRetryCount int, logger logger.Logger) RecursorPool {
	recursorSettings := recursorRetrySettings{
		retryCount: recursorRetryCount,
	}
	if recursorSelection == config.SmartRecursorSelection {
		return newSmartFailoverRecursorPool(recursors, recursorSettings, logger)
	}

	return newSerialFailoverRecursorPool(recursors, recursorSettings, logger)
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
	retryCount int
}

type recursorWithHistory struct {
	name       string
	failBuffer chan bool
	failCount  int32
}

func newSerialFailoverRecursorPool(recursors []string, recursorSettings recursorRetrySettings, logger logger.Logger) RecursorPool {
	return &serialFailoverRecursorPool{
		recursors,
		logger,
		"SerialFailoverRecursor",
		recursorSettings,
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
		if err := performWithRetryLogic(work, r, q.recursorRetrySettings.retryCount, q.logTag, q.logger); err == nil {
			return nil
		}
	}
	return ErrNoRecursorResponse
}

func performWithRetryLogic(work func(string) error, recursor string, retryCount int, logTag string, log logger.Logger) (err error) {
	for ret := -1; ret < retryCount; ret++ {
		err = work(recursor)
		if err == nil {
			return err
		}
		if _, ok := err.(net.Error); !ok {
			log.Debug(logTag, fmt.Sprintf("dns request error %v no retry - %v\n", err, recursor))
			return err
		}
		log.Error(logTag, fmt.Sprintf("dns request network error %s retry [%d/%d] for recoursor %s \n", err.(net.Error), ret+1, retryCount, recursor))
	}

	//retry count reached r
	log.Error(logTag, fmt.Sprintf("write error response to client after retry count reached [%d/%d] with rcode=%d - %s \n", retryCount, retryCount, dns.RcodeServerFailure, err.Error()))
	return err
}

func (q *smartFailoverRecursorPool) PerformStrategically(work func(string) error) error {
	offset := atomic.LoadUint64(&q.preferredRecursorIndex)
	uintRecursorCount := uint64(len(q.recursors))

	for i := uint64(0); i < uintRecursorCount; i++ {
		index := int((i + offset) % uintRecursorCount)

		err := performWithRetryLogic(work, q.recursors[index].name, q.recursorRetrySettings.retryCount, q.logTag, q.logger)
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
