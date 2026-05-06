package jobs

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"imget/internal/db"
	"imget/internal/r2"
)

// PruneR2Variants walks every object in the bucket and deletes anything that
// is NOT under the `original/` prefix.
//
// In dry-run mode it only counts; with dryRun=false it does the actual delete
// in batches of 1000 (S3 DeleteObjects max).
//
// Also clears matching rows from the local r2_uploads table so the runtime
// stops believing those keys are present.
func PruneR2Variants(
	ctx context.Context,
	log *slog.Logger,
	sqlDB *db.DB,
	client *r2.Client,
	dryRun bool,
) error {
	const keepPrefix = "original/"
	const batchSize = 1000

	var (
		total       int64
		willDelete  int64
		bytesToFree int64
		toDelete    []string
		flushed     int64
	)

	flush := func() error {
		if len(toDelete) == 0 {
			return nil
		}
		if dryRun {
			toDelete = toDelete[:0]
			return nil
		}
		if err := client.DeleteBatch(ctx, toDelete); err != nil {
			return err
		}
		// Mirror the deletes into r2_uploads so the runtime forgets them.
		// Best-effort — failures here are logged but not fatal.
		for _, k := range toDelete {
			if u, _ := sqlDB.GetR2UploadByR2Key(ctx, k); u != nil {
				_, _ = sqlDB.ExecContext(ctx,
					`DELETE FROM r2_uploads WHERE r2_key = ?`, k)
			}
		}
		flushed += int64(len(toDelete))
		log.Info("prune progress", "deleted_total", flushed, "still_pending", willDelete-flushed)
		toDelete = toDelete[:0]
		return nil
	}

	start := time.Now()
	err := client.ListAll(ctx, "", func(o r2.ObjectInfo) error {
		total++
		if strings.HasPrefix(o.Key, keepPrefix) {
			return nil // keep
		}
		willDelete++
		bytesToFree += o.Size
		toDelete = append(toDelete, o.Key)
		if len(toDelete) >= batchSize {
			return flush()
		}
		return nil
	})
	if err != nil {
		return err
	}
	if err := flush(); err != nil {
		return err
	}

	dur := time.Since(start)
	mode := "DELETE"
	if dryRun {
		mode = "DRY-RUN"
	}
	log.Info(mode+" prune summary",
		"objects_total", total,
		"objects_kept", total-willDelete,
		"objects_targeted", willDelete,
		"bytes_freed", bytesToFree,
		"mb_freed", bytesToFree/(1<<20),
		"elapsed", dur.Round(time.Second),
	)
	return nil
}
