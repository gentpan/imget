package server

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"imget/internal/db"
	"imget/internal/imgpipe"
	"imget/internal/source"
)

// ============================================================
// home (/)
// ============================================================

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	summary, err := s.deps.DB.LibrarySummary(r.Context())
	if err != nil {
		s.deps.Logger.Warn("library summary failed", "err", err)
	}
	data := map[string]any{
		"Site":       s.site,
		"TotalCount": summary.TotalImages,
		"TotalBytes": summary.TotalBytes,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")
	if err := s.templates.render(w, "home.html.tmpl", data); err != nil {
		s.deps.Logger.Error("home render", "err", err)
	}
}

// ============================================================
// /{w}/{h}  — main image route
// ============================================================

func (s *Server) handleImage(w http.ResponseWriter, r *http.Request, width, height int) {
	q := r.URL.Query()

	rawType := firstValue(q, "type", "t")
	typ := source.NormalizeType(rawType)
	if rawType == "" {
		typ = "banner"
	}
	keyword := firstValue(q, "keyword", "q")
	format := firstValue(q, "format", "f")
	rValue := firstValue(q, "r", "v")
	sValue := firstValue(q, "s", "slot", "slot_id")
	freshStr := firstValue(q, "fresh")
	fresh, _ := strconv.Atoi(freshStr)
	if fresh > 1000 {
		fresh = 1000
	}
	if fresh < 0 {
		fresh = 0
	}

	rawRequested := q.Get("raw") == "1" ||
		strings.EqualFold(r.Header.Get("Sec-Fetch-Dest"), "image")
	wantsHTML := q.Get("preview") == "1" ||
		q.Get("page") == "1" ||
		q.Get("detail") == "1" ||
		strings.Contains(r.Header.Get("Accept"), "text/html")
	wantsHTML = wantsHTML && !rawRequested

	if width < s.deps.Cfg.MinDim || height < s.deps.Cfg.MinDim ||
		width > s.deps.Cfg.MaxDim || height > s.deps.Cfg.MaxDim {
		s.renderError(w, r, http.StatusBadRequest, "尺寸超出允许范围",
			"宽高需在 "+strconv.Itoa(s.deps.Cfg.MinDim)+" ~ "+strconv.Itoa(s.deps.Cfg.MaxDim)+" 之间")
		return
	}

	// Preparing page: HTML request + no source for type yet → kick off async fetch + show spinner.
	if wantsHTML && !s.deps.Pipeline.HasAnyOriginal(r.Context(), typ) {
		initial := s.deps.Cfg.InitialPrefetchCount
		if initial <= 0 {
			initial = 5
		}
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			_, _ = s.deps.Pipeline.FetchToLocal(ctx, imgpipe.FetchRequest{
				Type: typ, Keyword: keyword, Width: width, Height: height, Count: initial,
			})
		}()
		s.renderPreparing(w, r, width, height, typ, keyword)
		return
	}

	res, err := s.deps.Pipeline.Render(r.Context(), imgpipe.RenderRequest{
		Width:   width,
		Height:  height,
		Type:    typ,
		Keyword: keyword,
		Format:  format,
		Variant: rValue,
		Slot:    sValue,
		Fresh:   fresh,
	})
	if err != nil {
		s.deps.Logger.Warn("render failed", "err", err)
		s.renderError(w, r, http.StatusServiceUnavailable, "暂时无法生成图片", err.Error())
		return
	}

	if wantsHTML {
		s.renderImageDetail(w, r, res, width, height, typ, keyword, rValue, sValue)
		return
	}

	// Raw image delivery.
	if q.Get("download") == "1" {
		w.Header().Set("Content-Disposition",
			`attachment; filename="`+filepath.Base(res.RelativePath)+`"`)
	}

	// Vary on Accept + Sec-Fetch-Dest so browsers/CDNs don't reuse this
	// image response for a later navigation request to the same URL
	// (which expects the HTML detail page).
	w.Header().Set("Vary", "Accept, Sec-Fetch-Dest, Sec-Fetch-Mode")
	w.Header().Set("Cache-Control", "public, max-age=86400")

	// Prefer redirect to CDN if configured.
	if s.deps.Cfg.R2RedirectDirect && res.CDNURL != "" {
		http.Redirect(w, r, res.CDNURL, http.StatusFound)
		return
	}

	w.Header().Set("Content-Type", contentTypeFor(res.Format))
	http.ServeFile(w, r, res.AbsolutePath)
}

// formatRow is one row in the detail page's "引用链接" list.
type formatRow struct {
	Label string
	Value string
}

