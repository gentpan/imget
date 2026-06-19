// Package imgpipe is the image pipeline: source selection → render → cache → upload.
package imgpipe

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	mrand "math/rand/v2"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"

	"imget/internal/config"
	"imget/internal/db"
	"imget/internal/encoder"
	"imget/internal/metrics"
	"imget/internal/r2"
	"imget/internal/source"
)

// Pipeline orchestrates image fetching, rendering, caching and R2 uploads.
type Pipeline struct {
	cfg     *config.Config
	db      *db.DB
	enc     *encoder.Encoder
	r2c     *r2.Client // optional
	sources []source.Provider
	log     *slog.Logger
	hc      *http.Client

	uploadCh chan string // file paths (relative to ImagesDir) queued for upload
	wg       sync.WaitGroup

	// bgSem caps concurrent background work that the request goroutine fires
	// off (per-visit refetches, late upload retries). Without this bound, a
	// traffic spike can spawn unbounded goroutines and exhaust memory / fds.
	bgSem chan struct{}

	// renderSem bounds concurrent libvips encodes to ~CPU count. Encoding is
	// CPU-bound; a burst of cold-cache misses would otherwise run unbounded
	// parallel encodes and drive load average past what the box can sustain.
	renderSem chan struct{}

	// renderLocks: per-destination mutex used to prevent thundering-herd
	// when multiple requests arrive for the same yet-to-be-rendered file.
	// Bounded LRU prevents long-running services from leaking ~50 bytes
	// per unique (w,h,type,format,source) combo forever.
	renderLocks   *lru.Cache[string, *sync.Mutex]
	renderLocksMu sync.Mutex // guard cache.Get-or-Add against race on first miss

	// urlCacheLRU shadows the url_cache SQLite table for hot variants
	// (?r=... requests). Hit rate is typically >95%; saves a DB round trip
	// on the request hot path.
	urlCacheLRU *lru.Cache[string, string]

	// profileBatcher accumulates request_count / view_count / download_count
	// increments in memory and flushes them to SQLite every few seconds.
	// Cold (first-seen) profiles still hit the DB synchronously so reads
	// can find them.
	profileBatcher *profileBatcher
}

// Options configures Pipeline construction.
type Options struct {
	Config    *config.Config
	DB        *db.DB
	Encoder   *encoder.Encoder
	R2        *r2.Client        // nil when R2 not enabled
	Providers []source.Provider // ordered: try first, then fallback
	Logger    *slog.Logger
}

// New constructs a Pipeline and starts background upload workers.
// Call Close to drain workers and stop them.
func New(ctx context.Context, opts Options) (*Pipeline, error) {
	if opts.Config == nil || opts.DB == nil || opts.Encoder == nil {
		return nil, errors.New("imgpipe: config, db and encoder required")
	}
	log := opts.Logger
	if log == nil {
		log = slog.Default()
	}
	p := &Pipeline{
		cfg:     opts.Config,
		db:      opts.DB,
		enc:     opts.Encoder,
		r2c:     opts.R2,
		sources: opts.Providers,
		log:     log,
		hc:      &http.Client{Timeout: 30 * time.Second},
	}

	workers := opts.Config.R2UploadWorkers
	if workers < 1 {
		workers = 2
	}
	p.uploadCh = make(chan string, 256)
	p.bgSem = make(chan struct{}, 32) // hard cap on opportunistic goroutines

	// Cap parallel encodes at CPU count (min 1) so cold-miss bursts can't
	// oversubscribe the cores libvips is already saturating per encode.
	renderConc := runtime.GOMAXPROCS(0)
	if renderConc < 1 {
		renderConc = 1
	}
	p.renderSem = make(chan struct{}, renderConc)

	// Bounded caches — LRU eviction prevents the long-running service from
	// holding state for every unique combination ever seen.
	if locks, err := lru.New[string, *sync.Mutex](10_000); err == nil {
		p.renderLocks = locks
	}
	if uc, err := lru.New[string, string](10_000); err == nil {
		p.urlCacheLRU = uc
	}

	// Profile counter batcher (5s flush). Drained on Close().
	p.profileBatcher = newProfileBatcher(opts.DB, log, 5*time.Second)
	go p.profileBatcher.Run()
	for i := 0; i < workers; i++ {
		p.wg.Add(1)
		go p.uploadWorker(i)
	}
	return p, nil
}

