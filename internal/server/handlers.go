package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"imget/internal/db"
	"imget/internal/imgpipe"
	"imget/internal/mediatype"
	"imget/internal/source"
)

// keywordPattern accepts ASCII letters/digits, Chinese (Han), spaces, dash and
// underscore. Everything else (punctuation, emojis, URL-encoded junk) is
// rejected so we don't feed weird strings to Pexels/Pixabay queries.
var keywordPattern = regexp.MustCompile(`^[\p{L}\p{N} _\-\p{Han}]+$`)

// sanitizeKeyword returns a safe, trimmed keyword or "" when the input is
// empty, too long, or contains unsupported characters. Cap is 40 runes so a
// single query stays a reasonable cache key and doesn't bloat request_profiles.
func sanitizeKeyword(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if utf8.RuneCountInString(s) > 40 {
		return ""
	}
	if !keywordPattern.MatchString(s) {
		return ""
	}
	return s
}

// ============================================================
// home (/)
// ============================================================

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	data := s.localizedHomeData(r, "zh")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")
	if err := s.templates.render(w, "home_i18n.html.tmpl", data); err != nil {
		s.deps.Logger.Error("home render", "err", err)
	}
}

func (s *Server) localizedHomeData(r *http.Request, code string) map[string]any {
	summary := s.deps.Pipeline.LibrarySummary()
	locale, _ := localizedHomeFor(code)
	locale = localizeText(locale, s.site)
	canonicalURL := s.site.BaseURL + "/" + locale.Code + "/"
	if locale.Code == "zh" {
		canonicalURL = s.site.BaseURL + "/"
	}
	return map[string]any{
		"Site":         s.site,
		"Locale":       locale,
		"Languages":    languageLinks(locale.Code, s.site),
		"CanonicalURL": canonicalURL,
		"HomeURL":      canonicalURL,
		"Copyright":    localizedCopyrightDetails(locale.Code, s.site),
		"TotalCount":   summary.TotalImages,
		"TotalBytes":   summary.TotalBytes,
	}
}

