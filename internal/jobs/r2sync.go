package jobs

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"imget/internal/config"
	"imget/internal/db"
	"imget/internal/imgpipe"
)

// R2Sync walks the local images/ tree and uploads anything not yet recorded
// in r2_uploads. Idempotent.
func R2Sync(ctx context.Context, log *slog.Logger, cfg *config.Config, sqlDB *db.DB, pipe *imgpipe.Pipeline) error {
	root := cfg.AbsImagesDir()
	uploaded, skipped, failed := 0, 0, 0

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		name := d.Name()
		if strings.HasPrefix(name, ".") || strings.HasSuffix(name, ".part") {
			return nil
		}
		// Only image files.
		switch strings.ToLower(filepath.Ext(name)) {
		case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".avif":
		default:
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)

		if u, _ := sqlDB.GetR2Upload(ctx, rel); u != nil {
			if !strings.HasPrefix(rel, "original/") {
				if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
					log.Warn("r2sync local variant delete failed", "path", rel, "err", err)
				}
			}
			skipped++
			return nil
		}
		// Upload synchronously by waiting on QueueUpload? We'll call internal helper directly.
		if err := pipe.UploadOneInline(ctx, rel); err != nil {
			log.Warn("r2sync upload failed", "path", rel, "err", err)
			failed++
			return nil
		}
		uploaded++
		return nil
	})
	if err != nil {
		return err
	}
	log.Info("r2-sync done", "uploaded", uploaded, "skipped", skipped, "failed", failed)
	return nil
}
