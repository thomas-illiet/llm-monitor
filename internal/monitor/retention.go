package monitor

import (
	"context"
	"time"
)

// RunHistoryRetention prunes persisted history outside the configured retention window.
func (s *Scheduler) RunHistoryRetention(ctx context.Context) error {
	history := s.cfg.Retention.History.Duration
	if history <= 0 || s.store == nil {
		return nil
	}
	cutoff := time.Now().UTC().Add(-history)
	if err := s.store.PruneHistoryBefore(ctx, cutoff); err != nil {
		s.logger.Error("prune history", "error", err, "cutoff", cutoff)
		return err
	}
	return nil
}
