package postgres

import (
	"gorm.io/gorm"
)

func LoadAllDaysList(db *gorm.DB) ([]string, error) {
	var days []string
	if err := db.Raw("SELECT DISTINCT (send_time AT TIME ZONE 'UTC')::date AS day FROM messages ORDER BY day").Scan(&days).Error; err != nil {
		return nil, err
	}
	return days, nil
}