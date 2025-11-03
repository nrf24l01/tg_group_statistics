package main

import (
	"fmt"
	"log"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nrf24l01/tg_group_statistics/analyzer/core"
	"github.com/nrf24l01/tg_group_statistics/analyzer/postgres"
	"gorm.io/gorm"
)

type UserGroupPair struct {
	UserID  string
	GroupID string
}

// cache for resolved group UUIDs: key is the original group identifier string
// (either Telegram numeric id as text, or UUID string). Use RWMutex for safe
// concurrent access.
var groupUUIDCache = struct {
	sync.RWMutex
	m map[string]uuid.UUID
}{m: make(map[string]uuid.UUID)}

// resolveGroupUUID tries to resolve the given group identifier which may be
// - a Telegram numeric ID (int64) stored in messages.chat_id, or
// - a UUID string representing groups.id
// If a Telegram ID is provided and no matching groups row exists, a new groups
// row will be created (same behavior as other code paths in this file).
func resolveGroupUUID(db *gorm.DB, groupID string) (uuid.UUID, error) {
	// Check cache first
	groupUUIDCache.RLock()
	if u, ok := groupUUIDCache.m[groupID]; ok {
		groupUUIDCache.RUnlock()
		return u, nil
	}
	groupUUIDCache.RUnlock()
	// try parse as int64 (Telegram ID)
	if tgID, err := strconv.ParseInt(groupID, 10, 64); err == nil {
		var grp postgres.Group
		if err := db.Where("tg_group_id = ?", tgID).First(&grp).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				grp = postgres.Group{TgGroupID: tgID}
				if err := db.Create(&grp).Error; err != nil {
					return uuid.Nil, err
				}
			} else {
				return uuid.Nil, err
			}
		}
		// store in cache
		groupUUIDCache.Lock()
		groupUUIDCache.m[groupID] = grp.ID
		groupUUIDCache.Unlock()
		return grp.ID, nil
	}

	// fallback: try parse as UUID
	u, err := uuid.Parse(groupID)
	if err != nil {
		return uuid.Nil, err
	}
	groupUUIDCache.Lock()
	groupUUIDCache.m[groupID] = u
	groupUUIDCache.Unlock()
	return u, nil
}

func getUserGroupPairs(db *gorm.DB) ([]UserGroupPair, error) {
	var pairs []UserGroupPair
	rows, err := db.Raw("SELECT DISTINCT sender_id AS user_id, chat_id AS group_id FROM messages").Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var userID string
		var groupID string
		if err := rows.Scan(&userID, &groupID); err != nil {
			return nil, err
		}
		pairs = append(pairs, UserGroupPair{UserID: userID, GroupID: groupID})
	}

	return pairs, nil
}

func getDaysPerGroup(db *gorm.DB) (map[string][]string, error) {
	days := make(map[string][]string)

	rows, err := db.Raw("SELECT DISTINCT chat_id AS group_id, (send_time AT TIME ZONE 'UTC')::date AS date FROM messages ORDER BY group_id, date").Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var groupID string
		var date string
		if err := rows.Scan(&groupID, &date); err != nil {
			return nil, err
		}
		days[groupID] = append(days[groupID], date)
	}

	return days, nil
}

// dateRange represents an inclusive day range [Start .. End]
type dateRange struct {
	Start time.Time
	End   time.Time
}

// buildDateRanges receives a list of date strings (either RFC3339 or YYYY-MM-DD)
// and returns a slice of contiguous date ranges. Each range is inclusive and
// expressed in UTC at midnight. Non-parseable entries are skipped.
func buildDateRanges(dates []string) ([]dateRange, error) {
	if len(dates) == 0 {
		return nil, nil
	}
	seen := make(map[string]struct{})
	ts := make([]time.Time, 0, len(dates))
	for _, ds := range dates {
		var t time.Time
		var err error
		t, err = time.Parse("2006-01-02", ds)
		if err != nil {
			t, err = time.Parse(time.RFC3339, ds)
			if err != nil {
				continue
			}
		}
		key := t.UTC().Format("2006-01-02")
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		// normalize to UTC midnight
		d, _ := time.Parse("2006-01-02", key)
		ts = append(ts, d)
	}

	if len(ts) == 0 {
		return nil, nil
	}

	sort.Slice(ts, func(i, j int) bool { return ts[i].Before(ts[j]) })

	var ranges []dateRange
	start := ts[0]
	prev := ts[0]
	for i := 1; i < len(ts); i++ {
		cur := ts[i]
		if cur.Sub(prev) == 24*time.Hour {
			// contiguous
			prev = cur
			continue
		}
		// gap -> close previous range
		ranges = append(ranges, dateRange{Start: start, End: prev})
		start = cur
		prev = cur
	}
	// append final range
	ranges = append(ranges, dateRange{Start: start, End: prev})

	return ranges, nil
}