// Close stops the profile-batcher (with one final flush) and the upload
// workers, waiting for in-flight uploads to finish.
func (p *Pipeline) Close() {
	if p.profileBatcher != nil {
		p.profileBatcher.Close()
	}
	close(p.uploadCh)
	p.wg.Wait()
}

// QueueUpload schedules a relative-path file for asynchronous R2 upload.
// Safe to call when R2 is disabled (it just no-ops).
//
// If both the upload channel AND the bounded fallback semaphore are full, the
// upload is dropped on the floor — the daily `r2-sync` cron rescans disk and
// uploads anything missing, so worst-case latency is ~24h.
func (p *Pipeline) QueueUpload(rel string) {
	if p.r2c == nil || rel == "" {
		return
	}
	select {
	case p.uploadCh <- rel:
		metrics.QueueDepth.WithLabelValues("r2_upload").Set(float64(len(p.uploadCh)))
		return
	default:
	}

	// Channel full — try a bounded inline goroutine.
	select {
	case p.bgSem <- struct{}{}:
		go func(r string) {
			defer func() { <-p.bgSem }()
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			if err := p.uploadOne(ctx, r); err != nil {
				p.log.Warn("inline R2 upload failed", "path", r, "err", err)
			}
		}(rel)
	default:
		// Both queue and inline pool are full. Skip; r2-sync cron will
		// pick this file up later.
		p.log.Warn("R2 upload dropped (queue+sem full)", "path", rel)
	}
}

// ======================================================================
// Render orchestration
// ======================================================================

// RenderRequest captures everything needed to produce a single rendered image.
type RenderRequest struct {
	Width   int
	Height  int
	Type    string // raw input, will be normalized
	Keyword string
	Format  string // "webp" or "avif"; empty falls back to config default
	Variant string // ?r= persistent variant ID (optional)
	Slot    string // ?s= multi-image slot ID (optional)
	Fresh   int    // ?fresh=N — refetch N images before selecting
}

// RenderResult describes the final cached artefact.
type RenderResult struct {
	RelativePath string // relative to ImagesDir
	AbsolutePath string // absolute on disk
	Format       string // "webp" or "avif"
	SourceRel    string // relative path of source original used
	CDNURL       string // R2 CDN URL if uploaded already
	IsFallback   bool   // true when a procedural gradient was used
}

// Render produces (or reuses cached) image and returns its path + CDN status.
func (p *Pipeline) Render(ctx context.Context, req RenderRequest) (*RenderResult, error) {
	typ := source.NormalizeType(req.Type)
	w, h := clampDim(req.Width, p.cfg.MinDim, p.cfg.MaxDim), clampDim(req.Height, p.cfg.MinDim, p.cfg.MaxDim)
	keyword := strings.TrimSpace(req.Keyword)

	format, err := p.resolveRenderFormat(req.Format)
	if err != nil {
		return nil, err
	}

	// Register profile (best effort).
	profileKey, err := p.profileBatcher.Register(ctx, w, h, typ, keyword)
	if err != nil {
		p.log.Warn("register profile failed", "err", err)
	}

	// Select / reuse source.
	sourceRel, isFallback, err := p.selectSource(ctx, profileKey, w, h, typ, keyword, req.Variant, req.Slot, req.Fresh)
	if err != nil {
		return nil, err
	}
	if sourceRel == "" {
		return nil, errors.New("no image source available")
	}

	return p.renderFromSource(ctx, sourceRel, typ, w, h, format, isFallback)
}

// RenderSource renders a requested size/format from one exact source original.
func (p *Pipeline) RenderSource(ctx context.Context, sourceRel, typ string, width, height int, format string) (*RenderResult, error) {
	typ = source.NormalizeType(typ)
	w, h := clampDim(width, p.cfg.MinDim, p.cfg.MaxDim), clampDim(height, p.cfg.MinDim, p.cfg.MaxDim)
	resolvedFormat, err := p.resolveRenderFormat(format)
	if err != nil {
		return nil, err
	}
	return p.renderFromSource(ctx, filepath.ToSlash(sourceRel), typ, w, h, resolvedFormat, false)
}

