// Package jobs implements the CLI subcommand bodies (daily-topup, r2-sync,
// cleanup-cache, import-r2). Each function takes an already-initialized
// pipeline and writes a short summary to its logger.
package jobs

import (
	"context"
	"log/slog"
	"time"

	"imget/internal/db"
	"imget/internal/imgpipe"
	"imget/internal/source"
)

// DailyTopup walks the request_profiles table and refetches `increment` new
// originals per profile. Profiles already topped-up today (UTC) are skipped.
func DailyTopup(ctx context.Context, log *slog.Logger, sqlDB *db.DB, p *imgpipe.Pipeline, increment, maxPerType int) error {
	profiles, err := sqlDB.ListProfiles(ctx, 5000)
	if err != nil {
		return err
	}
	if increment <= 0 {
		increment = 10
	}

	todayStart := time.Now().UTC().Truncate(24 * time.Hour).Unix()

	processed, skipped, total := 0, 0, 0
	for _, prof := range profiles {
		if prof.LastDailyTopupAt.Valid && prof.LastDailyTopupAt.Int64 >= todayStart {
			skipped++
			continue
		}

		typ := source.NormalizeType(prof.Type)
		// Cap per-type if max provided.
		if maxPerType > 0 {
			if n, _ := sqlDB.CountSourceImages(ctx, typ); int(n) >= maxPerType {
				log.Info("topup skip (cap reached)", "type", typ, "count", n, "cap", maxPerType)
				_ = sqlDB.MarkProfileRefresh(ctx, prof.ProfileKey, "daily", 0)
				continue
			}
		}

		saved, err := p.FetchToLocal(ctx, imgpipe.FetchRequest{
			Type:    typ,
			Keyword: prof.Keyword,
			Width:   prof.Width,
			Height:  prof.Height,
			Count:   increment,
		})
		errText := ""
		if err != nil {
			errText = err.Error()
		}
		_ = sqlDB.AddRefreshLog(ctx, prof.ProfileKey, prof.Width, prof.Height, prof.Type, prof.Keyword,
			"daily", increment, len(saved), errText)
		_ = sqlDB.MarkProfileRefresh(ctx, prof.ProfileKey, "daily", len(saved))

		if err != nil {
			log.Warn("topup error", "profile", prof.ProfileKey, "err", err)
			continue
		}
		log.Info("topup ok", "profile", prof.ProfileKey, "type", typ, "saved", len(saved))
		processed++
		total += len(saved)
	}

	log.Info("topup summary", "processed", processed, "skipped", skipped, "saved", total)
	return nil
}
