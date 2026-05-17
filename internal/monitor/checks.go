package monitor

import (
	"context"

	"llmservicemonitor/internal/store"
)

// runHTTPCheck probes target reachability and records the result.
func (s *Scheduler) RunHTTPCheck(ctx context.Context) error {
	result := s.client.HealthCheck(ctx)
	record := store.CheckRecord{
		At:         result.CheckedAt,
		OK:         result.OK,
		StatusCode: result.StatusCode,
		LatencyMS:  ms(result.Latency),
		Error:      result.Error,
	}
	if err := s.store.RecordHTTPCheck(ctx, record); err != nil {
		s.logger.Error("record http check", "error", err)
		return err
	}
	return nil
}

// runAuthCheck checks token acquisition and stores OAuth service health.
func (s *Scheduler) RunAuthCheck(ctx context.Context) error {
	result := s.auth.Check(ctx)
	record := store.CheckRecord{
		At:         result.CheckedAt,
		OK:         result.OK,
		StatusCode: result.StatusCode,
		LatencyMS:  ms(result.Latency),
		ExpiresAt:  result.ExpiresAt,
		Error:      result.Error,
	}
	if err := s.store.RecordAuthCheck(ctx, record); err != nil {
		s.logger.Error("record auth check", "error", err)
		return err
	}
	return nil
}
