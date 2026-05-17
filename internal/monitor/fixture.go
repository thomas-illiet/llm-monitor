package monitor

import "os"

// loadEmbeddingFixture reads and truncates the fixture used for embedding probes.
func (s *Scheduler) loadEmbeddingFixture() string {
	path := s.cfg.Tests.EmbeddingFixture.Path
	if path == "" {
		return "This is a small fixture used to monitor embedding model availability and latency."
	}
	data, err := os.ReadFile(path)
	if err != nil {
		s.logger.Error("read embedding fixture", "error", err)
		return ""
	}
	if maxBytes := s.cfg.Tests.EmbeddingFixture.MaxBytes; maxBytes > 0 && len(data) > maxBytes {
		data = data[:maxBytes]
	}
	return string(data)
}
