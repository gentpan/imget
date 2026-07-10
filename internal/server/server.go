// Package server wires HTTP routes to the image pipeline.
package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"imget/internal/config"
	"imget/internal/db"
	"imget/internal/encoder"
	"imget/internal/imgpipe"
	"imget/internal/metrics"
	"imget/internal/source"
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
	case 1:
		// /{lang} — localized home page. Keep this before the type shortcut
		// so language codes remain reserved and do not conflict with image
		// categories.
		if isLocalizedHomeCode(parts[0]) {
			s.handleLocalizedHome(w, r, parts[0])
			return
		}
		// /stats — global library + request statistics page. Reserved before the
		// type shortcut so it can't be shadowed by a category named "stats".
		if parts[0] == "stats" {
			s.handleStats(w, r)
			return
		}
		// /{type} — wallpaper shortcut: 1 segment matching an allowed type
		// returns one original-resolution image (302 to CDN if configured).
		if source.IsAllowedType(parts[0]) {
			s.handleOriginalByType(w, r, parts[0])
			return
		}
	case 2:
		// /type/{type} — explicit namespaced wallpaper form.
		if parts[0] == "type" && source.IsAllowedType(parts[1]) {
			s.handleOriginalByType(w, r, parts[1])
			return
		}
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
		case "c":
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

// handleOriginalByType serves one original-resolution image for the given
// category. Used by the wallpaper-friendly routes /{type} and /type/{type}.
//
// Query params:
//
//	?r=N        — fixed selection: same r maps to same image (cacheable)
//	?keyword=X  — override the default type keyword (free-form search)
//	?raw=1      — force binary image even on browser navigation
//
// Content negotiation:
//   - Browser navigation (Accept: text/html, no raw=1) → render the wallpaper
//     preview page (image + copy-able URLs).
//   - <img src="..."> / Sec-Fetch-Dest=image / ?raw=1 → 302 to R2 CDN, or
//     stream local when CDN isn't configured.
func (s *Server) handleOriginalByType(w http.ResponseWriter, r *http.Request, rawTyp string) {
	typ := source.NormalizeType(rawTyp)
	keyword := sanitizeKeyword(r.URL.Query().Get("keyword"))
	fixed := strings.TrimSpace(r.URL.Query().Get("r"))

	rawRequested := r.URL.Query().Get("raw") == "1" ||
		strings.EqualFold(r.Header.Get("Sec-Fetch-Dest"), "image")
	wantsHTML := strings.Contains(r.Header.Get("Accept"), "text/html") && !rawRequested

	// Category pages show a fresh random original on every visit (the page is
	// rendered no-store). A fixed image is opt-in via ?r=N; a specific keyword
	// narrows the pool. PickOriginal picks randomly when both are empty.
	rel, err := s.deps.Pipeline.PickOriginal(r.Context(), typ, keyword, fixed)
	if err != nil || rel == "" {
		s.deps.Logger.Warn("wallpaper pick failed", "type", typ, "err", err)
		http.Error(w, "no source available", http.StatusServiceUnavailable)
		return
	}

	if wantsHTML {
		s.renderWallpaperDetail(w, r, rel, typ, keyword, fixed)
		return
	}

	// Cache-Control: fixed r = stable, randoms get a short TTL so each fresh
	// hit through a CDN actually rotates.
	if fixed != "" {
		w.Header().Set("Cache-Control", "public, max-age=86400, immutable")
	} else {
		w.Header().Set("Cache-Control", "public, max-age=300")
	}

	if s.deps.Cfg.R2RedirectDirect {
		if cdn := s.deps.Pipeline.CDNURLFor(r.Context(), rel); cdn != "" {
			http.Redirect(w, r, cdn, http.StatusFound)
			return
		}
	}

	abs, err := s.deps.Pipeline.EnsureSourceLocal(r.Context(), rel)
	if err != nil {
		s.deps.Logger.Warn("wallpaper local-ensure failed", "rel", rel, "err", err)
		http.Error(w, "source missing", http.StatusServiceUnavailable)
		return
	}
	http.ServeFile(w, r, abs)
}

// renderWallpaperDetail renders the HTML preview page for a wallpaper-mode
// URL — what the browser sees when navigating to /landscape directly.
func (s *Server) renderWallpaperDetail(w http.ResponseWriter, r *http.Request, rel, typ, keyword, fixed string) {
	var meta FileMeta
	if abs, err := s.deps.Pipeline.EnsureSourceLocal(r.Context(), rel); err == nil {
		meta = GetFileMeta(abs)
	}

	cdn := s.deps.Pipeline.CDNURLFor(r.Context(), rel)
	rawURL := cdn
	if rawURL == "" {
		// No CDN (R2 disabled): serve the original locally. `rel` is
		// "original/{type}/{file}", but the local file route is
		// /files/{type}/{file} (handleFileDirect resolves the original/ dir),
		// so build that form rather than "/"+rel, which has no route and 404s.
		rawURL = s.deps.Cfg.SiteBaseURL + "/files/" + typ + "/" + filepath.Base(rel)
	}

	shortURL := s.deps.Cfg.SiteBaseURL + "/" + typ
	hasQuery := false
	if fixed != "" {
		shortURL += "?r=" + url.QueryEscape(fixed)
		hasQuery = true
	}
	if keyword != "" {
		if hasQuery {
			shortURL += "&keyword=" + url.QueryEscape(keyword)
		} else {
			shortURL += "?keyword=" + url.QueryEscape(keyword)
			hasQuery = true
		}
	}

	srcLabel, srcURL := s.imageSourceFor(r.Context(), rel)

	filename := filepath.Base(rel)
	downloadURL := s.deps.Cfg.SiteBaseURL + "/files/" + typ + "/" + filename + "?download=1"
	transformPath := "/c/" + typ + "/" + filename
	dedicatedPageURL := s.deps.Cfg.SiteBaseURL + "/p/files/" + typ + "/" + filename

	formats := []formatRow{
		{"短链", shortURL},
		{"直链", rawURL},
		{"专属页面", dedicatedPageURL},
	}
	formats = append(formats, s.buildFileTransformRows(transformPath, meta)...)

	data := map[string]any{
		"Site":         s.site,
		"Type":         typ,
		"TypeLabel":    TypeChineseLabel(typ),
		"FormatLabel":  FormatImageFormatLabel(meta.Extension),
		"Keyword":      keyword,
		"FixedR":       fixed,
		"ShortURL":     shortURL,
		"HasQuery":     hasQuery,
		"RawURL":       rawURL,
		"PageURL":      dedicatedPageURL,
		"DownloadURL":  downloadURL,
		"Meta":         meta,
		"SourceLabel":  srcLabel,
		"SourceURL":    srcURL,
		"PoolCount":    s.deps.Pipeline.CountOriginalsForType(typ),
		"HomeURL":      s.deps.Cfg.SiteBaseURL + "/",
		"ImageFormats": formats,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	if err := s.templates.render(w, "wallpaper.html.tmpl", data); err != nil {
		s.deps.Logger.Error("wallpaper render", "err", err)
	}
}
