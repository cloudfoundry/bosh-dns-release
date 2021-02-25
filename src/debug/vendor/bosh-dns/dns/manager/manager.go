package manager

//go:generate counterfeiter . DNSManager

type DNSManager interface {
	SetPrimary() error
	Read() ([]string, error)
}