func (p *Pipeline) resolveRenderFormat(requested string) (string, error) {
	format := strings.ToLower(strings.TrimSpace(requested))
	if format == "" {
		format = p.cfg.DefaultFormat
	}
	if format != "webp" && format != "avif" {
		format = "webp"
	}
	// Honor operator's ENABLED_FORMATS allow-list AND encoder availability.
	enabled := p.cfg.FormatEnabled(format) && p.enc.Available(encoder.Format(format))
	if !enabled {
		// Fall back to any other enabled+available format.
		alt := ""
		if format != "webp" && p.cfg.FormatEnabled("webp") && p.enc.Available(encoder.FormatWebP) {
			alt = "webp"
		} else if format != "avif" && p.cfg.FormatEnabled("avif") && p.enc.Available(encoder.FormatAVIF) {
			alt = "avif"
		}
		if alt == "" {
			return "", fmt.Errorf("format %q not enabled or no encoder available", format)
		}
		format = alt
	}

	return format, nil
}

func (p *Pipeline) renderFromSource(ctx context.Context, sourceRel, typ string, w, h int, format string, isFallback bool) (*RenderResult, error) {
	// Compute deterministic render destination.
	renderRel := p.renderDestRel(sourceRel, typ, w, h, format)
	renderAbs := filepath.Join(p.cfg.AbsImagesDir(), renderRel)

	if err := p.ensureRendered(ctx, sourceRel, renderRel, renderAbs, w, h, format); err != nil {
		return nil, err
	}

	// Queue upload of this rendered file.
	p.QueueUpload(renderRel)

	// Compute the CDN URL eagerly. The string is deterministic
	// (CDN_BASE + "/" + rel), so we don't need the upload to finish to
	// know what URL the file will live at. The upload worker is fast
	// (<1s typical) and the user usually doesn't click the copied URL
	// instantly — by the time it's used, the object is in R2.
	//
	// The trade-off: if uploads are failing systematically, every URL
	// here will 404 at the CDN. That's the correct alert signal anyway;
	// silently falling back to /files/* would mask the real problem.
	cdn := ""
	if p.r2c != nil {
		cdn = p.r2c.CDNURL(renderRel)
	}

	return &RenderResult{
		RelativePath: renderRel,
		AbsolutePath: renderAbs,
		Format:       format,
		SourceRel:    sourceRel,
		CDNURL:       cdn,
		IsFallback:   isFallback,
	}, nil
}

func (p *Pipeline) ensureRendered(ctx context.Context, sourceRel, renderRel, renderAbs string, w, h int, format string) error {
	if fi, err := os.Stat(renderAbs); err == nil && fi.Size() > 0 {
		return nil
	}

	mu := p.lockFor(renderAbs)
	mu.Lock()
	defer mu.Unlock()

	// Double-check after acquiring lock.
	if fi, err := os.Stat(renderAbs); err == nil && fi.Size() > 0 {
		return nil
	}

	// Make sure source is on disk (may need to pull from R2).
	sourceAbs, err := p.ensureSourceLocal(ctx, sourceRel)
	if err != nil {
		return fmt.Errorf("source local: %w", err)
	}

	if err := p.renderToFile(ctx, sourceAbs, renderAbs, w, h, format); err != nil {
		return fmt.Errorf("render: %w", err)
	}
	return nil
}

func (p *Pipeline) renderDestRel(sourceRel, typ string, w, h int, format string) string {
	// Hash sourceRel + dimensions so the same source/dimension always maps
	// to the same cache key.
	h1 := sha1.Sum([]byte(fmt.Sprintf("%s|%dx%d|%s", sourceRel, w, h, format)))
	hash := hex.EncodeToString(h1[:])[:16]
	return filepath.ToSlash(filepath.Join(typ, fmt.Sprintf("%dx%d-%s.%s", w, h, hash, format)))
}

