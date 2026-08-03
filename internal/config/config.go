package config

import (
	"fmt"

	"github.com/spf13/viper"
)

const Version = 1

var config *Config

type Config struct {
	Version        int                `yaml:"version"`
	DefaultProfile string             `yaml:"default_profile"`
	Palette        map[string]string  `yaml:"palette"`
	Profiles       map[string]Profile `yaml:"profiles"`
}

type Profile struct {
	Extends []string `yaml:"extends"`
	Rules   []Rule   `yaml:"rules"`
}

type Rule struct {
	Description string            `yaml:"description"`
	Regex       string            `yaml:"regex"`
	Style       string            `yaml:"style"`
	Groups      map[string]string `yaml:"groups"`
	Exclusive   bool              `yaml:"exclusive"`
}

func LoadConfig() error {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")

	viper.SetDefault("version", Version)
	viper.SetDefault("default_profile", "default")
	viper.SetDefault("palette", map[string]string{
		"success":   "#5fd75f",
		"warning":   "#ffd75f",
		"danger":    "#ff5f5f",
		"info":      "#5fafff",
		"address":   "#d787ff",
		"interface": "#5fffff",
		"number":    "#d7d787",
	})

	viper.SetDefault("profiles", map[string]Profile{
		"default": {
			Extends: []string{},
			Rules:   []Rule{},
		},
	})

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return fmt.Errorf("failed to read config: %w", err)
		}
	}
	if err := viper.SafeWriteConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileAlreadyExistsError); !ok {
			panic(err)
		}
	}
	if err := viper.Unmarshal(&config); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if config.Version != Version {
		return fmt.Errorf("config version mismatch: expected %d, got %d", Version, config.Version)
	}

	return nil
}
