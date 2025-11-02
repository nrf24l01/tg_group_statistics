package core

import (
	"log"

	"github.com/caarlos0/env/v11"
)

type TGGroupStatsConfig struct {
	AccessTokenSecret  string  `env:"ACCESS_TOKEN_SECRET" envDefault:"default_secret"`
	ChatID 		       int64   `env:"CHAT_ID" envDefault:"0"`
}

func LoadTGGroupStatsConfigFromEnv() *TGGroupStatsConfig {
	config := &TGGroupStatsConfig{}
	if err := env.Parse(config); err != nil {
		log.Fatalf("Failed to parse environment variables: %v", err)
	}
	return config
}