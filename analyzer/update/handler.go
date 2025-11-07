package update

import (
	"github.com/nrf24l01/tg_group_statistics/analyzer/core"
	"gorm.io/gorm"
)

type Handler struct {
	DB *gorm.DB
	Cfg *core.Config
}