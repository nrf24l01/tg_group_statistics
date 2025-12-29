package update

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

func (h *Handler) updateMessagesWordCounts(updates map[uuid.UUID]datatypes.JSONMap) error {
	if len(updates) == 0 {
		return nil
	}

	tx := h.DB.Begin()
	if tx.Error != nil {
		return tx.Error
	}

	tmpSuffix := strings.ReplaceAll(uuid.New().String(), "-", "")[:12]
	tmpTable := "tmp_msg_word_counts_" + tmpSuffix

	createSQL := fmt.Sprintf("CREATE TEMP TABLE %s (id uuid PRIMARY KEY, word_counts jsonb NOT NULL) ON COMMIT DROP", tmpTable)
	if err := tx.Exec(createSQL).Error; err != nil {
		tx.Rollback()
		return err
	}

	const insertBatch = 500
	ids := make([]uuid.UUID, 0, len(updates))
	for id := range updates {
		ids = append(ids, id)
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
			jm := updates[id]
			b, err := json.Marshal(jm)
			if err != nil {
				tx.Rollback()
				return err
			}
			// ($1, $2::jsonb), ($3, $4::jsonb) ...
			placeholders = append(placeholders, fmt.Sprintf("($%d, $%d::jsonb)", j*2+1, j*2+2))
			args = append(args, id, string(b))
		}

		insertSQL := fmt.Sprintf("INSERT INTO %s (id, word_counts) VALUES %s ON CONFLICT (id) DO UPDATE SET word_counts = EXCLUDED.word_counts", tmpTable, strings.Join(placeholders, ","))
		if err := tx.Exec(insertSQL, args...).Error; err != nil {
			tx.Rollback()
			return err
		}
	}

	updateSQL := fmt.Sprintf(
		"UPDATE messages m SET word_counts = t.word_counts FROM %s t WHERE m.id = t.id AND (m.word_counts IS NULL OR m.word_counts = '{}'::jsonb)",
		tmpTable,
	)
	if err := tx.Exec(updateSQL).Error; err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}
