package store

import (
	"encoding/base64"
	"fmt"
)

// ModelKey returns a URL-safe stable key for a raw provider model ID.
func ModelKey(modelID string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(modelID))
}

// ModelIDFromKey decodes a URL-safe model key back to the provider model ID.
func ModelIDFromKey(modelKey string) (string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(modelKey)
	if err != nil {
		return "", fmt.Errorf("invalid model_key")
	}
	return string(raw), nil
}

// ModelIdentityKey returns a compact internal map key for one provider model.
func ModelIdentityKey(providerID, modelID string) string {
	return providerID + "\x00" + modelID
}
