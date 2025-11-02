package handlers

import (
	"net/http"
	"sort"
	"time"

	"github.com/labstack/echo/v4"
	echoKitSchemas "github.com/nrf24l01/go-web-utils/echokit/schemas"
	"github.com/nrf24l01/tg_group_statistics/backend/schemas"

	"github.com/nrf24l01/tg_group_statistics/analyzer/postgres"
)

func (h *Handler) GetMessagesPerUserPerDay(c echo.Context) error {
	req := c.Get("validatedQuery").(*schemas.TimeRangeQuery)

	// assume timestamps are seconds since epoch
	start := time.Unix(int64(req.StartTimestamp/1000), 0).UTC()
	end := time.Unix(int64(req.EndTimestamp/1000), 0).UTC()
	startDay := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
	endDay := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, time.UTC)

	var stats []postgres.UserStats
	if err := h.DB.Preload("Sender").Joins("JOIN groups ON groups.id = users_stats.group_id").Where("groups.tg_group_id = ?", req.ChatID).Where("users_stats.date >= ? AND users_stats.date <= ?", startDay, endDay).Where("users_stats.msg_count > 0").Find(&stats).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, echoKitSchemas.DefaultInternalErrorResponse)
	}

	resp := schemas.MessagesPerUserPerDayResponse{}
	for _, s := range stats {
		username := ""
		name := ""
		if s.Sender.Username != nil {
			username = *s.Sender.Username
		}
		if s.Sender.Nick != nil {
			name = *s.Sender.Nick
		}
		resp.Stats = append(resp.Stats, schemas.UserStatsPerDay{
			TotalMessages: int(s.MsgCount),
			Username:      username,
			Name:          name,
			Day:           s.Date,
		})
	}

	return c.JSON(http.StatusOK, resp)
}

func (h *Handler) GetPerDayUserColumnHandler(c echo.Context) error {
	req := c.Get("validatedQuery").(*schemas.TimeRangeQuery)

	// assume timestamps are seconds since epoch
	start := time.Unix(int64(req.StartTimestamp/1000), 0).UTC()
	end := time.Unix(int64(req.EndTimestamp/1000), 0).UTC()
	startDay := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
	endDay := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, time.UTC)

	var stats []postgres.UserStats
	if err := h.DB.Preload("Sender").Joins("JOIN groups ON groups.id = users_stats.group_id").Where("groups.tg_group_id = ?", req.ChatID).Where("users_stats.date >= ? AND users_stats.date <= ?", startDay, endDay).Where("users_stats.msg_count > 0").Order("users_stats.date asc").Find(&stats).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, echoKitSchemas.DefaultInternalErrorResponse)
	}

	counts := map[string]map[string]int{}
	usersSet := map[string]struct{}{}

	userKeyFor := func(s postgres.UserStats) string {
		if s.Sender.Username != nil && *s.Sender.Username != "" {
			return *s.Sender.Username
		}
		if s.Sender.Nick != nil && *s.Sender.Nick != "" {
			return *s.Sender.Nick
		}
		return s.SenderID.String()
	}

	for _, s := range stats {
		dayStr := s.Date.UTC().Format(time.RFC3339)
		if _, ok := counts[dayStr]; !ok {
			counts[dayStr] = map[string]int{}
		}
		key := userKeyFor(s)
		counts[dayStr][key] = int(s.MsgCount)
		usersSet[key] = struct{}{}
	}

	users := make([]string, 0, len(usersSet))
	for u := range usersSet {
		users = append(users, u)
	}
	sort.Strings(users)

	rows := make([]map[string]interface{}, 0)
	for d := startDay; !d.After(endDay); d = d.Add(24 * time.Hour) {
		dayStr := d.UTC().Format(time.RFC3339)
		row := map[string]interface{}{"day": dayStr}
		for _, u := range users {
			val := 0
			if m, ok := counts[dayStr]; ok {
				if v, ok2 := m[u]; ok2 {
					val = v
				}
			}
			row[u] = val
		}
		rows = append(rows, row)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{"stats": rows})
}

func (h *Handler) GetMessagesPerDay(c echo.Context) error {
	req := c.Get("validatedQuery").(*schemas.TimeRangeQuery)

	// assume timestamps are seconds since epoch
	start := time.Unix(int64(req.StartTimestamp/1000), 0).UTC()
	end := time.Unix(int64(req.EndTimestamp/1000), 0).UTC()
	startDay := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
	endDay := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, time.UTC)

	var stats []postgres.GroupStats
	if err := h.DB.Joins("JOIN groups ON groups.id = groups_stats.group_id").Where("groups.tg_group_id = ?", req.ChatID).Where("groups_stats.date >= ? AND groups_stats.date <= ?", startDay, endDay).Where("groups_stats.msg_count > 0").Order("groups_stats.date asc").Find(&stats).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, echoKitSchemas.DefaultInternalErrorResponse)
	}

	counts := map[string]int{}
	for _, s := range stats {
		dayStr := s.Date.UTC().Format(time.RFC3339)
		counts[dayStr] = int(s.MsgCount)
	}

	rows := make([]schemas.DayTotal, 0)
	for d := startDay; !d.After(endDay); d = d.Add(24 * time.Hour) {
		dayStr := d.UTC().Format(time.RFC3339)
		val := 0
		if v, ok := counts[dayStr]; ok {
			val = v
		}
		rows = append(rows, schemas.DayTotal{Day: dayStr, TotalMsg: val})
	}

	return c.JSON(http.StatusOK, schemas.DayTotalsResponse{Stats: rows})
}

func (h *Handler) GetMessagesPerUserTotal(c echo.Context) error {
	req := c.Get("validatedQuery").(*schemas.TimeRangeQuery)

	// assume timestamps are seconds since epoch
	start := time.Unix(int64(req.StartTimestamp/1000), 0).UTC()
	end := time.Unix(int64(req.EndTimestamp/1000), 0).UTC()
	startDay := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
	endDay := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, time.UTC)

	var stats []postgres.UserStats
	if err := h.DB.Preload("Sender").Joins("JOIN groups ON groups.id = users_stats.group_id").Where("groups.tg_group_id = ?", req.ChatID).Where("users_stats.date >= ? AND users_stats.date <= ?", startDay, endDay).Where("users_stats.msg_count > 0").Find(&stats).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, echoKitSchemas.DefaultInternalErrorResponse)
	}

	userKeyFor := func(s postgres.UserStats) string {
		if s.Sender.Username != nil && *s.Sender.Username != "" {
			return *s.Sender.Username
		}
		if s.Sender.Nick != nil && *s.Sender.Nick != "" {
			return *s.Sender.Nick
		}
		return s.SenderID.String()
	}

	totals := map[string]int{}
	for _, s := range stats {
		k := userKeyFor(s)
		totals[k] += int(s.MsgCount)
	}

	users := make([]string, 0, len(totals))
	for u := range totals {
		users = append(users, u)
	}
	sort.Strings(users)

	rows := make([]schemas.UserTotal, 0, len(users))
	for _, u := range users {
		rows = append(rows, schemas.UserTotal{User: u, TotalMsg: totals[u]})
	}

	return c.JSON(http.StatusOK, schemas.UserTotalsResponse{Stats: rows})
}