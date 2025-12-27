package update

import (
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

func (h *Handler) groupFullUpdate(group_id int64) error {
	messages, err := h.getAllMessagesPerGroup(group_id, false)
	if err != nil {
		return err
	}
	userStats, err := h.loadUsersStats(group_id)
	if err != nil {
		return err
	}
	groupStat, err := h.loadGroupStats(group_id)
	if err != nil {
		return err
	}

	var used_messages []uuid.UUID
	messageWordCounts := make(map[uuid.UUID]map[string]int64)

	for _, msg := range messages {
		sender_id := msg.Sender.TgUserID
		time := removeTime(msg.SendTime)
		dateKey := time.Format("02-01-2006")

		wc := map[string]int64{}
		if msg.MessageText != nil {
			wc = countWords(*msg.MessageText)
		}

		// Update group stats
		userStat := userStats[sender_id]
		if userStat.MessagesPerDay == nil {
			userStat.MessagesPerDay = make(map[string]int)
		}
		if userStat.WordCountsPerDay == nil {
			userStat.WordCountsPerDay = make(map[string]map[string]int64)
		}
		userStat.MessagesPerDay[dateKey]++
		userStat.TotalMessages++
		userStat.WordCountsPerDay[dateKey] = mergeWordCounts(userStat.WordCountsPerDay[dateKey], wc)
		userStats[sender_id] = userStat
	
		// Update overall group stats
		if groupStat.WordCountsPerDay == nil {
			groupStat.WordCountsPerDay = make(map[string]map[string]int64)
		}
		groupStat.MessagesPerDay[dateKey]++
		groupStat.TotalMessages++
		groupStat.WordCountsPerDay[dateKey] = mergeWordCounts(groupStat.WordCountsPerDay[dateKey], wc)

		if len(msg.WordCounts) == 0 {
			messageWordCounts[msg.ID] = wc
		}

		used_messages = append(used_messages, msg.ID)
	}

	if len(messageWordCounts) > 0 {
		wcUpdates := make(map[uuid.UUID]datatypes.JSONMap, len(messageWordCounts))
		for id, wc := range messageWordCounts {
			wcUpdates[id] = wordCountsToJSONMap(wc)
		}
		if err := h.updateMessagesWordCounts(wcUpdates); err != nil {
			return err
		}
	}
	
	if err := h.applyUsersStats(group_id, userStats); err != nil {
		return err
	}
	if err := h.applyGroupStats(group_id, groupStat); err != nil {
		return err
	}
	if err := h.markMessagesAsUsed(used_messages); err != nil {
		return err
	}

	return nil
}