package server

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed static
var embeddedStatic embed.FS

// staticFS returns the embedded /static subtree as a root-level fs.FS so
// callers don't have to worry about the prefix.
func staticFS() fs.FS {
	sub, err := fs.Sub(embeddedStatic, "static")
	if err != nil {
		// embed.FS guarantees the directory exists at compile time.
		panic(err)
	}
	return sub
}

// assetsHandler serves the embedded /assets/* tree with cache headers.
func assetsHandler() http.Handler {
	srv := http.FileServer(http.FS(staticFS()))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=86400, immutable")
		http.StripPrefix("/assets", srv).ServeHTTP(w, r)
	})
}

// faviconHandler serves the embedded favicon.ico.
func faviconHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := embeddedStatic.ReadFile("static/favicon.ico")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/x-icon")
		w.Header().Set("Cache-Control", "public, max-age=604800")
		_, _ = w.Write(data)
	})
}
