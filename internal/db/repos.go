package db

import (
	"context"
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ============================================================
// request_profiles
// ============================================================

type Profile struct {
	ProfileKey            string
	Width, Height         int
	Type                  string
	Keyword               string
	RequestCount          int64
	ViewCount             int64
	DownloadCount         int64
	FirstRequestedAt      int64
	LastRequestedAt       int64
	LastDailyTopupAt      sql.NullInt64
	LastDailyTopupSaved   int64
	InitialPrefetchDoneAt sql.NullInt64
}

// ProfileKey deterministically hashes a profile request.
func ProfileKey(width, height int, typ, keyword string) string {
	h := sha1.Sum([]byte(fmt.Sprintf("%dx%d|%s|%s", width, height, typ, keyword)))
	return hex.EncodeToString(h[:])
}

// RegisterProfile upserts a profile and bumps request_count.
func (d *DB) RegisterProfile(ctx context.Context, width, height int, typ, keyword string) (string, error) {
	key := ProfileKey(width, height, typ, keyword)
	now := time.Now().Unix()
	_, err := d.ExecContext(ctx, `
		INSERT INTO request_profiles
			(profile_key, width, height, type, keyword,
			 request_count, first_requested_at, last_requested_at)
		VALUES (?, ?, ?, ?, ?, 1, ?, ?)
		ON CONFLICT(profile_key) DO UPDATE SET
			request_count = request_count + 1,
			last_requested_at = excluded.last_requested_at`,
		key, width, height, typ, keyword, now, now)
	return key, err
}

// ProfileMetricsDelta is one row of in-memory counter accumulation that gets
// flushed to the DB by BatchAddProfileMetrics in a single transaction.
type ProfileMetricsDelta struct {
	ProfileKey       string
	AddRequestCount  int64
	AddViewCount     int64
	AddDownloadCount int64
	LastRequestedAt  int64 // unix seconds; takes max with existing value
}

// BatchAddProfileMetrics applies N counter deltas in one transaction.
// Used by the in-memory batcher; rows that don't exist are silently skipped
// (the batcher only ever buffers profiles that have already been registered).
func (d *DB) BatchAddProfileMetrics(ctx context.Context, deltas []ProfileMetricsDelta) error {
	if len(deltas) == 0 {
		return nil
	}
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.PrepareContext(ctx, `
		UPDATE request_profiles
		   SET request_count   = request_count   + ?,
		       view_count      = view_count      + ?,
		       download_count  = download_count  + ?,
		       last_requested_at = MAX(last_requested_at, ?)
		 WHERE profile_key = ?`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, d := range deltas {
		if _, err := stmt.ExecContext(ctx,
			d.AddRequestCount, d.AddViewCount, d.AddDownloadCount,
			d.LastRequestedAt, d.ProfileKey); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// IncrementProfileMetric atomically bumps view_count or download_count.
func (d *DB) IncrementProfileMetric(ctx context.Context, profileKey, metric string) error {
	col := ""
	switch metric {
	case "view":
		col = "view_count"
	case "download":
		col = "download_count"
	default:
		return fmt.Errorf("unknown metric: %s", metric)
	}
	_, err := d.ExecContext(ctx,
		`UPDATE request_profiles SET `+col+` = `+col+` + 1 WHERE profile_key = ?`, profileKey)
	return err
}

func (d *DB) GetProfile(ctx context.Context, key string) (*Profile, error) {
	row := d.QueryRowContext(ctx, `
		SELECT profile_key, width, height, type, keyword,
		       request_count, view_count, download_count,
		       first_requested_at, last_requested_at,
		       last_daily_topup_at, last_daily_topup_saved,
		       initial_prefetch_done_at
		FROM request_profiles WHERE profile_key = ?`, key)
	p := &Profile{}
	err := row.Scan(&p.ProfileKey, &p.Width, &p.Height, &p.Type, &p.Keyword,
		&p.RequestCount, &p.ViewCount, &p.DownloadCount,
		&p.FirstRequestedAt, &p.LastRequestedAt,
		&p.LastDailyTopupAt, &p.LastDailyTopupSaved,
		&p.InitialPrefetchDoneAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return p, err
}

func (d *DB) ListProfiles(ctx context.Context, limit int) ([]*Profile, error) {
	if limit <= 0 {
		limit = 500
	}
	rows, err := d.QueryContext(ctx, `
		SELECT profile_key, width, height, type, keyword,
		       request_count, view_count, download_count,
		       first_requested_at, last_requested_at,
		       last_daily_topup_at, last_daily_topup_saved,
		       initial_prefetch_done_at
		FROM request_profiles
		ORDER BY last_requested_at DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Profile
	for rows.Next() {
		p := &Profile{}
		if err := rows.Scan(&p.ProfileKey, &p.Width, &p.Height, &p.Type, &p.Keyword,
			&p.RequestCount, &p.ViewCount, &p.DownloadCount,
			&p.FirstRequestedAt, &p.LastRequestedAt,
			&p.LastDailyTopupAt, &p.LastDailyTopupSaved,
			&p.InitialPrefetchDoneAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// MarkProfileRefresh records a refresh-mode timestamp.
//
//	mode = "daily" | "initial" | "manual"
func (d *DB) MarkProfileRefresh(ctx context.Context, profileKey, mode string, saved int) error {
	now := time.Now().Unix()
	switch mode {
	case "daily":
		_, err := d.ExecContext(ctx, `
			UPDATE request_profiles
			   SET last_daily_topup_at = ?, last_daily_topup_saved = ?
			 WHERE profile_key = ?`, now, saved, profileKey)
		return err
	case "initial":
		_, err := d.ExecContext(ctx, `
			UPDATE request_profiles
			   SET initial_prefetch_done_at = ?
			 WHERE profile_key = ?`, now, profileKey)
		return err
	case "manual":
		// No dedicated column; manual refreshes are tracked via refresh_logs.
		return nil
	}
	return fmt.Errorf("unknown refresh mode: %s", mode)
}

// ============================================================
// url_cache
// ============================================================

func (d *DB) GetURLCache(ctx context.Context, key string) (string, bool, error) {
	row := d.QueryRowContext(ctx,
		`SELECT selected_path FROM url_cache WHERE cache_key = ?`, key)
	var p string
	if err := row.Scan(&p); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false, nil
		}
		return "", false, err
	}
	return p, true, nil
}

func (d *DB) PutURLCache(ctx context.Context, key, path string) error {
	now := time.Now().Unix()
	_, err := d.ExecContext(ctx, `
		INSERT INTO url_cache (cache_key, selected_path, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT(cache_key) DO UPDATE SET
			selected_path = excluded.selected_path,
			updated_at    = excluded.updated_at`,
		key, path, now)
	return err
}

func (d *DB) DeleteURLCacheByPath(ctx context.Context, path string) error {
	_, err := d.ExecContext(ctx, `DELETE FROM url_cache WHERE selected_path = ?`, path)
	return err
}

// ============================================================
// refresh_logs
// ============================================================

func (d *DB) AddRefreshLog(ctx context.Context, profileKey string, width, height int, typ, keyword, mode string, requested, saved int, errText string) error {
	now := time.Now().Unix()
	var et any
	if errText != "" {
		et = errText
	}
	_, err := d.ExecContext(ctx, `
		INSERT INTO refresh_logs
			(profile_key, width, height, type, keyword,
			 mode, requested_count, saved_count, error_text, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		profileKey, width, height, typ, keyword, mode, requested, saved, et, now)
	return err
}

func (d *DB) PruneRefreshLogs(ctx context.Context, olderThanSeconds int64) (int64, error) {
	cutoff := time.Now().Unix() - olderThanSeconds
	res, err := d.ExecContext(ctx, `DELETE FROM refresh_logs WHERE created_at < ?`, cutoff)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// ============================================================
// r2_uploads
// ============================================================

type R2Upload struct {
	FilePath   string
	R2Key      string
	CDNURL     string
	FileSize   int64
	UploadedAt int64
}

func (d *DB) GetR2Upload(ctx context.Context, filePath string) (*R2Upload, error) {
	row := d.QueryRowContext(ctx, `
		SELECT file_path, r2_key, cdn_url, file_size, uploaded_at
		  FROM r2_uploads WHERE file_path = ?`, filePath)
	u := &R2Upload{}
	err := row.Scan(&u.FilePath, &u.R2Key, &u.CDNURL, &u.FileSize, &u.UploadedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return u, err
}

func (d *DB) GetR2UploadByR2Key(ctx context.Context, r2Key string) (*R2Upload, error) {
	row := d.QueryRowContext(ctx, `
		SELECT file_path, r2_key, cdn_url, file_size, uploaded_at
		  FROM r2_uploads WHERE r2_key = ?`, r2Key)
	u := &R2Upload{}
	err := row.Scan(&u.FilePath, &u.R2Key, &u.CDNURL, &u.FileSize, &u.UploadedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return u, err
}

func (d *DB) PutR2Upload(ctx context.Context, u R2Upload) error {
	if u.UploadedAt == 0 {
		u.UploadedAt = time.Now().Unix()
	}
	_, err := d.ExecContext(ctx, `
		INSERT INTO r2_uploads (file_path, r2_key, cdn_url, file_size, uploaded_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(file_path) DO UPDATE SET
			r2_key      = excluded.r2_key,
			cdn_url     = excluded.cdn_url,
			file_size   = excluded.file_size,
			uploaded_at = excluded.uploaded_at`,
		u.FilePath, u.R2Key, u.CDNURL, u.FileSize, u.UploadedAt)
	return err
}

// ============================================================
// source_images (R2 remote source pool)
// ============================================================

type SourceImage struct {
	R2Key      string
	Type       string
	SHA1       string
	Ext        string
	FileSize   int64
	CDNURL     sql.NullString
	ImportedAt int64
}

func (d *DB) PutSourceImage(ctx context.Context, s SourceImage) error {
	if s.ImportedAt == 0 {
		s.ImportedAt = time.Now().Unix()
	}
	var cdn any
	if s.CDNURL.Valid {
		cdn = s.CDNURL.String
	}
	_, err := d.ExecContext(ctx, `
		INSERT INTO source_images (r2_key, type, sha1, ext, file_size, cdn_url, imported_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(r2_key) DO UPDATE SET
			type        = excluded.type,
			sha1        = excluded.sha1,
			ext         = excluded.ext,
			file_size   = excluded.file_size,
			cdn_url     = excluded.cdn_url,
			imported_at = excluded.imported_at`,
		s.R2Key, s.Type, s.SHA1, s.Ext, s.FileSize, cdn, s.ImportedAt)
	return err
}

// CountSourceImages returns the total source images for a given type. Pass empty
// type to count all rows.
func (d *DB) CountSourceImages(ctx context.Context, typ string) (int64, error) {
	var n int64
	if typ == "" {
		err := d.QueryRowContext(ctx, `SELECT COUNT(*) FROM source_images`).Scan(&n)
		return n, err
	}
	err := d.QueryRowContext(ctx, `SELECT COUNT(*) FROM source_images WHERE type = ?`, typ).Scan(&n)
	return n, err
}

// PickRandomSourceImages returns up to N random rows for the given type.
func (d *DB) PickRandomSourceImages(ctx context.Context, typ string, n int) ([]SourceImage, error) {
	if n <= 0 {
		n = 1
	}
	rows, err := d.QueryContext(ctx, `
		SELECT r2_key, type, sha1, ext, file_size, cdn_url, imported_at
		  FROM source_images
		 WHERE type = ?
		 ORDER BY RANDOM()
		 LIMIT ?`, typ, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SourceImage
	for rows.Next() {
		var s SourceImage
		if err := rows.Scan(&s.R2Key, &s.Type, &s.SHA1, &s.Ext, &s.FileSize, &s.CDNURL, &s.ImportedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// SourceImageBySHA1 fetches a row by its sha1 (independent of type).
func (d *DB) SourceImageBySHA1(ctx context.Context, sha1Hex string) (*SourceImage, error) {
	row := d.QueryRowContext(ctx, `
		SELECT r2_key, type, sha1, ext, file_size, cdn_url, imported_at
		  FROM source_images WHERE sha1 = ? LIMIT 1`, sha1Hex)
	var s SourceImage
	err := row.Scan(&s.R2Key, &s.Type, &s.SHA1, &s.Ext, &s.FileSize, &s.CDNURL, &s.ImportedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &s, err
}

// ListSourceTypes returns each known source type with its row count.
func (d *DB) ListSourceTypes(ctx context.Context) (map[string]int64, error) {
	rows, err := d.QueryContext(ctx,
		`SELECT type, COUNT(*) FROM source_images GROUP BY type`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]int64)
	for rows.Next() {
		var t string
		var n int64
		if err := rows.Scan(&t, &n); err != nil {
			return nil, err
		}
		out[strings.ToLower(t)] = n
	}
	return out, rows.Err()
}

// ============================================================
// stats
// ============================================================

type LibrarySummary struct {
	TotalImages int64
	TotalBytes  int64
	ByType      map[string]int64
}

// LibrarySummary returns counts/bytes of *originals* — i.e. rows in
// r2_uploads whose file_path lives under `original/`. The source_images
// table only holds the frozen one-shot import pool, so reading from it
// would never reflect new daily-topup additions. Rendered variants
// (file_path NOT LIKE 'original/%') are excluded so the footer shows
// "image library size", not "R2 footprint".
func (d *DB) LibrarySummary(ctx context.Context) (LibrarySummary, error) {
	var s LibrarySummary
	row := d.QueryRowContext(ctx, `
		SELECT COUNT(*), COALESCE(SUM(file_size),0)
		  FROM r2_uploads
		 WHERE file_path LIKE 'original/%'`)
	if err := row.Scan(&s.TotalImages, &s.TotalBytes); err != nil {
		return s, err
	}
	// Extract type from file_path = "original/{type}/{sha1}.jpg".
	// SUBSTR(p, 10, INSTR(SUBSTR(p,10), '/') - 1)
	//   pos 10 = first char after "original/"
	//   then take chars up to the next '/'.
	rows, err := d.QueryContext(ctx, `
		SELECT SUBSTR(file_path, 10, INSTR(SUBSTR(file_path, 10), '/') - 1) AS typ,
		       COUNT(*)
		  FROM r2_uploads
		 WHERE file_path LIKE 'original/%'
		 GROUP BY typ`)
	if err != nil {
		return s, err
	}
	defer rows.Close()
	s.ByType = make(map[string]int64)
	for rows.Next() {
		var t string
		var n int64
		if err := rows.Scan(&t, &n); err != nil {
			return s, err
		}
		s.ByType[t] = n
	}
	return s, rows.Err()
}

// SpecStat is one (type × resolution) request profile rollup.
type SpecStat struct {
	Type      string
	Width     int
	Height    int
	Keyword   string
	Requests  int64
	Views     int64
	Downloads int64
}

// CategoryStat aggregates request/view counts for one category across all sizes.
type CategoryStat struct {
	Type     string
	Requests int64
	Views    int64
}

// GlobalStats is the whole-site request rollup shown on /stats. All numbers
// come from request_profiles, which tracks demand per (type, size, keyword)
// signature rather than per individual file.
type GlobalStats struct {
	TotalRequests  int64
	TotalViews     int64
	TotalDownloads int64
	ProfileCount   int64
	TopSpecs       []SpecStat     // most-requested (type × resolution) profiles
	TopCategories  []CategoryStat // most-requested categories
}

// GlobalStats reads request_profiles and returns site-wide totals plus the
// top-N specs and categories by request volume.
func (d *DB) GlobalStats(ctx context.Context, topN int) (GlobalStats, error) {
	if topN <= 0 {
		topN = 20
	}
	var g GlobalStats

	row := d.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(request_count),0), COALESCE(SUM(view_count),0),
		       COALESCE(SUM(download_count),0), COUNT(*)
		  FROM request_profiles`)
	if err := row.Scan(&g.TotalRequests, &g.TotalViews, &g.TotalDownloads, &g.ProfileCount); err != nil {
		return g, err
	}

	specRows, err := d.QueryContext(ctx, `
		SELECT type, width, height, keyword, request_count, view_count, download_count
		  FROM request_profiles
		 ORDER BY request_count DESC, view_count DESC
		 LIMIT ?`, topN)
	if err != nil {
		return g, err
	}
	defer specRows.Close()
	for specRows.Next() {
		var s SpecStat
		if err := specRows.Scan(&s.Type, &s.Width, &s.Height, &s.Keyword,
			&s.Requests, &s.Views, &s.Downloads); err != nil {
			return g, err
		}
		g.TopSpecs = append(g.TopSpecs, s)
	}
	if err := specRows.Err(); err != nil {
		return g, err
	}

	catRows, err := d.QueryContext(ctx, `
		SELECT type, SUM(request_count), SUM(view_count)
		  FROM request_profiles
		 GROUP BY type
		 ORDER BY SUM(request_count) DESC
		 LIMIT ?`, topN)
	if err != nil {
		return g, err
	}
	defer catRows.Close()
	for catRows.Next() {
		var c CategoryStat
		if err := catRows.Scan(&c.Type, &c.Requests, &c.Views); err != nil {
			return g, err
		}
		g.TopCategories = append(g.TopCategories, c)
	}
	return g, catRows.Err()
}
