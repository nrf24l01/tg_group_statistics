package update

import "sync"

func (h *Handler) Update() error {
	groups, err := h.getAllGroupIds()
	if err != nil {
		return err
	}
	maxWorkers := 4
	if n := len(groups); n == 0 {
		return nil
	} else if n < maxWorkers {
		maxWorkers = n
	}

	sema := make(chan struct{}, maxWorkers)
	var wg sync.WaitGroup
	errCh := make(chan error, len(groups))

	for _, gid := range groups {
		wg.Add(1)
		sema <- struct{}{}
		go func(id int64) {
			defer wg.Done()
			defer func() { <-sema }()
			if err := h.groupFullUpdate(id); err != nil {
				errCh <- err
			}
		}(gid)
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			return err
		}
	}
	return nil
}