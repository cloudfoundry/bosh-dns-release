package handlers

import (
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/cloudfoundry/bosh-utils/logger"
)

const (
	FAIL_HISTORY_LENGTH      = 25
	FAIL_TOLERANCE_THRESHOLD = 5
)

//go:generate counterfeiter . RecursorPool

type RecursorPool interface {
	PerformStrategically(func(string) error) error
}

type failoverRecursorPool struct {
	preferredRecursorIndex uint64

	logger    logger.Logger
	logTag    string
	recursors []recursorWithHistory
}

type recursorWithHistory struct {
	name       string
	failBuffer chan bool
	failCount  int32
}

func NewFailoverRecursorPool(recursors []string, logger logger.Logger) RecursorPool {
	recursorsWithHistory := []recursorWithHistory{}

	if recursors == nil {
		recursors = []string{}
	}

	for _, name := range recursors {
		failBuffer := make(chan bool, FAIL_HISTORY_LENGTH)
		for i := 0; i < FAIL_HISTORY_LENGTH; i++ {
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
	return &failoverRecursorPool{
		recursors:              recursorsWithHistory,
		preferredRecursorIndex: 0,
		logger:                 logger,
		logTag:                 logTag,
	}
}

func (q *failoverRecursorPool) PerformStrategically(work func(string) error) error {
	if len(q.recursors) == 0 {
		return NoRecursorsError{}
	}

	offset := atomic.LoadUint64(&q.preferredRecursorIndex)
	uintRecursorCount := uint64(len(q.recursors))

	for i := uint64(0); i < uintRecursorCount; i++ {
		index := int((i + offset) % uintRecursorCount)
		err := work(q.recursors[index].name)
		if err == nil {
			q.registerResult(index, false)
			return nil
		}

		failures := q.registerResult(index, true)
		if i == 0 && failures >= FAIL_TOLERANCE_THRESHOLD {
			q.shiftPreference()
		}
	}

	return errors.New("no response from recursors")
}

func (q *failoverRecursorPool) shiftPreference() {
	atomic.AddUint64(&q.preferredRecursorIndex, 1)
	index := q.preferredRecursorIndex % uint64(len(q.recursors))
	q.logger.Info(q.logTag, fmt.Sprintf("shifting recursor preference: %s\n", q.recursors[index].name))
}

func (q *failoverRecursorPool) registerResult(index int, wasError bool) int32 {
	failingRecursor := &q.recursors[index]

	oldestResult := <-failingRecursor.failBuffer
	failingRecursor.failBuffer <- wasError

	change := int32(0)

	if oldestResult {
		change -= 1
	}

	if wasError {
		change += 1
	}

	return atomic.AddInt32(&failingRecursor.failCount, change)
}
