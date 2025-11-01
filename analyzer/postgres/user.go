package postgres

import "github.com/nrf24l01/go-web-utils/pg_kit"

type User struct {
    pg_kit.BaseModel
    TgUserID  int64     `gorm:"uniqueIndex;not null"`
    Username  *string   `gorm:"type:text"`
    Nick      *string   `gorm:"type:text"`
}