package jobs

import (
	"context"
	"log/slog"

	"imget/internal/r2"
)

// deleteBatchSize is the S3 DeleteObjects hard limit per request.
const deleteBatchSize = 1000

// batchDeleter accumulates R2 keys and deletes them in chunks of deleteBatchSize.
// In dryRun mode it counts but never deletes. The optional afterDelete hook runs
// once per successfully-deleted chunk (prune-variants uses it to mirror the
// deletions into r2_uploads). It replaces the byte-for-byte identical flush
// closures that lived in both prune.go and prune_orphans.go.
type batchDeleter struct {
	client      *r2.Client
	log         *slog.Logger
	dryRun      bool
	label       string
	afterDelete func(ctx context.Context, keys []string)

	pending []string
	deleted int64
}

func newBatchDeleter(client *r2.Client, log *slog.Logger, dryRun bool, label string) *batchDeleter {
	return &batchDeleter{client: client, log: log, dryRun: dryRun, label: label}
}

// add queues a key, auto-flushing once a full batch has accumulated.
func (b *batchDeleter) add(ctx context.Context, key string) error {
	b.pending = append(b.pending, key)
	if len(b.pending) >= deleteBatchSize {
		return b.flush(ctx)
	}
	return nil
}

// flush deletes the currently-pending keys (a no-op in dryRun mode).
func (b *batchDeleter) flush(ctx context.Context) error {
	if len(b.pending) == 0 {
		return nil
	}
	keys := b.pending
	b.pending = nil
	if b.dryRun {
		return nil
	}
	if err := b.client.DeleteBatch(ctx, keys); err != nil {
		return err
	}
	if b.afterDelete != nil {
		b.afterDelete(ctx, keys)
	}
	b.deleted += int64(len(keys))
	b.log.Info(b.label+" progress", "deleted_total", b.deleted)
	return nil
}