func calculateUserStatsPerGroup(db *gorm.DB, userID string, groupID string, dates []string) error {
	// parse sender UUID
	senderUUID, err := uuid.Parse(userID)
	if err != nil {
		return err
	}

	// resolve groupID: messages.store chat IDs as int64 (Telegram IDs). Try parse int64 and lookup Group.
	var groupUUID uuid.UUID
	var tgID int64
	var hasTgID bool
	if parsed, err := strconv.ParseInt(groupID, 10, 64); err == nil {
		tgID = parsed
		hasTgID = true
		var grp postgres.Group
		if err := db.Where("tg_group_id = ?", tgID).First(&grp).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				// create group record so we have its UUID
				grp = postgres.Group{TgGroupID: tgID}
				if err := db.Create(&grp).Error; err != nil {
					return fmt.Errorf("failed to create group for tg_id %d: %w", tgID, err)
				}
			} else {
				return err
			}
		}
		groupUUID = grp.ID
	} else {
		// fallback: maybe groupID already a UUID string
		groupUUID, err = uuid.Parse(groupID)
		if err != nil {
			return fmt.Errorf("invalid group id %q: %w", groupID, err)
		}
	}

	// Normalize requested dates to YYYY-MM-DD and deduplicate
	uniq := make(map[string]struct{})
	var dateKeys []string
	origMap := make(map[string]string) // map key->original representation (prefer YYYY-MM-DD)
	for _, ds := range dates {
		var day time.Time
		day, err = time.Parse(time.RFC3339, ds)
		if err != nil {
			day, err = time.Parse("2006-01-02", ds)
			if err != nil {
				log.Printf("calculateUserStatsPerGroup: skipping invalid date %q: %v", ds, err)
				continue
			}
		}
		key := day.UTC().Format("2006-01-02")
		if _, seen := uniq[key]; !seen {
			uniq[key] = struct{}{}
			dateKeys = append(dateKeys, key)
			origMap[key] = key
		}
	}

	if len(dateKeys) == 0 {
		return nil
	}

	// Check which of these dates already exist in users_stats; break the
	// requested dates into contiguous ranges and query each range. This
	// keeps the checks small and index-friendly for Timescale/Postgres.
	existing := make(map[string]struct{})
	ranges, err := buildDateRanges(dateKeys)
	if err != nil {
		return err
	}
	for _, r := range ranges {
		startDate := time.Date(r.Start.Year(), r.Start.Month(), r.Start.Day(), 0, 0, 0, 0, time.UTC)
		endDate := time.Date(r.End.Year(), r.End.Month(), r.End.Day(), 0, 0, 0, 0, time.UTC)

		existRows, err := db.Raw("SELECT date FROM users_stats WHERE sender_id = ? AND group_id = ? AND date >= ? AND date <= ?", senderUUID, groupUUID, startDate, endDate).Rows()
		if err != nil {
			return err
		}
		for existRows.Next() {
			var dt time.Time
			if err := existRows.Scan(&dt); err != nil {
				existRows.Close()
				return err
			}
			existing[dt.Format("2006-01-02")] = struct{}{}
		}
		existRows.Close()
	}

	// Build list of dates that still need calculating
	var remainingKeys []string
	for _, k := range dateKeys {
		if _, ok := existing[k]; !ok {
			remainingKeys = append(remainingKeys, k)
		}
	}

	if len(remainingKeys) == 0 {
		// nothing to compute
		return nil
	}

	// Query messages counts restricted to remainingKeys.
	// Use a time range [start, end) instead of casting send_time to date in WHERE
	// so Postgres/Timescale can use indexes and chunk pruning.
	// remainingKeys are dates in YYYY-MM-DD format; compute min/max.
	var minDate, maxDate time.Time
	for i, k := range remainingKeys {
		d, err := time.Parse("2006-01-02", k)
		if err != nil {
			continue
		}
		if i == 0 || d.Before(minDate) {
			minDate = d
		}
		if i == 0 || d.After(maxDate) {
			maxDate = d
		}
	}
	// start is minDate 00:00 UTC, end is (maxDate + 1 day) 00:00 UTC
	start := time.Date(minDate.Year(), minDate.Month(), minDate.Day(), 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 0, int(maxDate.Sub(minDate).Hours()/24)+1)

	args := make([]interface{}, 0, 4)
	if hasTgID {
		args = append(args, senderUUID, tgID, start, end)
	} else {
		args = append(args, senderUUID, groupUUID, start, end)
	}
	query := "SELECT (send_time AT TIME ZONE 'UTC')::date AS date, COUNT(*) AS cnt FROM messages WHERE sender_id = ? AND chat_id = ? AND send_time >= ? AND send_time < ? GROUP BY date"
	rows, err := db.Raw(query, args...).Rows()
	if err != nil {
		return err
	}
	defer rows.Close()

	counts := make(map[string]int64)
	for rows.Next() {
		var dt time.Time
		var cnt int64
		if err := rows.Scan(&dt, &cnt); err != nil {
			return err
		}
		counts[dt.Format("2006-01-02")] = cnt
	}

	// Build slice of UserStats only for dates that still need calculating (remainingKeys)
	var statsList []postgres.UserStats
	for _, key := range remainingKeys {
		// parse key which is in YYYY-MM-DD format
		day, err2 := time.Parse("2006-01-02", key)
		if err2 != nil {
			log.Printf("calculateUserStatsPerGroup: unexpected parse error for key %q: %v", key, err2)
			continue
		}
		day = time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC)
		cnt := counts[key]

		statsList = append(statsList, postgres.UserStats{
			SenderID: senderUUID,
			GroupID:  groupUUID,
			Date:     day,
			MsgCount: cnt,
		})
	}

	if len(statsList) == 0 {
		return nil
	}

	// Bulk insert: these dates were confirmed not to exist in users_stats, so plain Create is sufficient
	if err := db.Create(&statsList).Error; err != nil {
		return err
	}

	return nil
}

