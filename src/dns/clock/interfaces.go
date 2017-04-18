package clock

import "time"

//go:generate counterfeiter . Clock
type Clock interface {
	Now() time.Time
}
