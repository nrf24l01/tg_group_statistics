package postgres

import (
	"time"

	"github.com/google/uuid"
	"github.com/nrf24l01/go-web-utils/pg_kit"
)

type Message struct {
    pg_kit.BaseModel
    ChatID      int64     `gorm:"not null;uniqueIndex:idx_chat_message"`
    MessageID   int64     `gorm:"not null;uniqueIndex:idx_chat_message"`
    SendTime    time.Time `gorm:"type:timestamptz;not null"`
    SenderID    uuid.UUID `gorm:"type:uuid;not null"`
    Sender      User      `gorm:"constraint:OnDelete:CASCADE;foreignKey:SenderID;references:ID"`
    MessageType string    `gorm:"type:text;not null"`
    MessageText *string   `gorm:"type:text"`
}