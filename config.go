package main

import (
"io/ioutil"

"gopkg.in/yaml.v2"
)

// GoogleCredential represents a Google API credential
type GoogleCredential struct {
APIKey         string `yaml:"api_key"`
SearchEngineID string `yaml:"search_engine_id"`
}

// Config represents the application configuration
type Config struct {
Google []GoogleCredential `yaml:"google"`
}

// LoadConfig loads configuration from YAML file
func LoadConfig(filename string) (*Config, error) {
data, err := ioutil.ReadFile(filename)
if err != nil {
return nil, err
}

config := &Config{}
if err := yaml.Unmarshal(data, config); err != nil {
return nil, err
}

return config, nil
}
