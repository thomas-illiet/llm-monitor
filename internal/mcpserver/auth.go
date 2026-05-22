package mcpserver

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

func bearerAuth(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		value := strings.TrimSpace(req.Header.Get("Authorization"))
		const prefix = "Bearer "
		if !strings.HasPrefix(value, prefix) {
			w.Header().Set("WWW-Authenticate", `Bearer realm="llm-monitor-mcp"`)
			http.Error(w, "missing bearer token", http.StatusUnauthorized)
			return
		}
		if subtle.ConstantTimeCompare([]byte(strings.TrimSpace(strings.TrimPrefix(value, prefix))), []byte(token)) != 1 {
			w.Header().Set("WWW-Authenticate", `Bearer realm="llm-monitor-mcp"`)
			http.Error(w, "invalid bearer token", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, req)
	})
}
