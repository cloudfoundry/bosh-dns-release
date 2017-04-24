package aliases

//go:generate counterfeiter . NamedConfigLoader

type NamedConfigLoader interface {
	Load(string) (Config, error)
}
