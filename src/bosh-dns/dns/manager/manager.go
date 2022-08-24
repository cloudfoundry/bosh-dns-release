package manager

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

//counterfeiter:generate . DNSManager

type DNSManager interface {
	SetPrimary() error
	Read() ([]string, error)
}
