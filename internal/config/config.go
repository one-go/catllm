// Package config handles loading and parsing the gateway configuration file.
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the gateway configuration
type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Providers []ProviderConfig `yaml:"providers"`
	Routes    []RouteConfig   `yaml:"routes"`
}

// ServerConfig represents HTTP server settings
type ServerConfig struct {
	Port int `yaml:"port"`
}

// ProviderConfig represents an upstream LLM provider
type ProviderConfig struct {
	Name    string `yaml:"name"`
	BaseURL string `yaml:"base_url"`
	APIKey  string `yaml:"api_key"`
	Timeout int    `yaml:"timeout"` // seconds
}

// RouteConfig represents a routing rule
type RouteConfig struct {
	Model    string `yaml:"model"`
	Provider string `yaml:"provider"`
}

// Load reads and parses the config file
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	return &cfg, nil
}

func (c *Config) validate() error {
	if c.Server.Port == 0 {
		c.Server.Port = 8080
	}
	if len(c.Providers) == 0 {
		return fmt.Errorf("no providers configured")
	}
	if len(c.Routes) == 0 {
		return fmt.Errorf("no routes configured")
	}
	return nil
}

// GetProvider returns the provider config by name
func (c *Config) GetProvider(name string) *ProviderConfig {
	for i := range c.Providers {
		if c.Providers[i].Name == name {
			return &c.Providers[i]
		}
	}
	return nil
}

// GetRoute returns the provider name for a given model
func (c *Config) GetRoute(model string) string {
	for _, r := range c.Routes {
		if r.Model == model {
			return r.Provider
		}
	}
	return ""
}
