package config

import "time"

// EnabledValue reports whether retry behavior should be used.
func (r RetryConfig) EnabledValue() bool {
	if r.Enabled != nil && !*r.Enabled {
		return false
	}
	return r.MaxRetriesValue() > 0
}

// MaxRetriesValue returns the configured retry count, defaulting to a light profile.
func (r RetryConfig) MaxRetriesValue() int {
	if r.MaxRetries == nil {
		return 2
	}
	return *r.MaxRetries
}

// WaitMinValue returns the lower retry backoff bound.
func (r RetryConfig) WaitMinValue() time.Duration {
	if r.WaitMin.Duration == 0 {
		return 500 * time.Millisecond
	}
	return r.WaitMin.Duration
}

// WaitMaxValue returns the upper retry backoff bound.
func (r RetryConfig) WaitMaxValue() time.Duration {
	if r.WaitMax.Duration == 0 {
		return 5 * time.Second
	}
	return r.WaitMax.Duration
}
