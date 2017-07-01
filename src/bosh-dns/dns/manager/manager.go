package manager

//go:generate counterfeiter . DNSManager

type DNSManager interface {
	SetPrimary(string) error
	Read() ([]string, error)
}
