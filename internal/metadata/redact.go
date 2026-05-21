package metadata

import "strings"

const redactedValue = "[redacted]"

// RedactProviderMetadata returns a deep copy with manifestly sensitive fields hidden.
func RedactProviderMetadata(values map[string]any) map[string]any {
	if values == nil {
		return map[string]any{}
	}
	redacted := make(map[string]any, len(values))
	for key, value := range values {
		if IsSensitiveProviderKey(key) {
			redacted[key] = redactedValue
			continue
		}
		redacted[key] = redactValue(value)
	}
	return redacted
}

// IsSensitiveProviderKey reports whether a provider metadata key should be hidden.
func IsSensitiveProviderKey(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	if normalized == "" {
		return false
	}
	compact := strings.NewReplacer("_", "", "-", "", ".", "", " ", "").Replace(normalized)
	return strings.Contains(compact, "apikey") ||
		strings.Contains(compact, "accesstoken") ||
		strings.Contains(compact, "refreshtoken") ||
		strings.Contains(normalized, "secret") ||
		strings.Contains(normalized, "password") ||
		strings.Contains(normalized, "authorization") ||
		strings.Contains(normalized, "credential") ||
		strings.Contains(normalized, "bearer")
}

func redactValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return RedactProviderMetadata(typed)
	case []any:
		redacted := make([]any, len(typed))
		for i, item := range typed {
			redacted[i] = redactValue(item)
		}
		return redacted
	default:
		return typed
	}
}
