package update

import (
	"time"

	"github.com/nrf24l01/tg_group_statistics/analyzer/postgres"
)

func removeTime(date time.Time) time.Time {
	year, month, day := date.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func (h *Handler) getAllMessagesPerGroup(group_id int64, usedForStats bool) ([]postgres.Message, error) {
	var messages []postgres.Message

	if err := h.DB.Where("group_id = ? AND used_for_stats = ?", group_id, usedForStats).Find(&messages).Error; err != nil {
		return nil, err
	}
	for i := range messages {
		messages[i].SendTime = removeTime(messages[i].SendTime)
	}
	return messages, nil
}

func (h *Handler) getAllDatesPerGroup(group_id int64) ([]Date, error) {
	var dates []Date

	rows, err := h.DB.Model(&postgres.Message{}).
		Select("DISTINCT DATE(send_time) AS date").
		Where("group_id = ?", group_id).
		Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var date time.Time
		if err := rows.Scan(&date); err != nil {
			return nil, err
		}
		dates = append(dates, Date{Date: removeTime(date)})
	}
	return dates, nil
}

func (h *Handler) getAllGroupIds() ([]int64, error) {
	var groups []postgres.Group

	if err := h.DB.Find(&groups).Error; err != nil {
		return nil, err
	}
	var group_ids []int64
	for _, group := range groups {
		group_ids = append(group_ids, group.TgGroupID)
	}
	return group_ids, nil
}

func (h *Handler) getAllUsersPerGroup(group_id int64) ([]postgres.User, error) {
	var users []postgres.User

	if err := h.DB.Joins("JOIN messages ON messages.sender_id = users.id").
		Where("messages.group_id = ?", group_id).
		Group("users.id").
		Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}