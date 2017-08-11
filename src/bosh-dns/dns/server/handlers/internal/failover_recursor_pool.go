package internal

import (
	"errors"
	"sync"
)

const (
	FAIL_HISTORY_LENGTH      = 25
	FAIL_TOLERANCE_THRESHOLD = 5
)

type RecursorPool interface {
	PerformStrategically(func(string) error) error
}

type failoverRecursorPool struct {
	preferenceMutex        *sync.Mutex
	preferredRecursorIndex int

	recursors []recursorWithHistory
}

type recursorWithHistory struct {
	name           string
	failBuffer     chan bool
	failCount      int
	failCountMutex *sync.Mutex
}

func NewFailoverRecursorPool(recursors []string) RecursorPool {
	recursorsWithHistory := []recursorWithHistory{}
	preferenceMutex := &sync.Mutex{}

	if recursors == nil {
		recursors = []string{}
	}

	for _, name := range recursors {
		failBuffer := make(chan bool, FAIL_HISTORY_LENGTH)
		for i := 0; i < FAIL_HISTORY_LENGTH; i++ {
			failBuffer <- false
		}

		preferenceMutex.Lock()
		recursorsWithHistory = append(recursorsWithHistory, recursorWithHistory{
			name:           name,
			failBuffer:     failBuffer,
			failCount:      0,
			failCountMutex: &sync.Mutex{},
		})
		preferenceMutex.Unlock()
	}
	return &failoverRecursorPool{
		recursors:              recursorsWithHistory,
		preferredRecursorIndex: 0,
		preferenceMutex:        preferenceMutex,
	}
}

func (q *failoverRecursorPool) PerformStrategically(work func(string) error) error {
	if len(q.recursors) == 0 {
		return errors.New("no recursors configured")
	}

	q.preferenceMutex.Lock()
	offset := q.preferredRecursorIndex
	q.preferenceMutex.Unlock()
	for i := 0; i < len(q.recursors); i++ {
		index := (i + offset) % len(q.recursors)
		err := work(q.recursors[index].name)
		if err == nil {
			go q.registerResult(index, false)
			return nil
		}
		go q.registerResult(index, true)
	}

	return errors.New("no response from recursors")
}

func (q *failoverRecursorPool) shiftPreference() {
	q.preferenceMutex.Lock()
	q.preferredRecursorIndex = (q.preferredRecursorIndex + 1) % len(q.recursors)
	q.preferenceMutex.Unlock()
}

func (q *failoverRecursorPool) registerResult(index int, wasError bool) {
	failingRecursor := &q.recursors[index]

	oldestResult := <-failingRecursor.failBuffer
	failingRecursor.failBuffer <- wasError

	overflowed := false
	change := 0

	if oldestResult {
		change -= 1
	}

	if wasError {
		change += 1
	}

	if change != 0 {
		failingRecursor.failCountMutex.Lock()
		failingRecursor.failCount += change
		overflowed = (failingRecursor.failCount >= FAIL_TOLERANCE_THRESHOLD)
		failingRecursor.failCountMutex.Unlock()
	}

	if overflowed {
		q.shiftPreference()
	}
}
