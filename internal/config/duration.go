package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// UnmarshalYAML parses human-readable duration values such as "30s", "24h", or "90d".
func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	d.Set = true
	if value.Kind == 0 || value.Value == "" {
		return nil
	}
	parsed, err := parseDuration(value.Value)
	if err != nil {
		return fmt.Errorf("parse duration %q: %w", value.Value, err)
	}
	d.Duration = parsed
	return nil
}

// MarshalYAML writes durations back using Go's standard duration string format.
func (d Duration) MarshalYAML() (any, error) {
	return d.Duration.String(), nil
}

// parseDuration extends Go duration strings with a day unit for config files.
func parseDuration(raw string) (time.Duration, error) {
	raw = strings.TrimSpace(raw)
	parsed, err := time.ParseDuration(raw)
	if err == nil || !strings.Contains(raw, "d") {
		return parsed, err
	}

	sign := ""
	if strings.HasPrefix(raw, "+") || strings.HasPrefix(raw, "-") {
		sign = raw[:1]
		raw = raw[1:]
	}

	var converted strings.Builder
	converted.WriteString(sign)
	for i := 0; i < len(raw); {
		start := i
		for i < len(raw) && ((raw[i] >= '0' && raw[i] <= '9') || raw[i] == '.') {
			i++
		}
		if start == i {
			return 0, err
		}
		number := raw[start:i]
		unitStart := i
		for i < len(raw) && !((raw[i] >= '0' && raw[i] <= '9') || raw[i] == '.') {
			i++
		}
		unit := raw[unitStart:i]
		if unit == "d" {
			days, parseErr := strconv.ParseFloat(number, 64)
			if parseErr != nil {
				return 0, parseErr
			}
			converted.WriteString(strconv.FormatFloat(days*24, 'f', -1, 64))
			converted.WriteString("h")
			continue
		}
		converted.WriteString(number)
		converted.WriteString(unit)
	}
	return time.ParseDuration(converted.String())
}
