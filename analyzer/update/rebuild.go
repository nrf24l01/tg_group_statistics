package update

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/nrf24l01/tg_group_statistics/analyzer/postgres"
	"gorm.io/datatypes"
)

func (h *Handler) RebuildAllStatsWithWordCounts() error {
	groups, err := h.getAllGroupIds()
	if err != nil {
		return err
	}
	for _, gid := range groups {
		if err := h.rebuildGroupStatsWithWordCounts(gid); err != nil {
			return fmt.Errorf("rebuild group %d: %w", gid, err)
		}
	}
	return nil
}

func (h *Handler) rebuildGroupStatsWithWordCounts(groupID int64) error {
	var group postgres.Group
	if err := h.DB.Where("tg_group_id = ?", groupID).First(&group).Error; err != nil {
		return err
	}

	// Start from scratch for this group to avoid mixing old rows without word_counts.
	if err := h.DB.Where("group_id = ?", group.ID).Delete(&postgres.UserStats{}).Error; err != nil {
		return err
	}
	if err := h.DB.Where("group_id = ?", group.ID).Delete(&postgres.GroupStats{}).Error; err != nil {
		return err
	}

	userStats := make(map[int64]UserStats)
	groupStats := GroupStats{
		MessagesPerDay:   make(map[string]int),
		WordCountsPerDay: make(map[string]map[string]int64),
	}

	const batchSize = 5000
	offset := 0
	for {
		var messages []postgres.Message
		err := h.DB.Preload("Sender").
			Where("chat_id = ?", groupID).
			Order("send_time ASC").
			Limit(batchSize).
			Offset(offset).
			Find(&messages).Error
		if err != nil {
			return err
		}
		if len(messages) == 0 {
			break
		}
		offset += len(messages)

		wcUpdates := make(map[uuid.UUID]datatypes.JSONMap)

		for _, msg := range messages {
			senderTG := msg.Sender.TgUserID
			day := removeTime(msg.SendTime)
			dateKey := day.Format("02-01-2006")

			wc := map[string]int64{}
			if msg.MessageText != nil {
				wc = countWords(*msg.MessageText)
			}

			us := userStats[senderTG]
			if us.MessagesPerDay == nil {
				us.MessagesPerDay = make(map[string]int)
			}
			if us.WordCountsPerDay == nil {
				us.WordCountsPerDay = make(map[string]map[string]int64)
			}
			us.MessagesPerDay[dateKey]++
			us.TotalMessages++
			us.WordCountsPerDay[dateKey] = mergeWordCounts(us.WordCountsPerDay[dateKey], wc)
			userStats[senderTG] = us

			groupStats.MessagesPerDay[dateKey]++
			groupStats.TotalMessages++
			groupStats.WordCountsPerDay[dateKey] = mergeWordCounts(groupStats.WordCountsPerDay[dateKey], wc)

			if len(msg.WordCounts) == 0 {
				wcUpdates[msg.ID] = wordCountsToJSONMap(wc)
			}
		}

		if len(wcUpdates) > 0 {
			if err := h.updateMessagesWordCounts(wcUpdates); err != nil {
				return err
			}
		}
	}

	if err := h.applyUsersStats(groupID, userStats); err != nil {
		return err
	}
	if err := h.applyGroupStats(groupID, groupStats); err != nil {
		return err
	}

	// Mark everything as used, because stats were rebuilt from all messages.
	if err := h.DB.Model(&postgres.Message{}).
		Where("chat_id = ?", groupID).
		Update("used_for_stats", true).Error; err != nil {
		return err
	}

	return nil
}
