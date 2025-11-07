package update

import (
	"time"

	"github.com/nrf24l01/tg_group_statistics/analyzer/postgres"
)

func removeTime(date time.Time) time.Time {
	date = date.UTC()
	year, month, day := date.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func (h *Handler) getAllMessagesPerGroup(group_id int64, usedForStats bool) ([]postgres.Message, error) {
	var messages []postgres.Message

	// Use Preload to eagerly load the Sender relation instead of Joins("users")
	// Joins("users") produced malformed SQL in some DB setups (created a table alias on FROM).
	// messages are linked to telegram groups via ChatID (int64). Filter by chat_id.
	if err := h.DB.Preload("Sender").Where("chat_id = ? AND used_for_stats = ?", group_id, usedForStats).Find(&messages).Error; err != nil {
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
		Where("chat_id = ?", group_id).
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
		Where("messages.chat_id = ?", group_id).
		Group("users.id").
		Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

func (h *Handler) loadUsersStats(group_id int64) (map[int64]UserStats, error) {
	var group postgres.Group
	if err := h.DB.Where("tg_group_id = ?", group_id).First(&group).Error; err != nil {
		return nil, err
	}

	var statsDB []postgres.UserStats
	result := make(map[int64]UserStats)

	if err := h.DB.Preload("Sender").Where("group_id = ?", group.ID).Find(&statsDB).Error; err != nil {
		return nil, err
	}

	for _, stat := range statsDB {
		uid := stat.Sender.TgUserID
		dateKey := removeTime(stat.Date).Format("02-01-2006")

		us := result[uid]
		if us.MessagesPerDay == nil {
			us.MessagesPerDay = make(map[string]int)
		}
		us.MessagesPerDay[dateKey] += int(stat.MsgCount)
		us.TotalMessages += int(stat.MsgCount)

		result[uid] = us
	}
	return result, nil
}

func (h *Handler) loadGroupStats(group_id int64) (GroupStats, error) {
	var group postgres.Group
	if err := h.DB.Where("tg_group_id = ?", group_id).First(&group).Error; err != nil {
		return GroupStats{}, err
	}

	var statsDB []postgres.GroupStats
	result := GroupStats{
		MessagesPerDay: make(map[string]int),
	}

	if err := h.DB.Where("group_id = ?", group.ID).Find(&statsDB).Error; err != nil {
		return GroupStats{}, err
	}

	for _, stat := range statsDB {
		dateKey := removeTime(stat.Date).Format("02-01-2006")
		result.MessagesPerDay[dateKey] += int(stat.MsgCount)
		result.TotalMessages += int(stat.MsgCount)
	}
	return result, nil
}