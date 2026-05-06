// Package server wires HTTP routes to the image pipeline.
package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"imget/internal/config"
	"imget/internal/db"
	"imget/internal/encoder"
	"imget/internal/imgpipe"
	"imget/internal/metrics"
)

// Deps bundles everything a Server needs.
type Deps struct {
	Cfg      *config.Config
	DB       *db.DB
	Pipeline *imgpipe.Pipeline
	Logger   *slog.Logger
}

type Server struct {
	deps      Deps
	templates *templates
	site      SiteContext
}

func New(deps Deps) (*Server, error) {
	if deps.Cfg == nil || deps.DB == nil || deps.Pipeline == nil {
		return nil, errors.New("server: missing deps")
	}
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	tmpls, err := loadTemplates(deps.Cfg.AbsTemplatesDir())
	if err != nil {
		return nil, err
	}
	// Build the SiteContext once at startup so every render shares the same
	// (cheap-to-copy) struct.
	enc, _ := encoder.New(encoder.Options{})
	site := newSiteContext(deps.Cfg, enc)
	return &Server{deps: deps, templates: tmpls, site: site}, nil
}

// Handler returns the http.Handler for the configured routes. Static
// assets (/assets/*, /favicon.ico) are served from the binary's
// embedded FS — no on-disk dependency.
//
// /metrics exposes Prometheus metrics — restrict access at the nginx layer
// if you don't want them public.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/assets/", assetsHandler())
	mux.Handle("/favicon.ico", faviconHandler())
	mux.Handle("/metrics", metrics.Handler())
	mux.HandleFunc("/", s.route)
	return s.middleware(mux)
}

// route dispatches paths whose shape we can't enumerate at registration time.
func (s *Server) route(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	switch {
	case path == "/" || path == "":
		s.handleHome(w, r)
		return
	case path == "/healthz":
		s.handleHealth(w, r)
		return
	}

	parts := splitPath(path)

	switch len(parts) {
	case 2:
		// /{w}/{h}  OR  /{type}/{file.ext}
		if hasImageExt(parts[1]) {
			s.handleFileDirect(w, r, parts[0], parts[1])
			return
		}
		if w_, h_, ok := parseDimsPair(parts[0], parts[1]); ok {
			s.handleImage(w, r, w_, h_)
			return
		}
	case 3:
		// /files/{type}/{file}    /p/{type}/{file}
		switch parts[0] {
		case "files":
			s.handleFileDirect(w, r, parts[1], parts[2])
			return
		case "p":
			s.handleFileDetail(w, r, parts[1], parts[2])
			return
		}
	case 4:
		// /p/files/{type}/{file}
		if parts[0] == "p" && parts[1] == "files" {
			s.handleFileDetail(w, r, parts[2], parts[3])
			return
		}
	}

	http.NotFound(w, r)
}

func (s *Server) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		defer func() {
			if rec := recover(); rec != nil {
				s.deps.Logger.Error("panic", "panic", rec, "path", r.URL.Path)
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
		s.deps.Logger.Debug("http",
			"method", r.Method,
			"path", r.URL.Path,
			"dur_ms", time.Since(start).Milliseconds())
	})
}

// ListenAndServe runs the server until ctx is cancelled.
func (s *Server) ListenAndServe(ctx context.Context, addr string) error {
	srv := &http.Server{
		Addr:    addr,
		Handler: s.Handler(),
		// Slow-loris / slow-write protections. The render path can take
		// up to a few seconds under cold cache (Pixabay fetch + libvips
		// encode), so WriteTimeout is generous; tighten if you front
		// this with a CDN that can absorb retries.
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1 MiB
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
	s.deps.Logger.Info("listening", "addr", addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// ============================================================
// helpers
// ============================================================

func splitPath(p string) []string {
	p = strings.Trim(p, "/")
	if p == "" {
		return nil
	}
	return strings.Split(p, "/")
}

func parseDimsPair(a, b string) (int, int, bool) {
	w, errA := strconv.Atoi(a)
	h, errB := strconv.Atoi(b)
	if errA != nil || errB != nil {
		return 0, 0, false
	}
	return w, h, true
}

func hasImageExt(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".avif":
		return true
	}
	return false
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if err := s.deps.DB.PingContext(r.Context()); err != nil {
		http.Error(w, "db: "+err.Error(), http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	_, _ = w.Write([]byte("ok"))
}
