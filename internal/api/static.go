package api

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// frontend serves static assets and falls back to index.html for SPA routes.
func (r *Router) frontend(w http.ResponseWriter, req *http.Request) {
	cleanPath := path.Clean(strings.TrimPrefix(req.URL.Path, "/"))
	if cleanPath == "." || cleanPath == "/" {
		cleanPath = "index.html"
	}
	if _, err := fs.Stat(r.static, cleanPath); err == nil {
		http.ServeFileFS(w, req, r.static, cleanPath)
		return
	}
	http.ServeFileFS(w, req, r.static, "index.html")
}
