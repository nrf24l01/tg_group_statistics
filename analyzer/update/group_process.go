package update

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

	for _, msg := range messages {
		sender_id := msg.Sender.TgUserID
		time := removeTime(msg.SendTime)
		dateKey := time.Format("02-01-2006")

		// Update group stats
		userStat := userStats[sender_id]
		userStat.MessagesPerDay[dateKey]++
		userStat.TotalMessages++
		userStats[sender_id] = userStat
	
		// Update overall group stats
		groupStat.MessagesPerDay[dateKey]++
		groupStat.TotalMessages++
	}
	
	if err := h.applyUsersStats(group_id, userStats); err != nil {
		return err
	}
	if err := h.applyGroupStats(group_id, groupStat); err != nil {
		return err
	}

	return nil
}