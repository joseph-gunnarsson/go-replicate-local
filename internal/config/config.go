package config

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v2"
)

type Config struct {
	LBPort   int                `yaml:"lb_port"`
	Services map[string]Service `yaml:"services"`
}

type Service struct {
	Name        string            `yaml:"name"`
	Path        string            `yaml:"path"`
	StartPort   int               `yaml:"start_port"`
	EndPort     int               `yaml:"end_port"`
	Replicas    int               `yaml:"replicas"`
	RoutePrefix string            `yaml:"route_prefix"`
	Env         map[string]string `yaml:"env"`
}

var DefaultConfig = Config{
	LBPort: 8079,
	Services: map[string]Service{
		"auth-service": {
			Name:        "auth-service",
			Path:        "./examples/auth-service/main.go",
			StartPort:   8083,
			EndPort:     8200,
			Replicas:    100,
			RoutePrefix: "/auth",
			Env:         map[string]string{},
		},
		"second-ser": {
			Name:        "second-ser",
			Path:        "./examples/auth-service/main.go",
			StartPort:   8201,
			EndPort:     8400,
			Replicas:    2,
			RoutePrefix: "/ser2",
			Env:         map[string]string{},
		},
	},
}

func LoadConfig(path string) (Config, error) {
	file, err := os.ReadFile(path)
	if err != nil {
		return Config{}, errors.New("Error reading config file:" + err.Error())
	}
	var config Config = Config{LBPort: 8080}
	err = yaml.Unmarshal(file, &config)
	if err != nil {
		return Config{}, errors.New("Error parsing config file" + err.Error())
	}

	if err := config.Validate(); err != nil {
		return Config{}, fmt.Errorf("config validation failed: %w", err)
	}

	return config, nil
}

func (c Config) Validate() error {
	if c.LBPort <= 0 {
		return errors.New("lb_port must be greater than 0")
	}
	for _, svc := range c.Services {
		if svc.StartPort <= 0 {
			return fmt.Errorf("service %s: start_port must be > 0", svc.Name)
		}
		if svc.EndPort <= svc.StartPort {
			return fmt.Errorf("service %s: end_port must be greater than start_port", svc.Name)
		}
		if (svc.EndPort - svc.StartPort + 1) < svc.Replicas {
			return fmt.Errorf("service %s: port range (%d-%d) is too small for %d replicas", svc.Name, svc.StartPort, svc.EndPort, svc.Replicas)
		}
	}
	return nil
}