func update(db *gorm.DB, config *core.Config) {
	log.Print("Update task started")
	userGroupPairs, err := getUserGroupPairs(db)
	if err != nil {
		log.Fatalf("failed to get user-group pairs: %v", err)
	}

	daysPerGroup, err := getDaysPerGroup(db)
	if err != nil {
		log.Fatalf("failed to get days per group: %v", err)
	}
	
	// Run calculations concurrently with a semaphore to limit parallel DB work.
	const maxWorkers = 8
	semaphore := make(chan struct{}, maxWorkers)
	var wg sync.WaitGroup

	today := time.Now().UTC()
	todayKey := today.Format("2006-01-02")

	for _, pair := range userGroupPairs {
		wg.Add(1)
		p := pair
		go func() {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			origDates := daysPerGroup[p.GroupID]
			if len(origDates) == 0 {
				log.Printf("User %s Group %s: no message days found -> skip", p.UserID, p.GroupID)
				return
			}

			resolvedGroupUUID, err := resolveGroupUUID(db, p.GroupID)
			if err != nil {
				log.Printf("failed to resolve group id %s for user %s: %v", p.GroupID, p.UserID, err)
				return
			}

			// Parse sender UUID so we can compare typed UUIDs (avoid ::text)
			senderUUID, err := uuid.Parse(p.UserID)
			if err != nil {
				log.Printf("invalid sender uuid %s: %v", p.UserID, err)
				return
			}

			var existCount int64
			if err := db.Raw("SELECT COUNT(*) FROM users_stats WHERE sender_id = ? AND group_id = ?", senderUUID, resolvedGroupUUID).Scan(&existCount).Error; err != nil {
				log.Printf("failed to check existing stats for user %s group %s: %v", p.UserID, p.GroupID, err)
				return
			}

			var filtered []string
			if existCount == 0 {
				filtered = origDates
			} else {
				for _, ds := range origDates {
					var d time.Time
					var err error
					d, err = time.Parse("2006-01-02", ds)
					if err != nil {
						d, err = time.Parse(time.RFC3339, ds)
						if err != nil {
							continue
						}
					}
					if d.UTC().Format("2006-01-02") == todayKey {
						filtered = append(filtered, ds)
					}
				}
			}

			if existCount == 0 {
				log.Printf("User %s Group %s: no existing stats -> backfill %d date(s)", p.UserID, p.GroupID, len(filtered))
			}

			if len(filtered) == 0 {
				return
			}

			if err := calculateUserStatsPerGroup(db, p.UserID, p.GroupID, filtered); err != nil {
				log.Printf("failed to calculate stats for user %s in group %s: %v", p.UserID, p.GroupID, err)
			}
		}()
	}

	// Also calculate per-group totals (group-level stats) for each group in daysPerGroup.
	for groupID, dates := range daysPerGroup {
		// decide which dates to process for group: if no existing stats -> backfill all, else only today's date
		var filtered []string

		var existCount int64
		// Resolve group identifier to UUID before checking groups_stats
		resolvedGroupUUID, err := resolveGroupUUID(db, groupID)
		if err != nil {
			log.Printf("failed to resolve group id %s: %v", groupID, err)
			continue
		}
		if err := db.Raw("SELECT COUNT(*) FROM groups_stats WHERE group_id = ?", resolvedGroupUUID).Scan(&existCount).Error; err != nil {
			log.Printf("failed to check existing group stats for group %s: %v", groupID, err)
			continue
		}

		if existCount == 0 {
			filtered = dates
		} else {
			for _, ds := range dates {
				var d time.Time
				var err error
				d, err = time.Parse("2006-01-02", ds)
				if err != nil {
					d, err = time.Parse(time.RFC3339, ds)
					if err != nil {
						continue
					}
				}
				if d.UTC().Format("2006-01-02") == todayKey {
					filtered = append(filtered, ds)
				}
			}
		}

		// Log decision about processing this group
		if existCount == 0 {
			log.Printf("Group %s: no existing stats -> backfill %d date(s)", groupID, len(filtered))
		} else if len(filtered) > 0 {
			log.Printf("Group %s: existing stats present -> update %d date(s): %v", groupID, len(filtered), filtered)
		}

		if len(filtered) == 0 {
			continue
		}

		wg.Add(1)
		gID := groupID
		d := filtered
		go func() {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			if err := calculateGroupStatsPerGroup(db, gID, d); err != nil {
				log.Printf("failed to calculate group stats for group %s: %v", gID, err)
			}
		}()
	}

	wg.Wait()
	log.Printf("Update task finished")
}

