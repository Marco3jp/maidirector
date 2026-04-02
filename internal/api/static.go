package api

import (
	"io/fs"
	"net/http"
	"strings"
)

type spaHandler struct {
	fs http.Handler
	staticFS fs.FS
}

func newSPAHandler(staticFS fs.FS) *spaHandler {
	return &spaHandler{
		fs:       http.FileServerFS(staticFS),
		staticFS: staticFS,
	}
}

func (h *spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		path = "index.html"
	}

	if _, err := fs.Stat(h.staticFS, path); err != nil {
		r.URL.Path = "/"
	}

	h.fs.ServeHTTP(w, r)
}
