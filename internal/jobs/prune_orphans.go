package jobs

import (
	"context"
	"log/slog"
	"time"

	"imget/internal/db"
	"imget/internal/r2"
)

// PruneR2Orphans walks every object in the bucket and deletes any whose key
// is not referenced by r2_uploads or source_images. These accumulate when
// rows get removed from the DB (UNIQUE-conflict pruning, deduped Picsum
// fallbacks, manual cleanup) but the underlying R2 object never gets erased.
//
// In dry-run mode it only counts. With dryRun=false it issues batched
// DeleteObjects calls (1000 keys per request, the S3 hard limit).
func PruneR2Orphans(
	ctx context.Context,
	log *slog.Logger,
	sqlDB *db.DB,
	client *r2.Client,
	dryRun bool,
) error {
	// 1. Load every key that the DB still considers "ours" into a set.
	// Union r2_uploads + source_images so we don't accidentally delete a
	// frozen import-r2 row that hasn't been touched since deploy.
	known := make(map[string]struct{}, 20000)

	rows, err := sqlDB.QueryContext(ctx, `SELECT r2_key FROM r2_uploads`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			rows.Close()
			return err
		}
		known[k] = struct{}{}
	}
	rows.Close()

	rows, err = sqlDB.QueryContext(ctx, `SELECT r2_key FROM source_images`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			rows.Close()
			return err
		}
		known[k] = struct{}{}
	}
	rows.Close()

	log.Info("loaded known keys", "count", len(known))

	// 2. Walk the bucket; any key not in `known` is an orphan.
	var (
		total       int64
		orphans     int64
		bytesToFree int64
	)

	bd := newBatchDeleter(client, log, dryRun, "prune-orphans")

	start := time.Now()
	if err := client.ListAll(ctx, "", func(o r2.ObjectInfo) error {
		total++
		if _, ok := known[o.Key]; ok {
			return nil
		}
		orphans++
		bytesToFree += o.Size
		// Surface a handful of examples for the operator before deletion.
		if orphans <= 5 {
			log.Info("orphan example",
				"key", o.Key,
				"size", o.Size)
		}
		return bd.add(ctx, o.Key)
	}); err != nil {
		return err
	}
	if err := bd.flush(ctx); err != nil {
		return err
	}

	mode := "DELETE"
	if dryRun {
		mode = "DRY-RUN"
	}
	log.Info(mode+" prune-orphans summary",
		"objects_total", total,
		"objects_kept", total-orphans,
		"objects_orphan", orphans,
		"bytes_freed", bytesToFree,
		"mb_freed", bytesToFree/(1<<20),
		"elapsed", time.Since(start).Round(time.Second),
	)
	return nil
}
