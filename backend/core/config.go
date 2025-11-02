package core

import (
	utilsConfig "github.com/nrf24l01/go-web-utils/config"
)

type Config struct {
	WebAppConfig          *utilsConfig.WebAppConfig
	PGConfig              *utilsConfig.PGConfig
	TGGroupStatsConfig	  *TGGroupStatsConfig
}

func BuildConfigFromEnv() (*Config, error) {
	config := &Config{
		WebAppConfig:         utilsConfig.LoadWebAppConfigFromEnv(),
		PGConfig:             utilsConfig.LoadPGConfigFromEnv(),
		TGGroupStatsConfig:   LoadTGGroupStatsConfigFromEnv(),
	}

	return config, nil
}