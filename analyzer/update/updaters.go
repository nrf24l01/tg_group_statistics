package update

import (
	"time"

	"github.com/nrf24l01/tg_group_statistics/analyzer/postgres"
	"gorm.io/gorm/clause"
)

func (h *Handler) applyUsersStats(group_id int64, userStats map[int64]UserStats) error {
	if len(userStats) == 0 {
		return nil
	}

	var group postgres.Group
	if err := h.DB.Where("tg_group_id = ?", group_id).First(&group).Error; err != nil {
		return err
	}

	tgIDs := make([]int64, 0, len(userStats))
	for uid := range userStats {
		tgIDs = append(tgIDs, uid)
	}

	var users []postgres.User
	if err := h.DB.Where("tg_user_id IN ?", tgIDs).Find(&users).Error; err != nil {
		return err
	}
	userMap := make(map[int64]postgres.User)
	for _, u := range users {
		userMap[u.TgUserID] = u
	}

	var toCreate []postgres.User
	for _, tg := range tgIDs {
		if _, ok := userMap[tg]; !ok {
			toCreate = append(toCreate, postgres.User{TgUserID: tg})
		}
	}
	if len(toCreate) > 0 {
		if err := h.DB.Create(&toCreate).Error; err != nil {
			return err
		}
		for _, u := range toCreate {
			userMap[u.TgUserID] = u
		}
	}

	var rows []postgres.UserStats
	for tg, stats := range userStats {
		u, ok := userMap[tg]
		if !ok {
			continue
		}
		for dateKey, cnt := range stats.MessagesPerDay {
			d, err := time.Parse("02-01-2006", dateKey)
			if err != nil {
				d, err = time.Parse("2006-01-02", dateKey)
				if err != nil {
					continue
				}
			}
			d = time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC)

			rows = append(rows, postgres.UserStats{
				SenderID: u.ID,
				GroupID:  group.ID,
				Date:     d,
				MsgCount: int64(cnt),
			})
		}
	}

	if len(rows) == 0 {
		return nil
	}

 	const chunkSize = 500
	for i := 0; i < len(rows); i += chunkSize {
		end := i + chunkSize
		if end > len(rows) {
			end = len(rows)
		}
		chunk := rows[i:end]
		if err := h.DB.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "sender_id"}, {Name: "group_id"}, {Name: "date"}},
			DoUpdates: clause.AssignmentColumns([]string{"msg_count"}),
		}).Create(&chunk).Error; err != nil {
			return err
		}
	}

	return nil
}

func (h *Handler) applyGroupStats(group_id int64, groupStats GroupStats) error {
	var group postgres.Group
	if err := h.DB.Where("tg_group_id = ?", group_id).First(&group).Error; err != nil {
		return err
	}

	var rows []postgres.GroupStats
	for dateKey, cnt := range groupStats.MessagesPerDay {
		d, err := time.Parse("02-01-2006", dateKey)
		if err != nil {
			d, err = time.Parse("2006-01-02", dateKey)
			if err != nil {
				continue
			}
		}
		d = time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC)

		rows = append(rows, postgres.GroupStats{
			GroupID:  group.ID,
			Date:     d,
			MsgCount: int64(cnt),
		})
	}

	if len(rows) == 0 {
		return nil
	}

	const chunkSize = 500
	for i := 0; i < len(rows); i += chunkSize {
		end := i + chunkSize
		if end > len(rows) {
			end = len(rows)
		}
		chunk := rows[i:end]
		if err := h.DB.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "group_id"}, {Name: "date"}},
			DoUpdates: clause.AssignmentColumns([]string{"msg_count"}),
		}).Create(&chunk).Error; err != nil {
			return err
		}
	}

	return nil
}