package postgres

import "github.com/nrf24l01/go-web-utils/pg_kit"

type Group struct {
    pg_kit.BaseModel
    TgGroupID  int64     `gorm:"uniqueIndex;not null"`
    Name       *string   `gorm:"type:text"`
}