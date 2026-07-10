package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"

	"imget/internal/config"
	"imget/internal/db"
	"imget/internal/encoder"
	"imget/internal/imgpipe"
	"imget/internal/jobs"
	"imget/internal/metrics"
	"imget/internal/r2"
	"imget/internal/server"
	"imget/internal/source"
)

var metricsRegistered sync.Once

func main() {
	defer encoder.Shutdown() // safe even if Init was never called

	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "serve":
		exit(runServe(args))
	case "daily-topup":
		exit(runDailyTopup(args))
	case "topup-types":
		exit(runTopupTypes(args))
	case "r2-sync":
		exit(runR2Sync(args))
	case "cleanup-cache":
		exit(runCleanup(args))
	case "import-r2":
		exit(runImportR2(args))
	case "r2-prune-variants":
		exit(runPruneVariants(args))
	case "r2-prune-orphans":
		exit(runPruneOrphans(args))
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", cmd)
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `imget — img.et server (Go edition)

Usage:
  imget <command> [flags]

Commands:
  serve            Start the HTTP server
  daily-topup      Refresh request_profiles by fetching N new images each
  topup-types      Fetch random 5-10 (configurable) new originals per category
  r2-sync          Upload any disk-resident files missing from R2
  cleanup-cache    Delete rendered variants older than TTL (originals safe)
  import-r2        One-shot: list R2 bucket + populate source_images table
  r2-prune-variants  Delete every R2 object NOT under original/ (--yes for real)
  r2-prune-orphans   Delete R2 objects not referenced by r2_uploads/source_images (--yes for real)

All configuration comes from environment variables (see .env.example).`)
}

// =====================================================================
// serve
// =====================================================================

func runServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	addr := fs.String("addr", "", "override HTTP_ADDR")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, log, err := boot()
	if err != nil {
		return err
	}
	sqlDB, err := db.Open(cfg.AbsDBPath())
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	pipe, r2cli, err := buildPipeline(context.Background(), cfg, sqlDB, log)
	if err != nil {
		return err
	}
	_ = r2cli
	defer pipe.Close()

	srv, err := server.New(server.Deps{
		Cfg: cfg, DB: sqlDB, Pipeline: pipe, Logger: log,
	})
	if err != nil {
		return err
	}

	listenAddr := cfg.HTTPAddr
	if *addr != "" {
		listenAddr = *addr
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return srv.ListenAndServe(ctx, listenAddr)
}

// =====================================================================
// daily-topup
// =====================================================================

func runDailyTopup(args []string) error {
	fs := flag.NewFlagSet("daily-topup", flag.ExitOnError)
	increment := fs.Int("n", 0, "images to add per profile (defaults to DAILY_TOPUP_INCREMENT)")
	maxPerType := fs.Int("max-per-type", 0, "stop fetching for a type once it has this many sources (0 = no cap)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if len(fs.Args()) > 0 {
		// positional shortcut: imget daily-topup 25
		if n, err := strconv.Atoi(fs.Arg(0)); err == nil {
			*increment = n
		}
	}

	return withPipeline(false, func(ctx context.Context, cfg *config.Config, sqlDB *db.DB, pipe *imgpipe.Pipeline, log *slog.Logger) error {
		inc := *increment
		if inc == 0 {
			inc = cfg.DailyTopupIncrement
		}
		maxT := *maxPerType
		if maxT == 0 {
			maxT = cfg.DailyTopupMaxPerType
		}
		return jobs.DailyTopup(ctx, log, sqlDB, pipe, inc, maxT)
	})
}

// =====================================================================
// topup-types
// =====================================================================

func runTopupTypes(args []string) error {
	fs := flag.NewFlagSet("topup-types", flag.ExitOnError)
	minN := fs.Int("min", 0, "min new originals per category (0 = use TOPUP_TYPES_MIN_PER_TYPE)")
	maxN := fs.Int("max", 0, "max new originals per category (0 = use TOPUP_TYPES_MAX_PER_TYPE)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	return withPipeline(false, func(ctx context.Context, cfg *config.Config, sqlDB *db.DB, pipe *imgpipe.Pipeline, log *slog.Logger) error {
		mn, mx := *minN, *maxN
		if mn == 0 {
			mn = cfg.TopupTypesMinPerType
		}
		if mx == 0 {
			mx = cfg.TopupTypesMaxPerType
		}
		return jobs.TopupByType(ctx, log, sqlDB, pipe, mn, mx)
	})
}

// =====================================================================
// r2-sync
// =====================================================================

