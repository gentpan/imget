package imgpipe

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// CountOriginalsForType returns the number of files in images/original/{type}/
// (locally cached originals only — does not count R2 source pool).
func (p *Pipeline) CountOriginalsForType(typ string) int {
	dir := filepath.Join(p.cfg.AbsImagesDir(), "original", typ)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	n := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		n++
	}
	return n
}

// CountRenderedForProfile counts cached rendered variants for (type,w,h).
// Files are named "{w}x{h}-{hash}.{format}" under images/{type}/.
func (p *Pipeline) CountRenderedForProfile(typ string, w, h int) int {
	dir := filepath.Join(p.cfg.AbsImagesDir(), typ)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	prefix := fmt.Sprintf("%dx%d-", w, h)
	n := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasPrefix(e.Name(), prefix) {
			n++
		}
	}
	return n
}

// LibrarySummary is the whole-library rollup shown in the footer: count and
// byte size of local originals, plus a per-type breakdown.
type LibrarySummary struct {
	TotalImages int64
	TotalBytes  int64
	ByType      map[string]int64
}

var (
	libSumMu     sync.Mutex
	libSumCache  LibrarySummary
	libSumExpiry time.Time
)

// LibrarySummary walks images/original/{type}/ and totals the local originals.
// Result is cached for a few minutes so the homepage doesn't re-stat the tree
// on every request. This reflects the real on-disk library — unlike the old
// r2_uploads-based count, which stopped moving once R2 uploads were disabled.
func (p *Pipeline) LibrarySummary() LibrarySummary {
	libSumMu.Lock()
	defer libSumMu.Unlock()
	if !libSumExpiry.IsZero() && time.Now().Before(libSumExpiry) {
		return libSumCache
	}

	sum := LibrarySummary{ByType: make(map[string]int64)}
	base := filepath.Join(p.cfg.AbsImagesDir(), "original")
	typeDirs, err := os.ReadDir(base)
	if err == nil {
		for _, td := range typeDirs {
			if !td.IsDir() {
				continue
			}
			typ := td.Name()
			entries, err := os.ReadDir(filepath.Join(base, typ))
			if err != nil {
				continue
			}
			var cnt, bytes int64
			for _, e := range entries {
				if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
					continue
				}
				cnt++
				if info, err := e.Info(); err == nil {
					bytes += info.Size()
				}
			}
			if cnt > 0 {
				sum.ByType[typ] = cnt
				sum.TotalImages += cnt
				sum.TotalBytes += bytes
			}
		}
	}

	libSumCache = sum
	libSumExpiry = time.Now().Add(5 * time.Minute)
	return sum
}

// HasAnyOriginal returns true if any original (local OR R2 source pool) exists.
func (p *Pipeline) HasAnyOriginal(ctx context.Context, typ string) bool {
	if p.CountOriginalsForType(typ) > 0 {
		return true
	}
	if p.db == nil {
		return false
	}
	n, _ := p.db.CountSourceImages(ctx, typ)
	return n > 0
}