func (s *Server) handleLocalizedHome(w http.ResponseWriter, r *http.Request, code string) {
	locale, ok := localizedHomeFor(code)
	if !ok || code == "zh" {
		http.Redirect(w, r, s.deps.Cfg.SiteBaseURL+"/", http.StatusMovedPermanently)
		return
	}
	if r.URL.Path != "/"+locale.Code+"/" {
		http.Redirect(w, r, s.deps.Cfg.SiteBaseURL+"/"+locale.Code+"/", http.StatusMovedPermanently)
		return
	}

	data := s.localizedHomeData(r, locale.Code)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")
	if err := s.templates.render(w, "home_i18n.html.tmpl", data); err != nil {
		s.deps.Logger.Error("localized home render", "code", code, "err", err)
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
	keyword := sanitizeKeyword(firstValue(q, "keyword", "q"))
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

	// Metrics beacon: the detail page JS POSTs ?__track=1&event=download here
	// (see main.min.js → trackMetric). Bump the counter and return fresh JSON
	// counts without re-rendering an image. (view is counted on detail render.)
	if q.Get("__track") == "1" {
		s.handleTrackBeacon(w, r, width, height, typ, keyword)
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

	// Prefer redirect to CDN, but ONLY once the upload is confirmed.
	//
	// res.CDNURL is computed eagerly (deterministic CDN_BASE + rel) before the
	// async upload finishes, so redirecting to it on a freshly-rendered variant
	// 404s at the CDN until the upload lands. CDNURLFor reads r2_uploads → it is
	// non-empty only after the upload row exists. This mirrors the guard the
	// /files/* and wallpaper paths already use, and the detail-page preview fix
	// (commit ea1733b) — the raw path was the remaining hole.
	if s.deps.Cfg.R2RedirectDirect {
		if cdn := s.deps.Pipeline.CDNURLFor(r.Context(), res.RelativePath); cdn != "" {
			http.Redirect(w, r, cdn, http.StatusFound)
			return
		}
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

	// localImageURL — always served locally from this render's exact file path,
	// so the detail page preview matches the 直链 / Markdown / BBCode snippets
	// (which advertise res.CDNURL → s3.img.et/<same rel>). Used only for the
	// <img class="viewer-image">, so the preview is immediately available even
	// before the R2 upload finishes, and always shows the same image as the
	// snippets reference (no new random pick on a /raw=1 request).
	localImageURL := s.deps.Cfg.SiteBaseURL + "/files/" + res.RelativePath

	currentMeta := GetFileMeta(res.AbsolutePath)
	currentFormatLabel := FormatImageFormatLabel(currentMeta.Extension)
	if currentFormatLabel == "" {
		currentFormatLabel = FormatImageFormatLabel(res.Format)
	}

	// Profile metrics (view + download counts). Count this detail-page view
	// synchronously: detail pages are no-store and far rarer than raw-image
	// hits, so a single UPDATE here is cheap (the hot raw path stays batched).
	// The profile row is guaranteed to exist — Render() registered it above.
	profileKey := db.ProfileKey(width, height, typ, keyword)
	if err := s.deps.DB.IncrementProfileMetric(r.Context(), profileKey, "view"); err != nil {
		s.deps.Logger.Warn("view increment failed", "err", err)
	}
	var viewCount, downloadCount int64
	if prof, _ := s.deps.DB.GetProfile(r.Context(), profileKey); prof != nil {
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
		"LocalImageURL":      localImageURL,
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
	abs := filepath.Join(s.deps.Cfg.AbsImagesDir(), rel)
	if _, err := os.Stat(abs); err != nil {
		alt := filepath.Join(s.deps.Cfg.AbsImagesDir(), "original", typ, file)
		if _, err := os.Stat(alt); err == nil {
			abs = alt
			rel = filepath.ToSlash(filepath.Join("original", typ, file))
		} else {
			http.NotFound(w, r)
			return
		}
	}

	if w_, h_, format, ok := s.fileTransformRequest(r, abs); ok {
		s.handleFileTransform(w, r, typ, rel, w_, h_, format)
		return
	}

	if s.deps.Cfg.R2RedirectDirect {
		if cdn := s.deps.Pipeline.CDNURLFor(r.Context(), rel); cdn != "" {
			w.Header().Set("Vary", "Accept, Sec-Fetch-Dest, Sec-Fetch-Mode")
			w.Header().Set("Cache-Control", "public, max-age=86400")
			http.Redirect(w, r, cdn, http.StatusFound)
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

func (s *Server) fileTransformRequest(r *http.Request, abs string) (int, int, string, bool) {
	q := r.URL.Query()
	format := firstValue(q, "format", "f")
	widthStr := firstValue(q, "w", "width")
	heightStr := firstValue(q, "h", "height")
	if format == "" && widthStr == "" && heightStr == "" {
		return 0, 0, "", false
	}

	meta := GetFileMeta(abs)
	width, height := meta.Width, meta.Height
	if widthStr != "" {
		if v, err := strconv.Atoi(widthStr); err == nil {
			width = v
		}
	}
	if heightStr != "" {
		if v, err := strconv.Atoi(heightStr); err == nil {
			height = v
		}
	}
	return width, height, format, true
}

func (s *Server) handleFileTransform(w http.ResponseWriter, r *http.Request, typ, sourceRel string, width, height int, format string) {
	// A transform's source original can legitimately be larger than the render
	// ceiling (e.g. a 10000×7499 photo). When the format-proxy converts it to
	// WebP/AVIF at native size, clamp to fit within MaxDim rather than 400 —
	// the full-resolution original is still available via the raw /files link.
	if width > s.deps.Cfg.MaxDim || height > s.deps.Cfg.MaxDim {
		width, height = fitWithin(width, height, s.deps.Cfg.MaxDim)
	}
	if width < s.deps.Cfg.MinDim || height < s.deps.Cfg.MinDim {
		s.renderError(w, r, http.StatusBadRequest, "尺寸超出允许范围",
			"宽高需在 "+strconv.Itoa(s.deps.Cfg.MinDim)+" ~ "+strconv.Itoa(s.deps.Cfg.MaxDim)+" 之间")
		return
	}

	res, err := s.deps.Pipeline.RenderSource(r.Context(), sourceRel, typ, width, height, format)
	if err != nil {
		s.deps.Logger.Warn("file transform failed", "source", sourceRel, "err", err)
		s.renderError(w, r, http.StatusServiceUnavailable, "暂时无法生成图片", err.Error())
		return
	}

	if r.URL.Query().Get("download") == "1" {
		w.Header().Set("Content-Disposition", `attachment; filename="`+filepath.Base(res.RelativePath)+`"`)
	}
	w.Header().Set("Vary", "Accept, Sec-Fetch-Dest, Sec-Fetch-Mode")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	if s.deps.Cfg.R2RedirectDirect {
		if cdn := s.deps.Pipeline.CDNURLFor(r.Context(), res.RelativePath); cdn != "" {
			http.Redirect(w, r, cdn, http.StatusFound)
			return
		}
	}
	w.Header().Set("Content-Type", contentTypeFor(res.Format))
	http.ServeFile(w, r, res.AbsolutePath)
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
		} else {
			http.NotFound(w, r)
			return
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
		pagePath = "/p/files/" + typ + "/" + file
	}
	pageURL := s.deps.Cfg.SiteBaseURL + pagePath

	meta := GetFileMeta(abs)
	transformPath := "/c/" + typ + "/" + file
	transformLinks := s.buildFileTransformRows(transformPath, meta)

	// Mirror the wallpaper page's copy-able rows: direct link, dedicated page,
	// download, then every transform size/format.
	imageFormats := []formatRow{
		{"直链", rawURL},
		{"专属页面", pageURL},
		{"下载", downloadURL},
	}
	imageFormats = append(imageFormats, transformLinks...)

	data := map[string]any{
		"Site":           s.site,
		"Type":           typ,
		"TypeLabel":      TypeChineseLabel(typ),
		"FileName":       filepath.Base(rel),
		"FormatLabel":    FormatImageFormatLabel(meta.Extension),
		"PoolCount":      s.deps.Pipeline.CountOriginalsForType(typ),
		"PageURL":        pageURL,
		"RawURL":         rawURL,
		"DownloadURL":    downloadURL,
		"HomeURL":        s.deps.Cfg.SiteBaseURL + "/",
		"Meta":           meta,
		"TransformLinks": transformLinks,
		"ImageFormats":   imageFormats,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	w.Header().Set("Vary", "Accept, Sec-Fetch-Mode, Sec-Fetch-Dest, Sec-Fetch-User")
	if err := s.templates.render(w, "public-image.html.tmpl", data); err != nil {
		s.deps.Logger.Error("public-image render", "err", err)
	}
}

// handleTrackBeacon records a view/download event for a (w,h,type,keyword)
// profile and replies with the fresh counts as JSON. Called from handleImage
// when ?__track=1 is present. The increment is a single synchronous UPDATE —
// these beacons fire only on user interaction (detail-page download), never on
// the raw-image hot path, so they don't need the in-memory batcher.
func (s *Server) handleTrackBeacon(w http.ResponseWriter, r *http.Request, width, height int, typ, keyword string) {
	key := db.ProfileKey(width, height, typ, keyword)
	switch r.URL.Query().Get("event") {
	case "view", "download":
		if err := s.deps.DB.IncrementProfileMetric(r.Context(), key, r.URL.Query().Get("event")); err != nil {
			s.deps.Logger.Warn("track increment failed", "err", err)
		}
	}

	var viewCount, downloadCount int64
	if prof, _ := s.deps.DB.GetProfile(r.Context(), key); prof != nil {
		viewCount = prof.ViewCount
		downloadCount = prof.DownloadCount
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":             true,
		"view_count":     viewCount,
		"download_count": downloadCount,
	})
}

// handleStats renders /stats — a site-wide rollup of the local library size and
// per-(type × resolution) request demand pulled from request_profiles.
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	gs, err := s.deps.DB.GlobalStats(r.Context(), 20)
	if err != nil {
		s.deps.Logger.Warn("global stats failed", "err", err)
		s.renderError(w, r, http.StatusServiceUnavailable, "统计不可用", "无法读取统计数据，请稍后再试。")
		return
	}
	lib := s.deps.Pipeline.LibrarySummary()

	specs := make([]map[string]any, 0, len(gs.TopSpecs))
	for _, sp := range gs.TopSpecs {
		specs = append(specs, map[string]any{
			"Type":       sp.Type,
			"TypeLabel":  TypeChineseLabel(sp.Type),
			"Resolution": fmt.Sprintf("%d × %d", sp.Width, sp.Height),
			"Keyword":    sp.Keyword,
			"Requests":   sp.Requests,
			"Views":      sp.Views,
		})
	}
	cats := make([]map[string]any, 0, len(gs.TopCategories))
	for _, c := range gs.TopCategories {
		cats = append(cats, map[string]any{
			"Type":      c.Type,
			"TypeLabel": TypeChineseLabel(c.Type),
			"Requests":  c.Requests,
			"Views":     c.Views,
			"Library":   lib.ByType[c.Type],
		})
	}

	// The single most-referenced spec (top of the list), used for the hero stat.
	var topSpec map[string]any
	if len(specs) > 0 {
		topSpec = specs[0]
	}

	// Locale + copyright power the shared homepage footer and info modal so the
	// stats page carries the same header/footer chrome as the home page.
	locale, _ := localizedHomeFor("zh")
	locale = localizeText(locale, s.site)

	data := map[string]any{
		"Site":           s.site,
		"HomeURL":        s.deps.Cfg.SiteBaseURL + "/",
		"Locale":         locale,
		"Copyright":      localizedCopyrightDetails("zh", s.site),
		"TotalRequests":  gs.TotalRequests,
		"TotalViews":     gs.TotalViews,
		"TotalDownloads": gs.TotalDownloads,
		"ProfileCount":   gs.ProfileCount,
		"TotalImages":    lib.TotalImages,
		"TotalCount":     lib.TotalImages,
		"TotalBytes":     lib.TotalBytes,
		"TopSpec":        topSpec,
		"TopSpecs":       specs,
		"TopCategories":  cats,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=120")
	if err := s.templates.render(w, "stats.html.tmpl", data); err != nil {
		s.deps.Logger.Error("stats render", "err", err)
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

func (s *Server) buildFileTransformRows(publicPath string, meta FileMeta) []formatRow {
	if meta.Width <= 0 || meta.Height <= 0 {
		return nil
	}
	base := s.deps.Cfg.SiteBaseURL + publicPath
	rows := make([]formatRow, 0, 6)

	add := func(label string, width, height int, format string) {
		if width < s.deps.Cfg.MinDim || height < s.deps.Cfg.MinDim ||
			width > s.deps.Cfg.MaxDim || height > s.deps.Cfg.MaxDim {
			return
		}
		u, err := url.Parse(base)
		if err != nil {
			return
		}
		q := u.Query()
		q.Set("w", strconv.Itoa(width))
		q.Set("h", strconv.Itoa(height))
		q.Set("format", format)
		u.RawQuery = q.Encode()
		rows = append(rows, formatRow{label, u.String()})
	}

	maxW, maxH := fitWithin(meta.Width, meta.Height, s.deps.Cfg.MaxDim)
	add("最大尺寸 WebP", maxW, maxH, "webp")
	if s.site.SupportsAvif {
		add("最大尺寸 AVIF", maxW, maxH, "avif")
	}
	add("3840 x 2160 WebP", 3840, 2160, "webp")
	if s.site.SupportsAvif {
		add("3840 x 2160 AVIF", 3840, 2160, "avif")
	}
	add("1920 x 1080 WebP", 1920, 1080, "webp")
	if s.site.SupportsAvif {
		add("1920 x 1080 AVIF", 1920, 1080, "avif")
	}
	return rows
}

func fitWithin(width, height, maxDim int) (int, int) {
	if width <= maxDim && height <= maxDim {
		return width, height
	}
	if width >= height {
		return maxDim, height * maxDim / width
	}
	return width * maxDim / height, maxDim
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
	return mediatype.ForExt(formatOrExt)
}
