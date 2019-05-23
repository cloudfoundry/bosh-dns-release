package config

import (
	"io/ioutil"

	yaml "gopkg.in/yaml.v2"
)

type Config struct {
	Port                 int    `yaml:"port"`
	ConfigurableResponse string `yaml:"configurable_response"`
}

func NewConfig() *Config {
	return &Config{}
}

func (c *Config) LoadFromFile(filename string) error {
	yamlFile, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}

	err = yaml.Unmarshal(yamlFile, c)
	if err != nil {
		return err
	}

	return nil
}
