package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v2"
)

// Config represents the application configuration
type Config struct {
	Server     ServerConfig  `yaml:"server"`
	Monitoring MonitorConfig `yaml:"monitoring"`
	Targets    []Target      `yaml:"targets"`
}

// ServerConfig contains HTTP server configuration
type ServerConfig struct {
	Port int `yaml:"port"`
}

// MonitorConfig contains monitoring configuration
type MonitorConfig struct {
	Interval   time.Duration `yaml:"interval"`
	Timeout    time.Duration `yaml:"timeout"`
	HopTimeout time.Duration `yaml:"hop_timeout"`
}

// Target represents a traceroute target
type Target struct {
	Host      string `yaml:"host"`
	Name      string `yaml:"name"`
	MaxHops   int    `yaml:"max_hops"`
	StartPort int    `yaml:"start_port"`
}

// LoadConfig loads configuration from YAML file
func LoadConfig(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Set default values if not specified
	if config.Server.Port == 0 {
		config.Server.Port = 9655
	}
	if config.Monitoring.Interval == 0 {
		config.Monitoring.Interval = 60 * time.Second
	}
	if config.Monitoring.Timeout == 0 {
		config.Monitoring.Timeout = 30 * time.Second
	}
	if config.Monitoring.HopTimeout == 0 {
		config.Monitoring.HopTimeout = 5 * time.Second
	}

	// Set default values for targets
	for i := range config.Targets {
		if config.Targets[i].MaxHops == 0 {
			config.Targets[i].MaxHops = 30
		}
		if config.Targets[i].StartPort == 0 {
			config.Targets[i].StartPort = 33434
		}
		if config.Targets[i].Name == "" {
			config.Targets[i].Name = config.Targets[i].Host
		}
	}

	return &config, nil
}

// GetListenAddress returns the server listen address
func (c *Config) GetListenAddress() string {
	return fmt.Sprintf(":%d", c.Server.Port)
}