func runR2Sync(args []string) error {
	fs := flag.NewFlagSet("r2-sync", flag.ExitOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	return withPipeline(true, func(ctx context.Context, cfg *config.Config, sqlDB *db.DB, pipe *imgpipe.Pipeline, log *slog.Logger) error {
		return jobs.R2Sync(ctx, log, cfg, sqlDB, pipe)
	})
}

// =====================================================================
// cleanup-cache
// =====================================================================

func runCleanup(args []string) error {
	fs := flag.NewFlagSet("cleanup-cache", flag.ExitOnError)
	hours := fs.Int("hours", 0, "TTL hours (defaults to LOCAL_RENDER_CACHE_TTL_HOURS)")
	dryRun := fs.Bool("dry-run", false, "report only, don't delete")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, log, err := boot()
	if err != nil {
		return err
	}
	sqlDB, err := db.Open(cfg.AbsDBPath())
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return jobs.CleanupCache(ctx, log, cfg, sqlDB, *hours, *dryRun)
}

// =====================================================================
// import-r2
// =====================================================================

func runImportR2(args []string) error {
	fs := flag.NewFlagSet("import-r2", flag.ExitOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, log, err := boot()
	if err != nil {
		return err
	}
	if !cfg.R2Enabled() {
		return fmt.Errorf("import-r2: R2 is not configured")
	}
	sqlDB, err := db.Open(cfg.AbsDBPath())
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	r2cli, err := newR2Client(ctx, cfg)
	if err != nil {
		return err
	}
	return jobs.ImportR2(ctx, log, sqlDB, r2cli)
}

// =====================================================================
// r2-prune-variants
// =====================================================================

func runPruneOrphans(args []string) error {
	fs := flag.NewFlagSet("r2-prune-orphans", flag.ExitOnError)
	yes := fs.Bool("yes", false, "actually delete (default is dry-run)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, log, err := boot()
	if err != nil {
		return err
	}
	if !cfg.R2Enabled() {
		return fmt.Errorf("r2-prune-orphans: R2 is not configured")
	}
	sqlDB, err := db.Open(cfg.AbsDBPath())
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	r2cli, err := newR2Client(ctx, cfg)
	if err != nil {
		return err
	}
	dryRun := !*yes
	if dryRun {
		log.Warn("DRY-RUN mode: nothing will be deleted. Pass --yes to actually run.")
	} else {
		log.Warn("DELETE mode: orphan R2 objects (not in DB) will be removed.")
	}
	return jobs.PruneR2Orphans(ctx, log, sqlDB, r2cli, dryRun)
}

func runPruneVariants(args []string) error {
	fs := flag.NewFlagSet("r2-prune-variants", flag.ExitOnError)
	yes := fs.Bool("yes", false, "actually delete (default is dry-run)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, log, err := boot()
	if err != nil {
		return err
	}
	if !cfg.R2Enabled() {
		return fmt.Errorf("r2-prune-variants: R2 is not configured")
	}
	sqlDB, err := db.Open(cfg.AbsDBPath())
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	r2cli, err := newR2Client(ctx, cfg)
	if err != nil {
		return err
	}
	dryRun := !*yes
	if dryRun {
		log.Warn("DRY-RUN mode: nothing will be deleted. Pass --yes to actually run.")
	} else {
		log.Warn("DELETE mode: ALL non-original objects will be removed from the bucket.")
	}
	return jobs.PruneR2Variants(ctx, log, sqlDB, r2cli, dryRun)
}

// =====================================================================
// shared bootstrap
// =====================================================================

func boot() (*config.Config, *slog.Logger, error) {
	// Initialize libvips before any image work happens. Cheap to call
	// repeatedly (Init is sync.Once-guarded internally).
	encoder.Init()

	// Register Prometheus metrics once per process (panics if called twice).
	metricsRegistered.Do(metrics.MustRegister)

	cfg, err := config.Load()
	if err != nil {
		return nil, nil, err
	}
	level := slog.LevelInfo
	switch cfg.LogLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
	return cfg, log, nil
}

func buildPipeline(ctx context.Context, cfg *config.Config, sqlDB *db.DB, log *slog.Logger) (*imgpipe.Pipeline, *r2.Client, error) {
	enc, err := encoder.New(encoder.Options{
		WebPQuality: cfg.WebPQuality,
		AVIFQuality: cfg.AVIFQuality,
		AVIFSpeed:   cfg.AVIFSpeed,
	})
	if err != nil {
		return nil, nil, err
	}

	var r2cli *r2.Client
	if cfg.R2Enabled() {
		c, err := newR2Client(ctx, cfg)
		if err != nil {
			return nil, nil, err
		}
		r2cli = c
	} else {
		log.Warn("R2 not configured; running in local-only mode")
	}

	providers := buildProviders(cfg)

	pipe, err := imgpipe.New(ctx, imgpipe.Options{
		Config:    cfg,
		DB:        sqlDB,
		Encoder:   enc,
		R2:        r2cli,
		Providers: providers,
		Logger:    log,
	})
	if err != nil {
		return nil, nil, err
	}
	return pipe, r2cli, nil
}

// newR2Client builds an R2 client from config — the same six-field Options
// literal that import-r2 / r2-prune-* / buildPipeline all need.
func newR2Client(ctx context.Context, cfg *config.Config) (*r2.Client, error) {
	return r2.New(ctx, r2.Options{
		Endpoint:        cfg.R2Endpoint,
		AccessKeyID:     cfg.R2AccessKeyID,
		SecretAccessKey: cfg.R2SecretAccessKey,
		Bucket:          cfg.R2Bucket,
		Region:          cfg.R2Region,
		CDNBaseURL:      cfg.R2CDNBaseURL,
	})
}

// withPipeline runs fn with a fully-booted pipeline (config, DB, signal-aware
// context, pipeline) and tears everything down afterwards. It collapses the
// boot→open→buildPipeline→defer-Close boilerplate shared by the topup/sync
// subcommands. Pass needR2=true to fail fast when R2 isn't configured.
func withPipeline(needR2 bool, fn func(ctx context.Context, cfg *config.Config, sqlDB *db.DB, pipe *imgpipe.Pipeline, log *slog.Logger) error) error {
	cfg, log, err := boot()
	if err != nil {
		return err
	}
	if needR2 && !cfg.R2Enabled() {
		return fmt.Errorf("R2 is not configured (set R2_ENDPOINT/...)")
	}
	sqlDB, err := db.Open(cfg.AbsDBPath())
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pipe, _, err := buildPipeline(ctx, cfg, sqlDB, log)
	if err != nil {
		return err
	}
	defer pipe.Close()

	return fn(ctx, cfg, sqlDB, pipe, log)
}

func buildProviders(cfg *config.Config) []source.Provider {
	out := make([]source.Provider, 0, 3)
	// Pexels first: returns the literal `src.original` (truly 4K/5K originals)
	// when a key is configured. Pixabay free tier caps URLs at 1280px, so we
	// only fall through to it when Pexels is unset or returns nothing.
	if cfg.PexelsAPIKey != "" {
		out = append(out, source.NewPexels(source.PexelsConfig{
			APIKey:        cfg.PexelsAPIKey,
			MinIntervalMS: cfg.PexelsMinIntervalMS,
			CooldownSec:   cfg.PexelsCooldownSec,
			PerPage:       cfg.PexelsPerPage,
			MinWidth:      cfg.PexelsMinWidth,
		}))
	}
	if cfg.PixabayAPIKey != "" || cfg.PixabayBackupAPIKey != "" {
		out = append(out, source.NewPixabay(source.PixabayConfig{
			APIKey:        cfg.PixabayAPIKey,
			BackupAPIKey:  cfg.PixabayBackupAPIKey,
			MinIntervalMS: cfg.PixabayMinIntervalMS,
			CooldownSec:   cfg.PixabayCooldownSec,
			PerPage:       cfg.PixabayPerPage,
		}))
	}
	// Unsplash (keyed, true originals) sits just after the other keyed sources.
	if cfg.UnsplashAccessKey != "" {
		out = append(out, source.NewUnsplash(source.UnsplashConfig{
			AccessKey: cfg.UnsplashAccessKey,
		}))
	}
	// CC / no-key providers — the volume backstop for the topup FetchAll union.
	if cfg.OpenverseEnabled {
		out = append(out, source.NewOpenverse(source.OpenverseConfig{
			ClientID:     cfg.OpenverseClientID,
			ClientSecret: cfg.OpenverseClientSecret,
		}))
	}
	if cfg.WikimediaEnabled {
		out = append(out, source.NewWikimedia(source.WikimediaConfig{
			UserAgent: cfg.WikimediaUserAgent,
		}))
	}
	// Bing daily wallpaper — scenic-only, contributes to the landscape bucket.
	if cfg.BingEnabled {
		out = append(out, source.NewBing(source.BingConfig{
			Market: cfg.BingMarket,
		}))
	}
	out = append(out, source.NewPicsum())
	return out
}

func exit(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
