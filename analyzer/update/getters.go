package update

import (
	"time"

	"github.com/google/uuid"
	"github.com/nrf24l01/tg_group_statistics/analyzer/postgres"
	"gorm.io/datatypes"
)

type messageRow struct {
	ID          string
	SendTime    time.Time
	TgUserID    int64
	MessageText *string
}

type userStatsRow struct {
	TgUserID   int64
	Date       time.Time
	MsgCount   int64
	WordCounts datatypes.JSONMap
}

type groupStatsRow struct {
	Date       time.Time
	MsgCount   int64
	WordCounts datatypes.JSONMap
}

func removeTime(date time.Time) time.Time {
	date = date.UTC()
	year, month, day := date.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func (h *Handler) getAllMessagesPerGroup(group_id int64, usedForStats bool) ([]postgres.Message, error) {
	var rows []messageRow
	if err := h.DB.Raw(`
		SELECT
			m.id::text AS id,
			m.send_time,
			u.tg_user_id,
			m.message_text
		FROM messages m
		JOIN users u ON u.id = m.sender_id
		WHERE m.chat_id = ?
		  AND m.used_for_stats = ?
		ORDER BY m.send_time, m.id
	`, group_id, usedForStats).Scan(&rows).Error; err != nil {
		return nil, err
	}

	messages := make([]postgres.Message, 0, len(rows))
	for _, row := range rows {
		id, err := uuid.Parse(row.ID)
		if err != nil {
			return nil, err
		}
		messages = append(messages, postgres.Message{
			BaseModel:   postgres.Message{}.BaseModel,
			SendTime:    removeTime(row.SendTime),
			MessageText: row.MessageText,
			Sender: postgres.User{
				TgUserID: row.TgUserID,
			},
		})
		messages[len(messages)-1].ID = id
	}
	return messages, nil
}

func (h *Handler) getAllDatesPerGroup(group_id int64) ([]Date, error) {
	var rawDates []time.Time
	if err := h.DB.Raw(`
		SELECT time_bucket('1 day', send_time AT TIME ZONE 'UTC')::date AS date
		FROM messages
		WHERE chat_id = ?
		GROUP BY 1
		ORDER BY 1
	`, group_id).Scan(&rawDates).Error; err != nil {
		return nil, err
	}

	dates := make([]Date, 0, len(rawDates))
	for _, date := range rawDates {
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

	if err := h.DB.Raw(`
		SELECT u.*
		FROM users u
		JOIN (
			SELECT DISTINCT sender_id
			FROM messages
			WHERE chat_id = ?
		) m ON m.sender_id = u.id
		ORDER BY u.tg_user_id
	`, group_id).Scan(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

func (h *Handler) loadUsersStats(group_id int64) (map[int64]UserStats, error) {
	var statsDB []userStatsRow
	result := make(map[int64]UserStats)

	if err := h.DB.Raw(`
		SELECT
			u.tg_user_id,
			us.date,
			us.msg_count,
			us.word_counts
		FROM users_stats us
		JOIN groups g ON g.id = us.group_id
		JOIN users u ON u.id = us.sender_id
		WHERE g.tg_group_id = ?
	`, group_id).Scan(&statsDB).Error; err != nil {
		return nil, err
	}

	for _, stat := range statsDB {
		uid := stat.TgUserID
		dateKey := removeTime(stat.Date).Format("02-01-2006")

		us := result[uid]
		if us.MessagesPerDay == nil {
			us.MessagesPerDay = make(map[string]int)
		}
		if us.WordCountsPerDay == nil {
			us.WordCountsPerDay = make(map[string]map[string]int64)
		}
		us.MessagesPerDay[dateKey] += int(stat.MsgCount)
		us.TotalMessages += int(stat.MsgCount)

		if len(stat.WordCounts) > 0 {
			day := ensureWordDayMap(us.WordCountsPerDay, dateKey)
			existing := jsonMapToInt64Counts(stat.WordCounts)
			for w, c := range existing {
				day[w] += c
			}
		}

		result[uid] = us
	}
	return result, nil
}

func (h *Handler) loadGroupStats(group_id int64) (GroupStats, error) {
	var statsDB []groupStatsRow
	result := GroupStats{
		MessagesPerDay:   make(map[string]int),
		WordCountsPerDay: make(map[string]map[string]int64),
	}

	if err := h.DB.Raw(`
		SELECT
			gs.date,
			gs.msg_count,
			gs.word_counts
		FROM groups_stats gs
		JOIN groups g ON g.id = gs.group_id
		WHERE g.tg_group_id = ?
	`, group_id).Scan(&statsDB).Error; err != nil {
		return GroupStats{}, err
	}

	for _, stat := range statsDB {
		dateKey := removeTime(stat.Date).Format("02-01-2006")
		result.MessagesPerDay[dateKey] += int(stat.MsgCount)
		result.TotalMessages += int(stat.MsgCount)
		if len(stat.WordCounts) > 0 {
			day := ensureWordDayMap(result.WordCountsPerDay, dateKey)
			existing := jsonMapToInt64Counts(stat.WordCounts)
			for w, c := range existing {
				day[w] += c
			}
		}
	}
	return result, nil
}
