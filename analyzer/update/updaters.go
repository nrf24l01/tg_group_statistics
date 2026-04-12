package update

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nrf24l01/tg_group_statistics/analyzer/postgres"
	"gorm.io/datatypes"
	"gorm.io/gorm"
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

			var wc datatypes.JSONMap
			if stats.WordCountsPerDay != nil {
				if day, ok := stats.WordCountsPerDay[dateKey]; ok {
					wc = int64CountsToJSONMap(day)
				}
			}
			if wc == nil {
				wc = datatypes.JSONMap{}
			}

			rows = append(rows, postgres.UserStats{
				SenderID:   u.ID,
				GroupID:    group.ID,
				Date:       d,
				MsgCount:   int64(cnt),
				WordCounts: wc,
			})
		}
	}

	if len(rows) == 0 {
		return nil
	}

	const chunkSize = 500
	var changedRows int64
	for i := 0; i < len(rows); i += chunkSize {
		end := i + chunkSize
		if end > len(rows) {
			end = len(rows)
		}
		chunk := rows[i:end]
		res := h.DB.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "sender_id"}, {Name: "group_id"}, {Name: "date"}},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"msg_count":   gorm.Expr("EXCLUDED.msg_count"),
				"word_counts": gorm.Expr("CASE WHEN users_stats.word_counts IS DISTINCT FROM EXCLUDED.word_counts THEN EXCLUDED.word_counts ELSE users_stats.word_counts END"),
			}),
			Where: clause.Where{Exprs: []clause.Expression{
				clause.Expr{SQL: "users_stats.msg_count IS DISTINCT FROM EXCLUDED.msg_count OR users_stats.word_counts IS DISTINCT FROM EXCLUDED.word_counts"},
			}},
		}).Create(&chunk)
		if res.Error != nil {
			return res.Error
		}
		changedRows += res.RowsAffected
	}

	log.Printf("users_stats upsert metrics: group_id=%d rows_total=%d rows_changed=%d rows_unchanged=%d", group_id, len(rows), changedRows, int64(len(rows))-changedRows)

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

		var wc datatypes.JSONMap
		if groupStats.WordCountsPerDay != nil {
			if day, ok := groupStats.WordCountsPerDay[dateKey]; ok {
				wc = int64CountsToJSONMap(day)
			}
		}
		if wc == nil {
			wc = datatypes.JSONMap{}
		}

		rows = append(rows, postgres.GroupStats{
			GroupID:    group.ID,
			Date:       d,
			MsgCount:   int64(cnt),
			WordCounts: wc,
		})
	}

	if len(rows) == 0 {
		return nil
	}

	const chunkSize = 500
	var changedRows int64
	for i := 0; i < len(rows); i += chunkSize {
		end := i + chunkSize
		if end > len(rows) {
			end = len(rows)
		}
		chunk := rows[i:end]
		res := h.DB.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "group_id"}, {Name: "date"}},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"msg_count":   gorm.Expr("EXCLUDED.msg_count"),
				"word_counts": gorm.Expr("CASE WHEN groups_stats.word_counts IS DISTINCT FROM EXCLUDED.word_counts THEN EXCLUDED.word_counts ELSE groups_stats.word_counts END"),
			}),
			Where: clause.Where{Exprs: []clause.Expression{
				clause.Expr{SQL: "groups_stats.msg_count IS DISTINCT FROM EXCLUDED.msg_count OR groups_stats.word_counts IS DISTINCT FROM EXCLUDED.word_counts"},
			}},
		}).Create(&chunk)
		if res.Error != nil {
			return res.Error
		}
		changedRows += res.RowsAffected
	}

	log.Printf("group_stats upsert metrics: group_id=%d rows_total=%d rows_changed=%d rows_unchanged=%d", group_id, len(rows), changedRows, int64(len(rows))-changedRows)

	return nil
}

func (h *Handler) markMessagesAsUsed(messageIDs []uuid.UUID) error {
	if len(messageIDs) == 0 {
		return nil
	}

	// For small lists just use batched IN updates.
	const smallBatch = 5000
	const insertBatch = 1000

	tx := h.DB.Begin()
	if tx.Error != nil {
		return tx.Error
	}

	// fast path for modest sizes
	if len(messageIDs) <= smallBatch {
		var changedRows int64
		for i := 0; i < len(messageIDs); i += smallBatch {
			end := i + smallBatch
			if end > len(messageIDs) {
				end = len(messageIDs)
			}
			chunk := messageIDs[i:end]

			res := tx.Model(&postgres.Message{}).
				Where("id IN ?", chunk).
				Where("used_for_stats IS DISTINCT FROM ?", true).
				Update("used_for_stats", true)
			if res.Error != nil {
				tx.Rollback()
				return res.Error
			}
			changedRows += res.RowsAffected
		}
		if err := tx.Commit().Error; err != nil {
			return err
		}

		log.Printf("messages used_for_stats metrics: ids_total=%d rows_changed=%d rows_unchanged=%d", len(messageIDs), changedRows, int64(len(messageIDs))-changedRows)
		return nil
	}

	// For very large lists, create a temp table and bulk insert IDs, then update via JOIN.
	// Use a short random suffix to avoid name collisions.
	tmpSuffix := strings.ReplaceAll(uuid.New().String(), "-", "")[:12]
	tmpTable := "tmp_msg_ids_" + tmpSuffix

	createSQL := fmt.Sprintf("CREATE TEMP TABLE %s (id uuid PRIMARY KEY) ON COMMIT DROP", tmpTable)
	if err := tx.Exec(createSQL).Error; err != nil {
		tx.Rollback()
		return err
	}

	// Bulk insert into temp table in manageable batches.
	for i := 0; i < len(messageIDs); i += insertBatch {
		end := i + insertBatch
		if end > len(messageIDs) {
			end = len(messageIDs)
		}
		chunk := messageIDs[i:end]

		placeholders := make([]string, 0, len(chunk))
		args := make([]interface{}, 0, len(chunk))
		for j, id := range chunk {
			placeholders = append(placeholders, fmt.Sprintf("($%d)", j+1))
			args = append(args, id)
		}

		insertSQL := fmt.Sprintf("INSERT INTO %s (id) VALUES %s ON CONFLICT DO NOTHING", tmpTable, strings.Join(placeholders, ","))
		if err := tx.Exec(insertSQL, args...).Error; err != nil {
			tx.Rollback()
			return err
		}
	}

	// Perform single update joining on the temp table.
	updateSQL := fmt.Sprintf("UPDATE messages SET used_for_stats = true FROM %s t WHERE messages.id = t.id AND messages.used_for_stats IS DISTINCT FROM true", tmpTable)
	res := tx.Exec(updateSQL)
	if res.Error != nil {
		tx.Rollback()
		return res.Error
	}

	if err := tx.Commit().Error; err != nil {
		return err
	}

	log.Printf("messages used_for_stats metrics: ids_total=%d rows_changed=%d rows_unchanged=%d", len(messageIDs), res.RowsAffected, int64(len(messageIDs))-res.RowsAffected)
	return nil
}
