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

	var (
		total       int64
		willDelete  int64
		bytesToFree int64
	)

	bd := newBatchDeleter(client, log, dryRun, "prune")
	// Mirror each deleted chunk into r2_uploads so the runtime forgets them.
	// Best-effort — a stray DELETE on a non-existent row is a harmless no-op.
	bd.afterDelete = func(ctx context.Context, keys []string) {
		for _, k := range keys {
			_, _ = sqlDB.ExecContext(ctx, `DELETE FROM r2_uploads WHERE r2_key = ?`, k)
		}
	}

	start := time.Now()
	err := client.ListAll(ctx, "", func(o r2.ObjectInfo) error {
		total++
		if strings.HasPrefix(o.Key, keepPrefix) {
			return nil // keep
		}
		willDelete++
		bytesToFree += o.Size
		return bd.add(ctx, o.Key)
	})
	if err != nil {
		return err
	}
	if err := bd.flush(ctx); err != nil {
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
