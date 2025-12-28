package handlers

import (
	"database/sql"
	"net/http"
	"sort"
	"time"

	"github.com/labstack/echo/v4"
	echoKitSchemas "github.com/nrf24l01/go-web-utils/echokit/schemas"
	"github.com/nrf24l01/tg_group_statistics/backend/schemas"
)

func (h *Handler) GetWordsPerUserTotal(c echo.Context) error {
	req := c.Get("validatedQuery").(*schemas.TimeRangeQuery)

	start := time.Unix(int64(req.StartTimestamp/1000), 0).UTC()
	end := time.Unix(int64(req.EndTimestamp/1000), 0).UTC()
	startDay := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
	endDay := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, time.UTC)

	type wordsRow struct {
		SenderID   string         `gorm:"column:sender_id"`
		Username   sql.NullString `gorm:"column:username"`
		Nick       sql.NullString `gorm:"column:nick"`
		TotalWords int64          `gorm:"column:total_words"`
	}

	rows := make([]wordsRow, 0)
	query := `
		SELECT
			u.id::text AS sender_id,
			u.username AS username,
			u.nick AS nick,
			COALESCE(SUM((j.value)::bigint), 0) AS total_words
		FROM users_stats us
		JOIN groups g ON g.id = us.group_id
		JOIN users u ON u.id = us.sender_id
		LEFT JOIN LATERAL jsonb_each_text(us.word_counts) AS j(key, value) ON true
		WHERE g.tg_group_id = ?
		  AND us.date >= ?
		  AND us.date <= ?
		GROUP BY u.id, u.username, u.nick
	`
	if err := h.DB.Raw(query, req.ChatID, startDay, endDay).Scan(&rows).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, echoKitSchemas.DefaultInternalErrorResponse)
	}

	totals := map[string]int{}
	for _, r := range rows {
		key := r.SenderID
		if r.Username.Valid && r.Username.String != "" {
			key = r.Username.String
		} else if r.Nick.Valid && r.Nick.String != "" {
			key = r.Nick.String
		} else {
			continue
		}
		totals[key] += int(r.TotalWords)
	}

	users := make([]string, 0, len(totals))
	for u := range totals {
		users = append(users, u)
	}
	sort.Strings(users)

	respRows := make([]schemas.UserWordsTotal, 0, len(users))
	for _, u := range users {
		respRows = append(respRows, schemas.UserWordsTotal{User: u, TotalWords: totals[u]})
	}

	return c.JSON(http.StatusOK, schemas.UserWordsTotalsResponse{Stats: respRows})
}
