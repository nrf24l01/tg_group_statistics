package postgres

import (
	"time"

	"github.com/google/uuid"
	"github.com/nrf24l01/go-web-utils/pg_kit"
	"gorm.io/datatypes"
)

type UserStats struct {
	pg_kit.BaseModel
	SenderID     uuid.UUID          `gorm:"type:uuid;not null;primaryKey;uniqueIndex:idx_sender_group_date"`
	Sender       User               `gorm:"constraint:OnDelete:CASCADE;foreignKey:SenderID;references:ID"`
	GroupID      uuid.UUID          `gorm:"type:uuid;not null;primaryKey;uniqueIndex:idx_sender_group_date"`
	Group        Group              `gorm:"constraint:OnDelete:CASCADE;foreignKey:GroupID;references:ID"`
	Date         time.Time          `gorm:"not null;primaryKey;uniqueIndex:idx_sender_group_date"`
	MsgCount     int64              `gorm:"not null;default:0"`
    WordCounts   datatypes.JSONMap  `gorm:"type:jsonb;not null;default:'{}'"`
}

func (UserStats) TableName() string {
	return "users_stats"
}