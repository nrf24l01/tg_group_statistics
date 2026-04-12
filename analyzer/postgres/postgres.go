package postgres

import (
	"gorm.io/gorm"
)

func LoadAllDaysList(db *gorm.DB) ([]string, error) {
	var days []string
	if err := db.Raw(`
		SELECT day::text
		FROM (
			SELECT time_bucket('1 day', send_time AT TIME ZONE 'UTC')::date AS day
			FROM messages
			GROUP BY 1
		) days
		ORDER BY day
	`).Scan(&days).Error; err != nil {
		return nil, err
	}
	return days, nil
}
