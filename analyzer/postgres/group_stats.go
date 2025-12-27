package postgres

import (
	"time"

	"github.com/google/uuid"
	"github.com/nrf24l01/go-web-utils/pg_kit"
	"gorm.io/datatypes"
)

type GroupStats struct {
	pg_kit.BaseModel
	GroupID     uuid.UUID          `gorm:"type:uuid;not null;primaryKey;uniqueIndex:idx_group_date"`
	Group       Group              `gorm:"constraint:OnDelete:CASCADE;foreignKey:GroupID;references:ID"`
	Date        time.Time          `gorm:"not null;primaryKey;uniqueIndex:idx_group_date"`
	MsgCount    int64              `gorm:"not null;default:0"`
    WordCounts  datatypes.JSONMap  `gorm:"type:jsonb;not null;default:'{}'"`
}

func (GroupStats) TableName() string {
	return "groups_stats"
}