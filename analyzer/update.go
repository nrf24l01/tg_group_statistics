package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"
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

func getUserGroupPairs(db *gorm.DB) ([]UserGroupPair, error) {
	var pairs []UserGroupPair
	rows, err := db.Raw("SELECT DISTINCT sender_id::text AS user_id, chat_id::text AS group_id FROM messages").Rows()
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

	rows, err := db.Raw("SELECT DISTINCT chat_id::text AS group_id, (send_time AT TIME ZONE 'UTC')::date AS date FROM messages ORDER BY group_id, date").Rows()
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

func calculateUserStatsPerGroup(db *gorm.DB, userID string, groupID string, dates []string) error {
	// parse sender UUID
	senderUUID, err := uuid.Parse(userID)
	if err != nil {
		return err
	}

	// resolve groupID: messages.store chat IDs as int64 (Telegram IDs). Try parse int64 and lookup Group.
	var groupUUID uuid.UUID
	if tgID, err := strconv.ParseInt(groupID, 10, 64); err == nil {
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

	// Check which of these dates already exist in users_stats; skip existing
	placeholders := strings.Repeat("?,", len(dateKeys))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]interface{}, 0, 2+len(dateKeys))
	args = append(args, userID, groupID)
	for _, k := range dateKeys {
		args = append(args, k)
	}
	existQuery := fmt.Sprintf("SELECT date FROM users_stats WHERE sender_id::text = ? AND group_id::text = ? AND date IN (%s)", placeholders)
	existRows, err := db.Raw(existQuery, args...).Rows()
	if err != nil {
		return err
	}
	defer existRows.Close()

	existing := make(map[string]struct{})
	for existRows.Next() {
		var dt time.Time
		if err := existRows.Scan(&dt); err != nil {
			return err
		}
		existing[dt.Format("2006-01-02")] = struct{}{}
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

	// Query messages counts restricted to remainingKeys
	placeholders = strings.Repeat("?,", len(remainingKeys))
	placeholders = placeholders[:len(placeholders)-1]
	args = make([]interface{}, 0, 2+len(remainingKeys))
	args = append(args, userID, groupID)
	for _, k := range remainingKeys {
		args = append(args, k)
	}
	query := fmt.Sprintf(
		"SELECT (send_time AT TIME ZONE 'UTC')::date AS date, COUNT(*) AS cnt FROM messages WHERE sender_id::text = ? AND chat_id::text = ? AND (send_time AT TIME ZONE 'UTC')::date IN (%s) GROUP BY date",
		placeholders,
	)
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
		// decide which dates to process:
		// - if we have no entries in users_stats for this sender/group, backfill all available days
		// - otherwise only process today's date (past days won't change)
		origDates := daysPerGroup[pair.GroupID]
		var filtered []string

		var existCount int64
		if err := db.Raw("SELECT COUNT(*) FROM users_stats WHERE sender_id::text = ? AND group_id::text = ?", pair.UserID, pair.GroupID).Scan(&existCount).Error; err != nil {
			log.Printf("failed to check existing stats for user %s group %s: %v", pair.UserID, pair.GroupID, err)
			continue
		}

		if existCount == 0 {
			// backfill all dates we have for the group
			filtered = origDates
		} else {
			// only today's date
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

		if len(filtered) == 0 {
			// nothing to do for this group/user
			continue
		}

		wg.Add(1)
		// capture variables for goroutine
		p := pair
		d := filtered
		go func() {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			if err := calculateUserStatsPerGroup(db, p.UserID, p.GroupID, d); err != nil {
				log.Printf("failed to calculate stats for user %s in group %s: %v", p.UserID, p.GroupID, err)
			}
		}()
	}

	wg.Wait()
}