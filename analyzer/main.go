package main

import (
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/nrf24l01/go-web-utils/pg_kit"
	"github.com/nrf24l01/tg_group_statistics/analyzer/core"
	"github.com/nrf24l01/tg_group_statistics/analyzer/postgres"
)

func main() {
	// Try to load .env file in non-production environment
	if os.Getenv("PRODUCTION_ENV") != "true" {
		err := godotenv.Load(".env")
		if err != nil {
			log.Fatalf("failed to load .env: %v", err)
		}
	}
	
	// Configuration initialization
	config := core.BuildConfigFromEnv()

	// Data sources initialization
	db, err := pg_kit.RegisterPostgres(config.PGConfig, &postgres.Group{}, &postgres.User{}, &postgres.Message{}, &postgres.UserStats{}, &postgres.GroupStats{})
	if err != nil {
		log.Fatalf("failed to connect to postgres: %v", err)
	}
	db.Exec(`
		CREATE EXTENSION IF NOT EXISTS timescaledb;
        SELECT create_hypertable('users_stats', 'date', if_not_exists => TRUE);
		SELECT create_hypertable('groups_stats', 'date', if_not_exists => TRUE);
    `)

	log.Print("Initialized successfully")
	update(db, config)
}