func (p *Pipeline) lockFor(key string) *sync.Mutex {
	if mu, ok := p.renderLocks.Get(key); ok {
		return mu
	}
	// Slow path — guarded so two parallel misses for the same key end up
	// sharing one *sync.Mutex (LRU's API doesn't expose Get-or-Compute).
	p.renderLocksMu.Lock()
	defer p.renderLocksMu.Unlock()
	if mu, ok := p.renderLocks.Get(key); ok {
		return mu
	}
	mu := &sync.Mutex{}
	p.renderLocks.Add(key, mu)
	return mu
}

// ======================================================================
// Source selection
// ======================================================================

// selectSource returns the relative path of an original to render, plus a
// flag indicating whether a procedural fallback was used.
func (p *Pipeline) selectSource(
	ctx context.Context,
	profileKey string,
	w, h int,
	typ, keyword, variant, slot string,
	fresh int,
) (string, bool, error) {
	// 0. Honor explicit fresh=N: download new images before selecting.
	if fresh > 0 {
		if _, err := p.FetchToLocal(ctx, FetchRequest{
			Type: typ, Keyword: keyword, Width: w, Height: h, Count: fresh,
		}); err != nil {
			p.log.Warn("fresh fetch failed", "err", err)
		}
	}
	// 0.5 PerVisitRefetch: every HTTP visit nudges the per-(w,h,type) pool
	// by N more, but only if there's a free slot in the bounded pool. Under
	// a traffic spike we'd rather drop the optional refetch than spawn
	// thousands of goroutines that all collide on Pixabay's rate limit.
	if pv := p.cfg.PerVisitRefetch; pv > 0 {
		select {
		case p.bgSem <- struct{}{}:
			go func() {
				defer func() { <-p.bgSem }()
				pctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
				defer cancel()
				_, _ = p.FetchToLocal(pctx, FetchRequest{
					Type: typ, Keyword: keyword, Width: w, Height: h, Count: pv,
				})
			}()
		default:
			// Pool saturated — skip this refetch. Daily topup keeps the
			// pool fresh on a slower cadence.
		}
	}

	// 1. Variant cache: same r=ID always resolves to the same source.
	cacheKey := variantCacheKey(profileKey, variant, slot)
	if cacheKey != "" {
		// Fast path: in-process LRU.
		if rel, ok := p.urlCacheLRU.Get(cacheKey); ok {
			if p.sourceUsable(rel) {
				metrics.CacheHits.WithLabelValues("url_cache").Inc()
				return rel, false, nil
			}
			p.urlCacheLRU.Remove(cacheKey)
			_ = p.db.DeleteURLCacheByPath(ctx, rel)
		}
		metrics.CacheMisses.WithLabelValues("url_cache").Inc()
		// Slow path: SQLite (cold cache or new variant).
		if rel, ok, err := p.db.GetURLCache(ctx, cacheKey); err == nil && ok {
			if p.sourceUsable(rel) {
				p.urlCacheLRU.Add(cacheKey, rel)
				return rel, false, nil
			}
			// Stale entry; forget it.
			_ = p.db.DeleteURLCacheByPath(ctx, rel)
		}
	}

	// 2. Local pool first.
	rel := p.pickLocalOriginal(typ, slot)
	if rel == "" {
		// 3. Remote source pool (R2 → on-demand download).
		if r := p.pickRemoteOriginal(ctx, typ); r != "" {
			rel = r
		}
	}
	if rel == "" {
		// 4. Live fetch from upstream providers.
		count := p.cfg.InitialPrefetchCount
		if count <= 0 {
			count = 5
		}
		if saved, err := p.FetchToLocal(ctx, FetchRequest{
			Type: typ, Keyword: keyword, Width: w, Height: h, Count: count,
		}); err == nil && len(saved) > 0 {
			// Use the first one we just saved.
			rel = saved[0]
		}
	}

	// 5. Procedural fallback.
	isFallback := false
	if rel == "" {
		fbRel, err := p.makeFallbackOriginal(typ)
		if err != nil {
			return "", false, fmt.Errorf("fallback original: %w", err)
		}
		rel = fbRel
		isFallback = true
	}

	if cacheKey != "" {
		if err := p.db.PutURLCache(ctx, cacheKey, rel); err != nil {
			p.log.Warn("url cache put failed", "err", err)
		}
		p.urlCacheLRU.Add(cacheKey, rel)
	}
	return rel, isFallback, nil
}

