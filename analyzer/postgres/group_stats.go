package postgres

import (
	"time"

	"github.com/google/uuid"
	"github.com/nrf24l01/go-web-utils/pg_kit"
)

type GroupStats struct {
	pg_kit.BaseModel
	GroupID   uuid.UUID `gorm:"type:uuid;not null;primaryKey"`
	Group     Group     `gorm:"constraint:OnDelete:CASCADE;foreignKey:GroupID;references:ID"`
	Date      time.Time `gorm:"not null;index;primaryKey"`
	MsgCount  int64     `gorm:"not null;default:0"`
}

func (GroupStats) TableName() string {
	return "groups_stats"
}