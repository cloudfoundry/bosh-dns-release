package handler

//go:generate counterfeiter . Handler
type Handler interface {
	Apply() error
	IsCorrect() (bool, error)
}
