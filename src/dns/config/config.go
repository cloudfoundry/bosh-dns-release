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
	Address         string
	Port            int
	Timeout         Timeout
	RecursorTimeout Timeout `json:"recursor_timeout"`
	Recursors       []string
	RecordsFile     string `json:"records_file"`
	AliasFilesGlob  string `json:"alias_files_glob"`
}

type Timeout time.Duration

func (t *Timeout) UnmarshalJSON(b []byte) error {
	var s string

	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}

	timeoutDuration, err := time.ParseDuration(s)
	if err != nil {
		return err
	}

	*t = Timeout(timeoutDuration)

	return nil
}

func LoadFromFile(configFilePath string) (Config, error) {
	configFileContents, err := ioutil.ReadFile(configFilePath)
	if err != nil {
		return Config{}, err
	}

	c := Config{
		Timeout:         Timeout(5 * time.Second),
		RecursorTimeout: Timeout(2 * time.Second),
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