func (s *Server) renderImageDetail(
	w http.ResponseWriter, r *http.Request,
	res *imgpipe.RenderResult,
	width, height int, typ, keyword, rVal, sVal string,
) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	w.Header().Set("Vary", "Accept, Sec-Fetch-Mode, Sec-Fetch-Dest, Sec-Fetch-User")

	pageURL := s.buildPageURL(r, "raw", "download")
	rawImageURL := s.buildPageURLWith(r, map[string]string{"raw": "1"}, "download")
	downloadURL := s.buildPageURLWith(r, map[string]string{"download": "1"}, "raw")
	trackBaseURL := s.buildPageURLWith(r, map[string]string{"__track": "1"}, "raw", "download")

	directImageURL := res.CDNURL
	if directImageURL == "" {
		directImageURL = s.deps.Cfg.SiteBaseURL + "/files/" + res.RelativePath
	}

	currentMeta := GetFileMeta(res.AbsolutePath)
	currentFormatLabel := FormatImageFormatLabel(currentMeta.Extension)
	if currentFormatLabel == "" {
		currentFormatLabel = FormatImageFormatLabel(res.Format)
	}

	// Profile metrics (view + download counts).
	var viewCount, downloadCount int64
	if prof, _ := s.deps.DB.GetProfile(r.Context(), db.ProfileKey(width, height, typ, keyword)); prof != nil {
		viewCount = prof.ViewCount
		downloadCount = prof.DownloadCount
	}

	generatedAt := time.Now().Unix()
	if fi, err := os.Stat(res.AbsolutePath); err == nil {
		generatedAt = fi.ModTime().Unix()
	}

	sourceLibraryCount := s.deps.Pipeline.CountOriginalsForType(typ)
	profileRenderCount := s.deps.Pipeline.CountRenderedForProfile(typ, width, height)

	// Build the 8 image format rows mirrored from PHP.
	imageFormats := []formatRow{
		{"短链", pageURL},
		{"直链", directImageURL},
		{"HTML", `<img src="` + directImageURL + `" alt="` + typ + `">`},
		{"HTML w/ Link", `<a href="` + pageURL + `"><img src="` + pageURL + `" alt="` + typ + `"></a>`},
		{"Markdown", `![` + typ + `](` + directImageURL + `)`},
		{"BBCode", `[img]` + directImageURL + `[/img]`},
		{"BBCode w/ Link", `[url=` + pageURL + `][img]` + pageURL + `[/img][/url]`},
		{"URL", pageURL},
	}

	data := map[string]any{
		"Site":               s.site,
		"Type":               typ,
		"TypeLabel":          TypeChineseLabel(typ),
		"Width":              width,
		"Height":             height,
		"Keyword":            keyword,
		"Format":             res.Format,
		"FormatLabel":        currentFormatLabel,
		"CurrentMeta":        currentMeta,
		"GeneratedAt":        generatedAt,
		"ViewCount":          viewCount,
		"DownloadCount":      downloadCount,
		"SourceLibraryCount": sourceLibraryCount,
		"ProfileRenderCount": profileRenderCount,
		"DirectImageURL":     directImageURL,
		"DownloadURL":        downloadURL,
		"PageURL":            pageURL,
		"RawImageURL":        rawImageURL,
		"TrackBaseURL":       trackBaseURL,
		"HomeURL":            s.deps.Cfg.SiteBaseURL + "/",
		"RValue":             rVal,
		"SValue":             sVal,
		"ImageFormats":       imageFormats,
	}
	if err := s.templates.render(w, "detail.html.tmpl", data); err != nil {
		s.deps.Logger.Error("detail render", "err", err)
	}
}

func (s *Server) renderPreparing(w http.ResponseWriter, r *http.Request, width, height int, typ, keyword string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")

	pageURL := s.buildPageURL(r, "raw", "download")

	data := map[string]any{
		"Site":               s.site,
		"Type":               typ,
		"TypeLabel":          TypeChineseLabel(typ),
		"Width":              width,
		"Height":             height,
		"Keyword":            keyword,
		"PageURL":            pageURL,
		"HomeURL":            s.deps.Cfg.SiteBaseURL + "/",
		"SourceLibraryCount": s.deps.Pipeline.CountOriginalsForType(typ),
	}
	if err := s.templates.render(w, "preparing.html.tmpl", data); err != nil {
		s.deps.Logger.Error("preparing render", "err", err)
	}
}

// ============================================================
// /files/{type}/{file}  — direct file delivery
// ============================================================

func (s *Server) handleFileDirect(w http.ResponseWriter, r *http.Request, typ, file string) {
	rel := filepath.ToSlash(filepath.Join(typ, file))

	if s.deps.Cfg.R2RedirectDirect {
		if cdn := s.deps.Pipeline.CDNURLFor(r.Context(), rel); cdn != "" {
			w.Header().Set("Vary", "Accept, Sec-Fetch-Dest, Sec-Fetch-Mode")
			w.Header().Set("Cache-Control", "public, max-age=86400")
			http.Redirect(w, r, cdn, http.StatusFound)
			return
		}
	}

	abs := filepath.Join(s.deps.Cfg.AbsImagesDir(), rel)
	if _, err := os.Stat(abs); err != nil {
		alt := filepath.Join(s.deps.Cfg.AbsImagesDir(), "original", typ, file)
		if _, err := os.Stat(alt); err == nil {
			abs = alt
		} else {
			http.NotFound(w, r)
			return
		}
	}
	w.Header().Set("Vary", "Accept, Sec-Fetch-Dest, Sec-Fetch-Mode")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Header().Set("Content-Type", contentTypeFor(strings.TrimPrefix(filepath.Ext(file), ".")))
	if r.URL.Query().Get("download") == "1" {
		w.Header().Set("Content-Disposition", `attachment; filename="`+file+`"`)
	}
	http.ServeFile(w, r, abs)
}

