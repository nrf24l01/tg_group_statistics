package core

import (
	"log"
	"strconv"
	"strings"

	"github.com/caarlos0/env/v11"
)

type TGGroupStatsConfig struct {
	AccessTokenSecret  string  `env:"ACCESS_TOKEN_SECRET" envDefault:"default_secret"`
	AllowedChatIDs     []int64 `env:"CHAT_ID" envDefault:"0"`
}

func LoadTGGroupStatsConfigFromEnv() *TGGroupStatsConfig {
	config := &TGGroupStatsConfig{}
	if err := env.Parse(config); err != nil {
		log.Fatalf("Failed to parse environment variables: %v", err)
	}
	for i, id := range config.AllowedChatIDs {
		idStr := strconv.FormatInt(id, 10)
		if !strings.HasPrefix(idStr, "-100") {
			newIdStr := "-100" + idStr
			newId, err := strconv.ParseInt(newIdStr, 10, 64)
			if err != nil {
				log.Fatalf("Failed to parse prefixed chat id %q: %v", newIdStr, err)
			}
			config.AllowedChatIDs[i] = newId
		}
	}
	return config
}