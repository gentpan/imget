// Package jobs implements the CLI subcommand bodies (daily-topup, r2-sync,
// cleanup-cache, import-r2). Each function takes an already-initialized
// pipeline and writes a short summary to its logger.
package jobs

import (
	"context"
	"log/slog"
	"math/rand"
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

// TopupByType iterates source.AllowedTypes and fetches a random count in
// [minN, maxN] of fresh originals per type. Uses a random Pixabay page and
// order=latest to escape the "top hits already collected" dedup deadlock
// that affects the profile-based DailyTopup when the pool has matured.
func TopupByType(ctx context.Context, log *slog.Logger, sqlDB *db.DB, p *imgpipe.Pipeline, minN, maxN int) error {
	if minN <= 0 {
		minN = 5
	}
	if maxN < minN {
		maxN = minN
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	types := source.AllowedTypes

	totalSaved, errs := 0, 0
	for _, typ := range types {
		n := minN
		if maxN > minN {
			n += rng.Intn(maxN - minN + 1)
		}
		// Rotate through the type's sub-keyword expansion: each run queries a
		// fresh narrow phrase, unlocking a new result window instead of
		// re-sampling the parent keyword's "latest" page. The image is still
		// filed under the parent type, so the public 20 buckets are unchanged.
		keyword := source.PickSubKeyword(typ, rng.Int())
		page := 1 + rng.Intn(6) // narrow sub-keywords have shallower result sets than the broad parent term.

		saved, err := p.FetchToLocal(ctx, imgpipe.FetchRequest{
			Type:         typ,
			Keyword:      keyword,
			Count:        n,
			Page:         page,
			Order:        "latest",
			AllProviders: true, // union every source so CC providers contribute, not just the first non-empty one.
		})
		errText := ""
		if err != nil {
			errText = err.Error()
			errs++
		}
		_ = sqlDB.AddRefreshLog(ctx, "type:"+typ, 0, 0, typ, keyword, "daily-types", n, len(saved), errText)

		if err != nil {
			log.Warn("topup-types error", "type", typ, "keyword", keyword, "page", page, "requested", n, "err", err)
			continue
		}
		log.Info("topup-types ok", "type", typ, "keyword", keyword, "page", page, "requested", n, "saved", len(saved))
		totalSaved += len(saved)
	}

	log.Info("topup-types summary", "types", len(types), "saved", totalSaved, "errors", errs)
	return nil
}
