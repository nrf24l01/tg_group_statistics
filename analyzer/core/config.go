package core

import (
	"github.com/nrf24l01/go-web-utils/config"
)

type Config struct {
	PGConfig *config.PGConfig
}

func BuildConfigFromEnv() (*Config) {
	cnf := Config{
		PGConfig: config.LoadPGConfigFromEnv(),
	}
	return &cnf
}