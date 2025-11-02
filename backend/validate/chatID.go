package validate

import (
	"github.com/go-playground/validator/v10"
	"github.com/nrf24l01/tg_group_statistics/backend/core"
)

func RegisterChatIdAllowedValidator(v *validator.Validate, groupStatsConfig *core.TGGroupStatsConfig) {
    v.RegisterValidation("chatid_allowed", func(fl validator.FieldLevel) bool {
        chatID := fl.Field().Int()
        for _, id := range groupStatsConfig.AllowedChatIDs {
            if chatID == id {
                return true
            }
        }
        return false
    })
}