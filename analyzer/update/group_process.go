package update

import "github.com/google/uuid"

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

	for _, msg := range messages {
		sender_id := msg.Sender.TgUserID
		time := removeTime(msg.SendTime)
		dateKey := time.Format("02-01-2006")

		var tokens []string
		if msg.MessageText != nil {
			tokens = tokenizeWords(*msg.MessageText)
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
		if len(tokens) > 0 {
			day := ensureWordDayMap(userStat.WordCountsPerDay, dateKey)
			addWordCounts(day, tokens)
		}
		userStats[sender_id] = userStat
	
		// Update overall group stats
		groupStat.MessagesPerDay[dateKey]++
		groupStat.TotalMessages++
		if groupStat.WordCountsPerDay == nil {
			groupStat.WordCountsPerDay = make(map[string]map[string]int64)
		}
		if len(tokens) > 0 {
			day := ensureWordDayMap(groupStat.WordCountsPerDay, dateKey)
			addWordCounts(day, tokens)
		}

		used_messages = append(used_messages, msg.ID)
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