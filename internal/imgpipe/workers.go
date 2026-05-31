package imgpipe

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"imget/internal/db"
	"imget/internal/metrics"
)

// UploadOneInline performs a single synchronous R2 upload. Used by r2-sync
// to walk the disk and back-fill the upload table.
func (p *Pipeline) UploadOneInline(ctx context.Context, rel string) error {
	return p.uploadOne(ctx, rel)
}

func (p *Pipeline) uploadWorker(id int) {
	defer p.wg.Done()
	for rel := range p.uploadCh {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		err := p.uploadOne(ctx, rel)
		cancel()
		if err != nil {
			p.log.Warn("R2 upload failed", "worker", id, "path", rel, "err", err)
		}
	}
}

// uploadOne uploads a single file (relative path) to R2 and records the
// upload in the r2_uploads table. Idempotent — if already uploaded with the
// same size, we skip the actual transfer.
func (p *Pipeline) uploadOne(ctx context.Context, rel string) error {
	if p.r2c == nil || rel == "" {
		return nil
	}
	abs := filepath.Join(p.cfg.AbsImagesDir(), rel)
	fi, err := os.Stat(abs)
	if err != nil {
		return fmt.Errorf("stat: %w", err)
	}

	// Check if already recorded with correct size.
	existing, err := p.db.GetR2Upload(ctx, rel)
	if err != nil {
		return err
	}
	if existing != nil && existing.FileSize == fi.Size() {
		p.removeLocalVariant(rel)
		return nil
	}

	contentType := contentTypeForExt(filepath.Ext(rel))
	size, err := p.r2c.PutFile(ctx, abs, rel, contentType, p.cfg.R2UploadVerify)
	if err != nil {
		metrics.R2UploadTotal.WithLabelValues("err").Inc()
		return err
	}
	metrics.R2UploadTotal.WithLabelValues("ok").Inc()
	metrics.R2UploadBytes.Observe(float64(size))

	cdn := p.r2c.CDNURL(rel)
	if err := p.db.PutR2Upload(ctx, db.R2Upload{
		FilePath: rel,
		R2Key:    rel,
		CDNURL:   cdn,
		FileSize: size,
	}); err != nil {
		return err
	}

	p.removeLocalVariant(rel)
	return nil
}

func (p *Pipeline) removeLocalVariant(rel string) {
	if rel == "" || strings.HasPrefix(filepath.ToSlash(rel), "original/") {
		return
	}
	if err := os.Remove(filepath.Join(p.cfg.AbsImagesDir(), rel)); err != nil && !os.IsNotExist(err) {
		p.log.Warn("delete local rendered variant failed", "path", rel, "err", err)
	}
}

func contentTypeForExt(ext string) string {
	switch strings.ToLower(ext) {
	case ".webp":
		return "image/webp"
	case ".avif":
		return "image/avif"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	}
	return "application/octet-stream"
}
