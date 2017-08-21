package internal

import (
	"errors"
	"sync/atomic"
)

const (
	FAIL_HISTORY_LENGTH      = 25
	FAIL_TOLERANCE_THRESHOLD = 5
)

type RecursorPool interface {
	PerformStrategically(func(string) error) error
}

type failoverRecursorPool struct {
	preferredRecursorIndex uint64

	recursors []recursorWithHistory
}

type recursorWithHistory struct {
	name       string
	failBuffer chan bool
	failCount  int32
}

func NewFailoverRecursorPool(recursors []string) RecursorPool {
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
	return &failoverRecursorPool{
		recursors:              recursorsWithHistory,
		preferredRecursorIndex: 0,
	}
}

func (q *failoverRecursorPool) PerformStrategically(work func(string) error) error {
	if len(q.recursors) == 0 {
		return errors.New("no recursors configured")
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
