package update

func (h *Handler) groupFullUpdate(group_id int64) error {
	messages, err := h.getAllMessagesPerGroup(group_id, false)
	if err != nil {
		return err
	}

	for _, msg := range messages {
		
	}

	return nil
}