func (p *Pipeline) sourceUsable(rel string) bool {
	if rel == "" {
		return false
	}
	if _, err := os.Stat(filepath.Join(p.cfg.AbsImagesDir(), rel)); err == nil {
		return true
	}
	// Allow remote-only originals (they're downloaded on demand).
	if p.r2c == nil {
		return false
	}
	if u, _ := p.db.GetR2UploadByR2Key(context.Background(), rel); u != nil {
		return true
	}
	if s, _ := p.db.SourceImageBySHA1(context.Background(), strings.TrimSuffix(filepath.Base(rel), filepath.Ext(rel))); s != nil {
		return true
	}
	return false
}

// PickOriginal returns the relative path of one original-resolution image
// for `typ`, suitable for serving directly without resizing — wallpaper-app
// use case behind `/{type}` and `/type/{type}` routes.
//
// Selection order mirrors selectSource but skips the (w,h) variant cache:
//  1. If `fixed` is set, look it up in url_cache (LRU then SQLite) so the
//     same r value resolves to the same image across requests.
//  2. Local disk pool under images/original/{type}/.
//  3. Remote source_images pool (download lazily).
//  4. Live fetch from upstream provider chain.
//
// Returns the same shape of `rel` (e.g. "original/landscape/abc.jpg") used
// elsewhere. Callers can map that to a CDN URL via the r2_uploads table,
// or stream the local file directly.
func (p *Pipeline) PickOriginal(ctx context.Context, typ, keyword, fixed string) (string, error) {
	typ = source.NormalizeType(typ)

	// 1. Honor fixed `?r=N` via the same url_cache used by render variants,
	// namespaced under a "wallpaper" scope so it doesn't collide with sized
	// variants of the same (type, r).
	cacheKey := ""
	if fixed != "" {
		cacheKey = variantCacheKey("wallpaper:"+typ, fixed, "")
		if rel, ok := p.urlCacheLRU.Get(cacheKey); ok && p.sourceUsable(rel) {
			return rel, nil
		}
		if rel, ok, _ := p.db.GetURLCache(ctx, cacheKey); ok && p.sourceUsable(rel) {
			p.urlCacheLRU.Add(cacheKey, rel)
			return rel, nil
		}
	}

	// 2. Local pool.
	rel := p.pickLocalOriginal(typ, fixed)
	if rel == "" {
		// 3. Remote source pool.
		if r := p.pickRemoteOriginal(ctx, typ); r != "" {
			rel = r
		}
	}
	if rel == "" {
		// 4. Live fetch from upstream. Pull a small batch so subsequent
		// wallpaper hits warm up too.
		if saved, err := p.FetchToLocal(ctx, FetchRequest{
			Type:    typ,
			Keyword: keyword,
			Count:   5,
		}); err == nil && len(saved) > 0 {
			rel = saved[0]
		}
	}
	if rel == "" {
		return "", fmt.Errorf("no source available for type %q", typ)
	}

	// Persist the fixed mapping so future `?r=N` hits skip the picker.
	if cacheKey != "" {
		_ = p.db.PutURLCache(ctx, cacheKey, rel)
		p.urlCacheLRU.Add(cacheKey, rel)
	}
	return rel, nil
}

// PickLargestOriginal returns the largest local original for category preview
// pages. It falls back to the regular picker when the local pool is empty.
func (p *Pipeline) PickLargestOriginal(ctx context.Context, typ string) (string, error) {
	typ = source.NormalizeType(typ)
	if rel := p.pickLargestLocalOriginal(typ); rel != "" {
		return rel, nil
	}
	return p.PickOriginal(ctx, typ, "", "")
}

// EnsureSourceLocal exposes ensureSourceLocal so handlers can stream the
// original file when CDN redirect is disabled or unavailable.
func (p *Pipeline) EnsureSourceLocal(ctx context.Context, rel string) (string, error) {
	return p.ensureSourceLocal(ctx, rel)
}

