package config

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net"
)

type Config struct {
	DNS DNSConfig
}

type DNSConfig struct {
	Address string
	Port    int
}

func LoadFromFile(configFilePath string) (Config, error) {
	configFileContents, err := ioutil.ReadFile(configFilePath)
	if err != nil {
		return Config{}, err
	}

	c := Config{}
	if err := json.Unmarshal(configFileContents, &c); err != nil {
		return Config{}, err
	}

	if net.ParseIP(c.DNS.Address).To4() == nil {
		return Config{}, errors.New("address is not ipv4")
	} else if c.DNS.Port == 0 {
		return Config{}, errors.New("port is required")
	}

	return c, nil
}
