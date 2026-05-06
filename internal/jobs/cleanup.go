package jobs

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"imget/internal/config"
	"imget/internal/db"
)

// CleanupCache deletes rendered variant files older than ttlHours, but only
// when they are confirmed present in r2_uploads. Originals are never touched.
//
// dryRun=true reports what would be deleted without doing it.
func CleanupCache(ctx context.Context, log *slog.Logger, cfg *config.Config, sqlDB *db.DB, ttlHours int, dryRun bool) error {
	root := cfg.AbsImagesDir()
	if ttlHours <= 0 {
		ttlHours = cfg.LocalRenderCacheTTLHours
	}
	if ttlHours <= 0 {
		ttlHours = 168
	}
	cutoff := time.Now().Add(-time.Duration(ttlHours) * time.Hour)

	deleted, kept, missingR2 := 0, 0, 0
	var freedBytes int64

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)

		// Skip originals.
		if strings.HasPrefix(rel, "original/") {
			return nil
		}
		// Only image files.
		switch strings.ToLower(filepath.Ext(d.Name())) {
		case ".webp", ".avif", ".jpg", ".jpeg", ".png", ".gif":
		default:
			return nil
		}

		fi, err := d.Info()
		if err != nil {
			return nil
		}
		if fi.ModTime().After(cutoff) {
			kept++
			return nil
		}

		// Only delete when present in R2.
		u, _ := sqlDB.GetR2Upload(ctx, rel)
		if u == nil {
			missingR2++
			return nil
		}

		if dryRun {
			log.Info("would delete", "path", rel, "size", fi.Size())
		} else {
			if err := os.Remove(path); err != nil {
				log.Warn("delete failed", "path", rel, "err", err)
				return nil
			}
		}
		deleted++
		freedBytes += fi.Size()
		return nil
	})
	if err != nil {
		return err
	}

	// Also prune stale refresh_logs (>30d) and refresh planner stats so the
	// SQLite query planner has accurate row-count estimates after we just
	// removed a chunk of variants.
	if !dryRun {
		if n, err := sqlDB.PruneRefreshLogs(ctx, 30*24*3600); err == nil && n > 0 {
			log.Info("pruned refresh_logs", "rows", n)
		}
		_ = sqlDB.Checkpoint(ctx)
		if _, err := sqlDB.ExecContext(ctx, "ANALYZE"); err != nil {
			log.Warn("ANALYZE failed", "err", err)
		} else {
			log.Info("ANALYZE done")
		}
	}

	log.Info("cleanup-cache done",
		"deleted", deleted, "kept", kept, "missing_r2", missingR2,
		"freed", freedBytes, "dry_run", dryRun)
	return nil
}
