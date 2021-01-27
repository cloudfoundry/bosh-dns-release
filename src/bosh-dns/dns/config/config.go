package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"strings"
	"time"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

const (
	SmartRecursorSelection  = "smart"
	SerialRecursorSelection = "serial"
	RFCFormatting           = "rfc3339"
)

type Config struct {
	Address            string       `json:"address"`
	Port               int          `json:"port"`
	BindTimeout        DurationJSON `json:"timeout,omitempty"`
	RecursorRetryCount int          `json:"recursor_retry_count,omitempty"`
	RequestTimeout     DurationJSON `json:"request_timeout,omitempty"`
	RecursorTimeout    DurationJSON `json:"recursor_timeout,omitempty"`
	Recursors          []string     `json:"recursors,omitempty"`
	ExcludedRecursors  []string     `json:"excluded_recursors,omitempty"`
	RecordsFile        string       `json:"records_file,omitempty"`
	RecursorSelection  string       `json:"recursor_selection"`
	AliasFilesGlob     string       `json:"alias_files_glob,omitempty"`
	HandlersFilesGlob  string       `json:"handlers_files_glob,omitempty"`
	AddressesFilesGlob string       `json:"addresses_files_glob,omitempty"`
	UpcheckDomains     []string     `json:"upcheck_domains,omitempty"`
	JobsDir            string       `json:"jobs_dir,omitempty"`

	LogLevel string `json:"log_level,omitempty"`

	API APIConfig `json:"api"`

	Health                HealthConfig          `json:"health"`
	Metrics               MetricsConfig         `json:"metrics"`
	Cache                 Cache                 `json:"cache"`
	InternalUpcheckDomain InternalUpcheckDomain `json:"internal_upcheck_domain"`
	Logging               LoggingConfig         `json:"logging,omitempty"`
}

func (c Config) GetLogLevel() (boshlog.LogLevel, error) {
	level, err := boshlog.Levelify(c.LogLevel)
	if err != nil {
		return boshlog.LevelNone, err
	}
	return level, nil
}

func (c Config) UseRFC3339Formatting() bool {
	return strings.EqualFold(c.Logging.Format.TimeStamp, RFCFormatting)
}

type APIConfig struct {
	Port            int    `json:"port"`
	CertificateFile string `json:"certificate_file"`
	PrivateKeyFile  string `json:"private_key_file"`
	CAFile          string `json:"ca_file"`
}

type HealthConfig struct {
	Enabled                 bool         `json:"enabled"`
	Port                    int          `json:"port"`
	CertificateFile         string       `json:"certificate_file"`
	PrivateKeyFile          string       `json:"private_key_file"`
	CAFile                  string       `json:"ca_file"`
	CheckInterval           DurationJSON `json:"check_interval,omitempty"`
	MaxTrackedQueries       int          `json:"max_tracked_queries,omitempty"`
	SynchronousCheckTimeout DurationJSON `json:"synchronous_check_timeout,omitempty"`
}

type MetricsConfig struct {
	Enabled bool   `json:"enabled"`
	Address string `json:"address"`
	Port    int    `json:"port"`
}

type Cache struct {
	Enabled bool `json:"enabled"`
}

type InternalUpcheckDomain struct {
	Enabled  bool   `json:"enabled"`
	DNSQuery string `json:"dns_query"`
}

type LoggingConfig struct {
	Format FormatConfig `json"format,omitempty"`
}

type FormatConfig struct {
	TimeStamp string `json:"timestamp,omitempty"`
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

func NewDefaultConfig() Config {
	return Config{
		BindTimeout:       DurationJSON(5 * time.Second),
		RequestTimeout:    DurationJSON(5 * time.Second),
		RecursorTimeout:   DurationJSON(2 * time.Second),
		RecursorSelection: "smart",
		Health: HealthConfig{
			MaxTrackedQueries:       2000,
			CheckInterval:           DurationJSON(20 * time.Second),
			SynchronousCheckTimeout: DurationJSON(time.Second),
		},
		Metrics: MetricsConfig{
			Enabled: false,
			Address: "127.0.0.1",
			Port:    53088,
		},
		LogLevel: boshlog.AsString(boshlog.LevelDebug),
	}
}

func LoadFromFile(configFilePath string) (Config, error) {
	configFileContents, err := ioutil.ReadFile(configFilePath)
	if err != nil {
		return Config{}, err
	}

	c := NewDefaultConfig()
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

	switch c.RecursorSelection {
	case "smart":
	case "serial":
	default:
		return Config{}, errors.New("invalid value for recursor_selection; expected 'serial' or 'smart'")
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