// ============================================================
// /p/{type}/{file}  — public detail page for an existing file
// ============================================================

func (s *Server) handleFileDetail(w http.ResponseWriter, r *http.Request, typ, file string) {
	rel := filepath.ToSlash(filepath.Join(typ, file))
	abs := filepath.Join(s.deps.Cfg.AbsImagesDir(), rel)
	publicPath := "/" + rel
	mode := ""

	// Detect /p/files/... vs /p/... routing — handler always receives "files" via routing.
	if strings.HasPrefix(r.URL.Path, "/p/files/") {
		publicPath = "/files/" + rel
		mode = "file"
	}

	if _, err := os.Stat(abs); err != nil {
		alt := filepath.Join(s.deps.Cfg.AbsImagesDir(), "original", typ, file)
		if _, err := os.Stat(alt); err == nil {
			abs = alt
			rel = filepath.ToSlash(filepath.Join("original", typ, file))
		}
	}

	cdn := s.deps.Pipeline.CDNURLFor(r.Context(), rel)
	rawURL := cdn
	if rawURL == "" {
		rawURL = s.deps.Cfg.SiteBaseURL + publicPath
	}
	downloadURL := s.deps.Cfg.SiteBaseURL + appendQuery(publicPath, "download", "1")

	pagePath := "/p/" + rel
	if mode == "file" {
		pagePath = "/p/files/" + rel
	}
	pageURL := s.deps.Cfg.SiteBaseURL + pagePath

	meta := GetFileMeta(abs)

	data := map[string]any{
		"Site":        s.site,
		"Type":        typ,
		"FileName":    filepath.Base(rel),
		"PageURL":     pageURL,
		"RawURL":      rawURL,
		"DownloadURL": downloadURL,
		"HomeURL":     s.deps.Cfg.SiteBaseURL + "/",
		"Meta":        meta,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	w.Header().Set("Vary", "Accept, Sec-Fetch-Mode, Sec-Fetch-Dest, Sec-Fetch-User")
	if err := s.templates.render(w, "public-image.html.tmpl", data); err != nil {
		s.deps.Logger.Error("public-image render", "err", err)
	}
}

// ============================================================
// shared helpers
// ============================================================

func (s *Server) renderError(w http.ResponseWriter, r *http.Request, status int, title, detail string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	w.WriteHeader(status)
	data := map[string]any{
		"Site":     s.site,
		"Status":   status,
		"Title":    title,
		"Detail":   detail,
		"HomeURL":  s.deps.Cfg.SiteBaseURL + "/",
		"RetryURL": r.URL.RequestURI(),
	}
	if err := s.templates.render(w, "error.html.tmpl", data); err != nil {
		_, _ = w.Write([]byte(title + ": " + detail))
	}
}

// buildPageURL returns the page URL with the supplied keys removed from the query.
func (s *Server) buildPageURL(r *http.Request, omitKeys ...string) string {
	return s.buildPageURLWith(r, nil, omitKeys...)
}

// buildPageURLWith returns the URL of the current page with `add` keys merged in
// and `omitKeys` removed.
func (s *Server) buildPageURLWith(r *http.Request, add map[string]string, omitKeys ...string) string {
	q := r.URL.Query()
	for _, k := range omitKeys {
		q.Del(k)
	}
	for k, v := range add {
		q.Set(k, v)
	}
	out := s.deps.Cfg.SiteBaseURL + r.URL.Path
	if encoded := q.Encode(); encoded != "" {
		out += "?" + encoded
	}
	return out
}

func appendQuery(path, key, value string) string {
	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	return path + sep + url.QueryEscape(key) + "=" + url.QueryEscape(value)
}

func firstValue(q map[string][]string, keys ...string) string {
	for _, k := range keys {
		if vs, ok := q[k]; ok {
			for _, v := range vs {
				if v = strings.TrimSpace(v); v != "" {
					return v
				}
			}
		}
	}
	return ""
}

func contentTypeFor(formatOrExt string) string {
	switch strings.ToLower(strings.TrimPrefix(formatOrExt, ".")) {
	case "webp":
		return "image/webp"
	case "avif":
		return "image/avif"
	case "jpg", "jpeg":
		return "image/jpeg"
	case "png":
		return "image/png"
	case "gif":
		return "image/gif"
	}
	return "application/octet-stream"
}

