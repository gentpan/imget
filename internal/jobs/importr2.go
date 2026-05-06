package jobs

import (
	"context"
	"database/sql"
	"log/slog"
	"path"
	"strings"

	"imget/internal/db"
	"imget/internal/r2"
	"imget/internal/source"
)

// ImportR2 walks the `original/` prefix in the bucket and records each
// original into both r2_uploads (so the runtime never re-uploads it) and
// source_images (so the renderer can pick it as a source). Non-original
// objects are ignored entirely — they're either already-rendered variants
// (handled separately by `r2-prune-variants`) or other state we don't want
// to import.
//
// Idempotent — re-running picks up newly-arrived originals.
func ImportR2(ctx context.Context, log *slog.Logger, sqlDB *db.DB, client *r2.Client) error {
	const prefix = "original/"
	uploadsAdded, sourcesAdded, malformed := 0, 0, 0

	err := client.ListAll(ctx, prefix, func(o r2.ObjectInfo) error {
		key := strings.TrimLeft(o.Key, "/")
		cdn := client.CDNURL(key)

		parts := strings.SplitN(key, "/", 3)
		if len(parts) < 3 {
			malformed++
			return nil
		}
		typeRaw := parts[1]
		fileName := parts[2]
		typ := source.NormalizeType(typeRaw)
		ext := strings.ToLower(path.Ext(fileName))
		base := strings.TrimSuffix(fileName, ext)

		// r2_uploads: we know this exact key is already in R2.
		if err := sqlDB.PutR2Upload(ctx, db.R2Upload{
			FilePath: key,
			R2Key:    key,
			CDNURL:   cdn,
			FileSize: o.Size,
		}); err != nil {
			log.Warn("import r2_uploads put failed", "key", key, "err", err)
			return nil
		}
		uploadsAdded++

		// source_images: pickable original by type.
		var cdnNS sql.NullString
		if cdn != "" {
			cdnNS = sql.NullString{String: cdn, Valid: true}
		}
		if err := sqlDB.PutSourceImage(ctx, db.SourceImage{
			R2Key:    key,
			Type:     typ,
			SHA1:     base,
			Ext:      strings.TrimPrefix(ext, "."),
			FileSize: o.Size,
			CDNURL:   cdnNS,
		}); err != nil {
			log.Warn("import source_images put failed", "key", key, "err", err)
			return nil
		}
		sourcesAdded++

		if (uploadsAdded % 500) == 0 {
			log.Info("import progress", "scanned", uploadsAdded)
		}
		return nil
	})
	if err != nil {
		return err
	}

	totals, _ := sqlDB.ListSourceTypes(ctx)
	log.Info("import-r2 done",
		"uploads_recorded", uploadsAdded,
		"sources_recorded", sourcesAdded,
		"malformed_skipped", malformed,
		"by_type", totals)
	return nil
}
