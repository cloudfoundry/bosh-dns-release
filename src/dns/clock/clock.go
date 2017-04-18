package clock

import "time"

type realClock struct{}

var Real Clock = realClock{}

func (c realClock) Now() time.Time {
	return time.Now()
}
