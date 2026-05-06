package imgpipe

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
