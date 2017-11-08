package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"strings"
	"time"
)

type Config struct {
	Address         string
	Port            int
	Timeout         DurationJSON
	RecursorTimeout DurationJSON `json:"recursor_timeout"`
	Recursors       []string
	RecordsFile     string   `json:"records_file"`
	AliasFilesGlob  string   `json:"alias_files_glob"`
	UpcheckDomains  []string `json:"upcheck_domains"`

	Health   HealthConfig `json:"health"`
	Cache    Cache        `json:"cache"`
	Handlers []Handler    `json:"handlers"`
}

type Handler struct {
	Domain string `json:"domain"`
	Source Source `json:"source"`
	Cache  Cache  `json:"cache"`
}

type Source struct {
	Type      string   `json:"type"`
	URL       string   `json:"url,omitempty"`
	Recursors []string `json:"recursors,omitempty"`
}

type HealthConfig struct {
	Enabled           bool
	Port              int          `json:"port"`
	CertificateFile   string       `json:"certificate_file"`
	PrivateKeyFile    string       `json:"private_key_file"`
	CAFile            string       `json:"ca_file"`
	CheckInterval     DurationJSON `json:"check_interval"`
	MaxTrackedQueries int          `json:"max_tracked_queries"`
}

type Cache struct {
	Enabled bool `json:"enabled"`
}

type DurationJSON time.Duration

func (t *DurationJSON) UnmarshalJSON(b []byte) error {
	var s string

	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}

	timeoutDuration, err := time.ParseDuration(s)
	if err != nil {
		return err
	}

	*t = DurationJSON(timeoutDuration)

	return nil
}

func (t DurationJSON) MarshalJSON() (b []byte, err error) {
	d := time.Duration(t)
	return []byte(fmt.Sprintf(`"%s"`, d.String())), nil
}

func LoadFromFile(configFilePath string) (Config, error) {
	configFileContents, err := ioutil.ReadFile(configFilePath)
	if err != nil {
		return Config{}, err
	}

	c := Config{
		Timeout:         DurationJSON(5 * time.Second),
		RecursorTimeout: DurationJSON(2 * time.Second),
		Health: HealthConfig{
			MaxTrackedQueries: 2000,
		},
	}

	if err := json.Unmarshal(configFileContents, &c); err != nil {
		return Config{}, err
	}

	if c.Port == 0 {
		return Config{}, errors.New("port is required")
	}

	c.Recursors, err = AppendDefaultDNSPortIfMissing(c.Recursors)
	if err != nil {
		return Config{}, err
	}

	for i := range c.Handlers {
		c.Handlers[i].Source.Recursors, err = AppendDefaultDNSPortIfMissing(c.Handlers[i].Source.Recursors)
		if err != nil {
			return Config{}, err
		}
	}

	return c, nil
}

func AppendDefaultDNSPortIfMissing(recursors []string) ([]string, error) {
	recursorsWithPort := []string{}
	for i := range recursors {
		_, _, err := net.SplitHostPort(recursors[i])
		if err != nil {
			if strings.Contains(err.Error(), "missing port in address") {
				recursorsWithPort = append(recursorsWithPort, net.JoinHostPort(recursors[i], "53"))
			} else {
				return []string{}, err
			}
		} else {
			recursorsWithPort = append(recursorsWithPort, recursors[i])
		}
	}
	return recursorsWithPort, nil
}
