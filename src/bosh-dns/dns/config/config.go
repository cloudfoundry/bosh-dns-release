package config

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net"
	"strings"
	"time"
)

type Config struct {
	Address            string
	Port               int
	Timeout            DurationJSON
	RecursorTimeout    DurationJSON `json:"recursor_timeout"`
	Recursors          []string
	RecordsFile        string   `json:"records_file"`
	AliasFilesGlob     string   `json:"alias_files_glob"`
	HealthcheckDomains []string `json:"healthcheck_domains"`

	Health HealthConfig `json:"health"`
}

type HealthConfig struct {
	Enabled         bool
	Port            int          `json:"port"`
	CertificateFile string       `json:"certificate_file"`
	PrivateKeyFile  string       `json:"private_key_file"`
	CAFile          string       `json:"ca_file"`
	CheckInterval   DurationJSON `json:"check_interval"`
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

func LoadFromFile(configFilePath string) (Config, error) {
	configFileContents, err := ioutil.ReadFile(configFilePath)
	if err != nil {
		return Config{}, err
	}

	c := Config{
		Timeout:         DurationJSON(5 * time.Second),
		RecursorTimeout: DurationJSON(2 * time.Second),
	}

	if err := json.Unmarshal(configFileContents, &c); err != nil {
		return Config{}, err
	}

	if c.Port == 0 {
		return Config{}, errors.New("port is required")
	}

	for i := range c.Recursors {
		_, _, err := net.SplitHostPort(c.Recursors[i])
		if err != nil {
			if strings.Contains(err.Error(), "missing port in address") {
				c.Recursors[i] = net.JoinHostPort(c.Recursors[i], "53")
			} else {
				return Config{}, err
			}
		}
	}

	return c, nil
}
