package postgres

import (
	"time"

	"github.com/google/uuid"
	"github.com/nrf24l01/go-web-utils/pg_kit"
)

type UserStats struct {
	pg_kit.BaseModel
	SenderID  uuid.UUID `gorm:"type:uuid;not null;primaryKey"`
	Sender    User      `gorm:"constraint:OnDelete:CASCADE;foreignKey:SenderID;references:ID"`
	GroupID   uuid.UUID `gorm:"type:uuid;not null;primaryKey"`
	Group     Group     `gorm:"constraint:OnDelete:CASCADE;foreignKey:GroupID;references:ID"`
	Date      time.Time `gorm:"not null;index;primaryKey"`
	MsgCount  int64     `gorm:"not null;default:0"`
}

func (UserStats) TableName() string {
	return "users_stats"
}