package update

func (h *Handler) Update() error {
	groups, err := h.getAllGroupIds()
	if err != nil {
		return err
	}
}