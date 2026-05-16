package config

import (
	"fmt"
	"os"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Bot      BotConfig      `yaml:"bot"`
	Database DatabaseConfig `yaml:"database"`
	Server   ServerConfig   `yaml:"server"`
	Logger   LoggerConfig   `yaml:"logger"`
	Rate     RateConfig     `yaml:"rate"`
}

type BotConfig struct {
	Token string `yaml:"token"`
}

type DatabaseConfig struct {
	Path string `yaml:"path"`
}

type ServerConfig struct {
	Addr        string        `yaml:"addr"`
	MiniAppURL  string        `yaml:"mini_app_url"`
	InitDataTTL time.Duration `yaml:"init_data_ttl"`
}

type LoggerConfig struct {
	Level string `yaml:"level"`
	File  string `yaml:"file"`
}

type RateConfig struct {
	UpdateInterval      time.Duration `yaml:"update_interval"`
	APIKey              string        `yaml:"api_key"`
	BaseCurrency        string        `yaml:"base_currency"`
	SupportedCurrencies []string      `yaml:"supported_currencies"`
}

var (
	instance *Config
	once     sync.Once
)

func Load(path string) (*Config, error) {
	var err error
	once.Do(func() {
		instance = &Config{}

		data, readErr := os.ReadFile(path)
		if readErr != nil {
			err = fmt.Errorf("read config file: %w", readErr)
			return
		}

		if unmarshalErr := yaml.Unmarshal(data, instance); unmarshalErr != nil {
			err = fmt.Errorf("unmarshal config: %w", unmarshalErr)
			return
		}

		if instance.Logger.Level == "" {
			instance.Logger.Level = "info"
		}
		if instance.Rate.BaseCurrency == "" {
			instance.Rate.BaseCurrency = "JPY"
		}
		if instance.Rate.UpdateInterval == 0 {
			instance.Rate.UpdateInterval = 24 * time.Hour
		}
		if len(instance.Rate.SupportedCurrencies) == 0 {
			instance.Rate.SupportedCurrencies = []string{
				"USD", "EUR", "GBP", "CNY",
				"KRW", "HKD", "SGD", "TWD",
				"AUD", "CAD", "CHF",
				"INR", "THB", "MYR", "VND",
			}
		}
		if instance.Server.Addr == "" {
			instance.Server.Addr = ":8080"
		}
		if instance.Server.InitDataTTL == 0 {
			instance.Server.InitDataTTL = 24 * time.Hour
		}

		if instance.Bot.Token == "" || instance.Bot.Token == "YOUR_BOT_TOKEN_HERE" {
			err = fmt.Errorf("bot token is required in config file")
			return
		}
		if instance.Server.MiniAppURL == "" {
			err = fmt.Errorf("server.mini_app_url is required in config file")
			return
		}
	})

	return instance, err
}
