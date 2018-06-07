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
	Address            string       `json:"address"`
	Port               int          `json:"port"`
	Timeout            DurationJSON `json:"timeout,omitempty"`
	RecursorTimeout    DurationJSON `json:"recursor_timeout,omitempty"`
	Recursors          []string     `json:"recursors,omitempty"`
	ExcludedRecursors  []string     `json:"excluded_recursors,omitempty"`
	RecordsFile        string       `json:"records_file,omitempty"`
	AliasFilesGlob     string       `json:"alias_files_glob,omitempty"`
	HandlersFilesGlob  string       `json:"handlers_files_glob,omitempty"`
	AddressesFilesGlob string       `json:"addresses_files_glob,omitempty"`
	UpcheckDomains     []string     `json:"upcheck_domains,omitempty"`

	API APIConfig `json:"api"`

	Health            HealthConfig `json:"health"`
	Cache             Cache        `json:"cache"`
	AgentAliasEnabled bool         `json:"agent_alias_enabled,omitempty"`
}

type APIConfig struct {
	Port            int    `json:"port"`
	CertificateFile string `json:"certificate_file"`
	PrivateKeyFile  string `json:"private_key_file"`
	CAFile          string `json:"ca_file"`
}

type HealthConfig struct {
	Enabled           bool         `json:"enabled"`
	Port              int          `json:"port"`
	CertificateFile   string       `json:"certificate_file"`
	PrivateKeyFile    string       `json:"private_key_file"`
	CAFile            string       `json:"ca_file"`
	CheckInterval     DurationJSON `json:"check_interval,omitempty"`
	MaxTrackedQueries int          `json:"max_tracked_queries,omitempty"`
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

	c.ExcludedRecursors, err = AppendDefaultDNSPortIfMissing(c.ExcludedRecursors)
	if err != nil {
		return Config{}, err
	}

	return c, nil
}

func AppendDefaultDNSPortIfMissing(recursors []string) ([]string, error) {
	recursorsWithPort := []string{}
	for _, recursor := range recursors {
		_, _, err := net.SplitHostPort(recursor)
		cleanedUpRecursor := recursor

		if err != nil {
			if strings.Contains(err.Error(), "missing port in address") || strings.Contains(err.Error(), "too many colons in address") {
				ip := net.ParseIP(recursor)
				if ip == nil {
					return []string{}, fmt.Errorf("Invalid IP address %s", recursor)
				}

				cleanedUpRecursor = net.JoinHostPort(ip.String(), "53")
			} else {
				return []string{}, err
			}
		}

		recursorsWithPort = append(recursorsWithPort, cleanedUpRecursor)
	}
	return recursorsWithPort, nil
}
