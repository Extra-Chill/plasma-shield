// Package web provides the embedded web UI for Plasma Shield.
package web

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed static
var staticFiles embed.FS

// Handler returns an http.Handler that serves the embedded web UI.
func Handler() http.Handler {
	// Strip the "static" prefix from the embedded filesystem
	sub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic(err)
	}
	return http.FileServer(http.FS(sub))
}
