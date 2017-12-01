package handlers

type Config struct {
	Handlers []Handler `json:"handlers"`
}

type Handler struct {
	Domain string      `json:"domain"`
	Source Source      `json:"source"`
	Cache  ConfigCache `json:"cache"`
}

type Source struct {
	Type      string   `json:"type"`
	URL       string   `json:"url,omitempty"`
	Recursors []string `json:"recursors,omitempty"`
}

type ConfigCache struct {
	Enabled bool `json:"enabled"`
}
