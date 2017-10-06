package internal

type NoRecursorsError struct{}

func (NoRecursorsError) Error() string {
	return "no recursors configured"
}