// pickLocalOriginal returns a random original path under images/original/{type}/, or "".
// When slot is non-empty we deterministically map slot → file index so different
// slots reliably yield different images.
func (p *Pipeline) pickLocalOriginal(typ, slot string) string {
	dir := filepath.Join(p.cfg.AbsImagesDir(), "original", typ)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	files := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		files = append(files, name)
	}
	if len(files) == 0 {
		return ""
	}
	sort.Strings(files)

	idx := pseudoRandIndex(slot, len(files))
	return filepath.ToSlash(filepath.Join("original", typ, files[idx]))
}

func (p *Pipeline) pickLargestLocalOriginal(typ string) string {
	dir := filepath.Join(p.cfg.AbsImagesDir(), "original", typ)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}

	bestName := ""
	var bestPixels int64
	var bestSize int64
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		lower := strings.ToLower(name)
		if strings.HasPrefix(name, ".") ||
			strings.Contains(lower, "fallback") ||
			!encoder.SupportedExt(filepath.Ext(name)) {
			continue
		}

		abs := filepath.Join(dir, name)
		width, height, err := encoder.Probe(abs)
		if err != nil || width <= 0 || height <= 0 {
			continue
		}
		pixels := int64(width) * int64(height)
		size := int64(0)
		if info, err := e.Info(); err == nil {
			size = info.Size()
		}
		if pixels > bestPixels || (pixels == bestPixels && size > bestSize) {
			bestName = name
			bestPixels = pixels
			bestSize = size
		}
	}
	if bestName == "" {
		return ""
	}
	return filepath.ToSlash(filepath.Join("original", typ, bestName))
}

// pickRemoteOriginal returns the relative key (= R2 key for source pool images),
// after ensuring a row exists for the type.
func (p *Pipeline) pickRemoteOriginal(ctx context.Context, typ string) string {
	if p.r2c == nil {
		return ""
	}
	rows, err := p.db.PickRandomSourceImages(ctx, typ, 1)
	if err != nil || len(rows) == 0 {
		return ""
	}
	// R2 key matches the relative path convention used elsewhere.
	return rows[0].R2Key
}

func (p *Pipeline) ensureSourceLocal(ctx context.Context, rel string) (string, error) {
	abs := filepath.Join(p.cfg.AbsImagesDir(), rel)
	if fi, err := os.Stat(abs); err == nil && fi.Size() > 0 {
		return abs, nil
	}
	if p.r2c == nil {
		return "", fmt.Errorf("source missing locally and R2 disabled: %s", rel)
	}
	if _, err := p.r2c.GetToFile(ctx, rel, abs); err != nil {
		return "", err
	}
	return abs, nil
}

// ======================================================================
// Helpers
// ======================================================================

func clampDim(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func variantCacheKey(profileKey, variant, slot string) string {
	if variant == "" && slot == "" {
		return ""
	}
	parts := []string{profileKey}
	if variant != "" {
		parts = append(parts, "r="+variant)
	}
	if slot != "" {
		parts = append(parts, "s="+slot)
	}
	h := sha1.Sum([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(h[:])
}

// pseudoRandIndex maps an optional seed to a stable index in [0, n).
// When seed is empty, picks a uniformly random index (so vanilla
// /{w}/{h} requests get a different image on each refresh, matching
// the PHP version's behaviour).
func pseudoRandIndex(seed string, n int) int {
	if n <= 1 {
		return 0
	}
	if seed == "" {
		return mrand.IntN(n)
	}
	h := sha1.Sum([]byte(seed))
	// Read first 8 bytes as big-endian uint, mod n.
	var v uint64
	for i := 0; i < 8; i++ {
		v = (v << 8) | uint64(h[i])
	}
	return int(v % uint64(n))
}

// ======================================================================
// CDN URL resolution
// ======================================================================

// CDNURLFor returns the CDN URL of an already-uploaded artefact, or "" if not found.
func (p *Pipeline) CDNURLFor(ctx context.Context, rel string) string {
	if p.r2c == nil {
		return ""
	}
	if u, _ := p.db.GetR2Upload(ctx, rel); u != nil {
		return u.CDNURL
	}
	return ""
}

// CDNBase returns the configured public CDN root (or "").
func (p *Pipeline) CDNBase() string {
	if p.r2c == nil {
		return ""
	}
	return p.r2c.CDNURL("")
}