// calculateGroupStatsPerGroup computes per-day message counts for a group (chat) and inserts missing rows into groups_stats.
func calculateGroupStatsPerGroup(db *gorm.DB, groupID string, dates []string) error {
	// resolve groupID: messages.store chat IDs as int64 (Telegram IDs). Try parse int64 and lookup Group.
	var groupUUID uuid.UUID
	var tgID int64
	var hasTgID bool
	if parsed, err := strconv.ParseInt(groupID, 10, 64); err == nil {
		tgID = parsed
		hasTgID = true
		var grp postgres.Group
		if err := db.Where("tg_group_id = ?", tgID).First(&grp).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				grp = postgres.Group{TgGroupID: tgID}
				if err := db.Create(&grp).Error; err != nil {
					return fmt.Errorf("failed to create group for tg_id %d: %w", tgID, err)
				}
			} else {
				return err
			}
		}
		groupUUID = grp.ID
	} else {
		// fallback: maybe groupID already a UUID string
		var err error
		groupUUID, err = uuid.Parse(groupID)
		if err != nil {
			return fmt.Errorf("invalid group id %q: %w", groupID, err)
		}
	}

	// Normalize requested dates to YYYY-MM-DD and deduplicate
	uniq := make(map[string]struct{})
	var dateKeys []string
	for _, ds := range dates {
		var day time.Time
		var err error
		day, err = time.Parse(time.RFC3339, ds)
		if err != nil {
			day, err = time.Parse("2006-01-02", ds)
			if err != nil {
				log.Printf("calculateGroupStatsPerGroup: skipping invalid date %q: %v", ds, err)
				continue
			}
		}
		key := day.UTC().Format("2006-01-02")
		if _, seen := uniq[key]; !seen {
			uniq[key] = struct{}{}
			dateKeys = append(dateKeys, key)
		}
	}

	if len(dateKeys) == 0 {
		return nil
	}

	// Check which of these dates already exist in groups_stats; use a range
	// lookup (date >= min AND date <= max) so Timescale/Postgres can use
	// indexes/chunk-pruning instead of a huge IN(...) list.
	var minExist, maxExist time.Time
	for i, k := range dateKeys {
		d, err := time.Parse("2006-01-02", k)
		if err != nil {
			continue
		}
		if i == 0 || d.Before(minExist) {
			minExist = d
		}
		if i == 0 || d.After(maxExist) {
			maxExist = d
		}
	}
	// If parsing failed for all keys, nothing to do.
	if minExist.IsZero() {
		return nil
	}

	startDate := time.Date(minExist.Year(), minExist.Month(), minExist.Day(), 0, 0, 0, 0, time.UTC)
	endDate := time.Date(maxExist.Year(), maxExist.Month(), maxExist.Day(), 0, 0, 0, 0, time.UTC)

	existRows, err := db.Raw("SELECT date FROM groups_stats WHERE group_id = ? AND date >= ? AND date <= ?", groupUUID, startDate, endDate).Rows()
	if err != nil {
		return err
	}
	defer existRows.Close()

	existing := make(map[string]struct{})
	ranges, err := buildDateRanges(dateKeys)
	if err != nil {
		return err
	}
	for _, r := range ranges {
		startDate := time.Date(r.Start.Year(), r.Start.Month(), r.Start.Day(), 0, 0, 0, 0, time.UTC)
		endDate := time.Date(r.End.Year(), r.End.Month(), r.End.Day(), 0, 0, 0, 0, time.UTC)

		existRows, err := db.Raw("SELECT date FROM groups_stats WHERE group_id = ? AND date >= ? AND date <= ?", groupUUID, startDate, endDate).Rows()
		if err != nil {
			return err
		}
		for existRows.Next() {
			var dt time.Time
			if err := existRows.Scan(&dt); err != nil {
				existRows.Close()
				return err
			}
			existing[dt.Format("2006-01-02")] = struct{}{}
		}
		existRows.Close()
	}

	// Build list of dates that still need calculating
	var remainingKeys []string
	for _, k := range dateKeys {
		if _, ok := existing[k]; !ok {
			remainingKeys = append(remainingKeys, k)
		}
	}

	if len(remainingKeys) == 0 {
		return nil
	}

	// Query messages counts restricted to remainingKeys using a time range so
	// Timescale/Postgres can use indexes/chunk pruning.
	var minDate, maxDate time.Time
	for i, k := range remainingKeys {
		d, err := time.Parse("2006-01-02", k)
		if err != nil {
			continue
		}
		if i == 0 || d.Before(minDate) {
			minDate = d
		}
		if i == 0 || d.After(maxDate) {
			maxDate = d
		}
	}
	start := time.Date(minDate.Year(), minDate.Month(), minDate.Day(), 0, 0, 0, 0, time.UTC)
	end := time.Date(maxDate.Year(), maxDate.Month(), maxDate.Day(), 0, 0, 0, 0, time.UTC).Add(24 * time.Hour)

	args := make([]interface{}, 0, 3)
	if hasTgID {
		args = append(args, tgID, start, end)
	} else {
		args = append(args, groupUUID, start, end)
	}
	query := "SELECT (send_time AT TIME ZONE 'UTC')::date AS date, COUNT(*) AS cnt FROM messages WHERE chat_id = ? AND send_time >= ? AND send_time < ? GROUP BY date"
	rows, err := db.Raw(query, args...).Rows()
	if err != nil {
		return err
	}
	defer rows.Close()

	counts := make(map[string]int64)
	for rows.Next() {
		var dt time.Time
		var cnt int64
		if err := rows.Scan(&dt, &cnt); err != nil {
			return err
		}
		counts[dt.Format("2006-01-02")] = cnt
	}

	var statsList []postgres.GroupStats
	for _, key := range remainingKeys {
		day, err := time.Parse("2006-01-02", key)
		if err != nil {
			log.Printf("calculateGroupStatsPerGroup: unexpected parse error for key %q: %v", key, err)
			continue
		}
		day = time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC)
		cnt := counts[key]

		statsList = append(statsList, postgres.GroupStats{
			GroupID:  groupUUID,
			Date:     day,
			MsgCount: cnt,
		})
	}

	if len(statsList) == 0 {
		return nil
	}

	if err := db.Create(&statsList).Error; err != nil {
		return err
	}

	return nil
}