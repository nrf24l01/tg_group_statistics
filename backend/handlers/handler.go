package handlers

import (
	"gorm.io/gorm"

	"github.com/nrf24l01/tg_group_statistics/backend/core"
)

type Handler struct {
	DB *gorm.DB
	Config *core.Config
}