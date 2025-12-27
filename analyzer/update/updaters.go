package update

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nrf24l01/tg_group_statistics/analyzer/postgres"
	"gorm.io/datatypes"
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

			wc := datatypes.JSONMap{}
			if stats.WordCountsPerDay != nil {
				wc = wordCountsToJSONMap(stats.WordCountsPerDay[dateKey])
			}

			rows = append(rows, postgres.UserStats{
				SenderID: u.ID,
				GroupID:  group.ID,
				Date:     d,
				MsgCount: int64(cnt),
				WordCounts: wc,
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
			DoUpdates: clause.AssignmentColumns([]string{"msg_count", "word_counts"}),
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

		wc := datatypes.JSONMap{}
		if groupStats.WordCountsPerDay != nil {
			wc = wordCountsToJSONMap(groupStats.WordCountsPerDay[dateKey])
		}

		rows = append(rows, postgres.GroupStats{
			GroupID:  group.ID,
			Date:     d,
			MsgCount: int64(cnt),
			WordCounts: wc,
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
			DoUpdates: clause.AssignmentColumns([]string{"msg_count", "word_counts"}),
		}).Create(&chunk).Error; err != nil {
			return err
		}
	}

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
		for i := 0; i < len(messageIDs); i += smallBatch {
			end := i + smallBatch
			if end > len(messageIDs) {
				end = len(messageIDs)
			}
			chunk := messageIDs[i:end]

			if err := tx.Model(&postgres.Message{}).
				Where("id IN ?", chunk).
				Update("used_for_stats", true).Error; err != nil {
				tx.Rollback()
				return err
			}
		}
		return tx.Commit().Error
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
	updateSQL := fmt.Sprintf("UPDATE messages SET used_for_stats = true FROM %s t WHERE messages.id = t.id", tmpTable)
	if err := tx.Exec(updateSQL).Error; err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}

func (h *Handler) updateMessagesWordCounts(wordCounts map[uuid.UUID]datatypes.JSONMap) error {
	if len(wordCounts) == 0 {
		return nil
	}

	ids := make([]uuid.UUID, 0, len(wordCounts))
	for id := range wordCounts {
		ids = append(ids, id)
	}

	const smallBatch = 2000
	const insertBatch = 500

	tx := h.DB.Begin()
	if tx.Error != nil {
		return tx.Error
	}

	// Fast path: per-row batched updates
	if len(ids) <= smallBatch {
		for _, id := range ids {
			wc := wordCounts[id]
			if err := tx.Model(&postgres.Message{}).
				Where("id = ?", id).
				Update("word_counts", wc).Error; err != nil {
				tx.Rollback()
				return err
			}
		}
		return tx.Commit().Error
	}

	// Large path: temp table and single UPDATE via JOIN
	tmpSuffix := strings.ReplaceAll(uuid.New().String(), "-", "")[:12]
	tmpTable := "tmp_msg_word_counts_" + tmpSuffix

	createSQL := fmt.Sprintf("CREATE TEMP TABLE %s (id uuid PRIMARY KEY, word_counts jsonb NOT NULL) ON COMMIT DROP", tmpTable)
	if err := tx.Exec(createSQL).Error; err != nil {
		tx.Rollback()
		return err
	}

	for i := 0; i < len(ids); i += insertBatch {
		end := i + insertBatch
		if end > len(ids) {
			end = len(ids)
		}
		chunk := ids[i:end]

		placeholders := make([]string, 0, len(chunk))
		args := make([]interface{}, 0, len(chunk)*2)
		for j, id := range chunk {
			placeholders = append(placeholders, fmt.Sprintf("($%d,$%d)", j*2+1, j*2+2))
			args = append(args, id, wordCounts[id])
		}

		insertSQL := fmt.Sprintf("INSERT INTO %s (id, word_counts) VALUES %s ON CONFLICT (id) DO UPDATE SET word_counts = EXCLUDED.word_counts", tmpTable, strings.Join(placeholders, ","))
		if err := tx.Exec(insertSQL, args...).Error; err != nil {
			tx.Rollback()
			return err
		}
	}

	updateSQL := fmt.Sprintf("UPDATE messages m SET word_counts = t.word_counts FROM %s t WHERE m.id = t.id", tmpTable)
	if err := tx.Exec(updateSQL).Error; err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}