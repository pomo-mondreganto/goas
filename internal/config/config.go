package config

import (
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type Config struct {
	LogLevel             string `mapstructure:"log_level"`
	Debug                bool   `mapstructure:"debug"`
	Token                string `mapstructure:"token"`
	Data                 string `mapstructure:"data"`
	Samples              string `mapstructure:"samples"`
	Dictionary           string `mapstructure:"dictionary"`
	InterestingThreshold int    `mapstructure:"interesting_threshold"`
	SuspiciousThreshold  int    `mapstructure:"suspicious_threshold"`
}

func Get() *Config {
	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		logrus.Fatalf("Error unmarshalling config: %v", err)
	}
	logrus.Debugf("Got config: %+v", cfg)
	return &cfg
}
