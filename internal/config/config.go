package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	HTTPAddr    string
	SiteBaseURL string

	DBPath       string
	ImagesDir    string
	TemplatesDir string

	// ---- Branding (open-source friendly: every visible string is overridable) ----
	SiteName      string // shows in <title>, brand mark, og:site_name
	SiteTagline   string // optional subtitle (e.g. "图得"); empty hides it
	DMCAEmail     string // empty hides the footer email link
	CopyrightYear int    // shown in footer; 0 means "use current year"
	OGImageURL    string // og:image, empty falls back to /assets/logo.png

	// External CDN URL — open-source users can swap. @font-face declarations
	// for body/code fonts are bundled inside main.min.css and don't need a
	// separate link.
	FontAwesomeCSSURL string // empty disables FontAwesome (icons won't render)

	// Optional analytics (Umami-compatible). Empty disables the script tag.
	AnalyticsScriptURL string
	AnalyticsWebsiteID string

	R2Endpoint        string
	R2AccessKeyID     string
	R2SecretAccessKey string
	R2Bucket          string
	R2Region          string
	R2CDNBaseURL      string
	R2RedirectDirect  bool
	R2UploadWorkers   int
	R2UploadVerify    bool // HEAD after PUT to confirm size; doubles request count when on

	PixabayAPIKey        string
	PixabayBackupAPIKey  string
	PixabayMinIntervalMS int
	PixabayCooldownSec   int
	PixabayPerPage       int

	PexelsAPIKey        string
	PexelsMinIntervalMS int
	PexelsCooldownSec   int
	PexelsPerPage       int
	PexelsMinWidth      int // skip Pexels hits whose original is narrower than this

	// CC / no-key providers. Wikimedia and Openverse work without credentials;
	// Openverse client creds and the Unsplash key only raise limits / unlock the
	// source. Empty disables the keyed source (Unsplash) or runs anonymously
	// (Openverse).
	WikimediaEnabled      bool
	WikimediaUserAgent    string
	OpenverseEnabled      bool
	OpenverseClientID     string
	OpenverseClientSecret string
	UnsplashAccessKey     string

	DefaultFormat  string
	EnabledFormats []string // explicit allow-list, e.g. ["webp","avif"] or ["webp"]
	WebPQuality    int
	AVIFQuality    int
	AVIFSpeed      int
	AVIFTimeoutSec int
	MinDim         int
	MaxDim         int

	LocalRenderCacheTTLHours int
	InitialPrefetchCount     int // first visit: how many to fetch
	PerVisitRefetch          int // each subsequent visit: how many to top up
	DailyTopupIncrement      int
	DailyTopupMaxPerType     int

	// topup-types: random count in [TopupTypesMinPerType, TopupTypesMaxPerType]
	// per category each daily run. Used when the CLI omits --min/--max.
	TopupTypesMinPerType int
	TopupTypesMaxPerType int

	LogLevel     string
	AllowedTypes []string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		HTTPAddr:    envStr("HTTP_ADDR", ":8080"),
		SiteBaseURL: strings.TrimRight(envStr("SITE_BASE_URL", "http://localhost:8080"), "/"),

		DBPath:       envStr("DB_PATH", "database/imget.sqlite"),
		ImagesDir:    envStr("IMAGES_DIR", "images"),
		TemplatesDir: envStr("TEMPLATES_DIR", ""),

		SiteName:      envStr("SITE_NAME", "imget"),
		SiteTagline:   envStr("SITE_TAGLINE", ""),
		DMCAEmail:     envStr("DMCA_EMAIL", ""),
		CopyrightYear: envInt("COPYRIGHT_YEAR", 0),
		OGImageURL:    envStr("OG_IMAGE_URL", ""),

		FontAwesomeCSSURL:  envStr("FONTAWESOME_CSS_URL", ""),
		AnalyticsScriptURL: envStr("ANALYTICS_SCRIPT_URL", ""),
		AnalyticsWebsiteID: envStr("ANALYTICS_WEBSITE_ID", ""),

		R2Endpoint:        strings.TrimRight(envStr("R2_ENDPOINT", ""), "/"),
		R2AccessKeyID:     envStr("R2_ACCESS_KEY_ID", ""),
		R2SecretAccessKey: envStr("R2_SECRET_ACCESS_KEY", ""),
		R2Bucket:          envStr("R2_BUCKET", ""),
		R2Region:          envStr("R2_REGION", "auto"),
		R2CDNBaseURL:      strings.TrimRight(envStr("R2_CDN_BASE_URL", ""), "/"),
		R2RedirectDirect:  envBool("R2_REDIRECT_DIRECT", true),
		R2UploadWorkers:   envInt("R2_UPLOAD_WORKERS", 8),
		R2UploadVerify:    envBool("R2_UPLOAD_VERIFY", false),

		PixabayAPIKey:        envStr("PIXABAY_API_KEY", ""),
		PixabayBackupAPIKey:  envStr("PIXABAY_API_KEY_BACKUP", ""),
		PixabayMinIntervalMS: envInt("PIXABAY_MIN_INTERVAL_MS", 900),
		PixabayCooldownSec:   envInt("PIXABAY_COOLDOWN_SEC", 300),
		PixabayPerPage:       envInt("PIXABAY_PER_PAGE", 50),

		PexelsAPIKey:        envStr("PEXELS_API_KEY", ""),
		PexelsMinIntervalMS: envInt("PEXELS_MIN_INTERVAL_MS", 1000),
		PexelsCooldownSec:   envInt("PEXELS_COOLDOWN_SEC", 300),
		PexelsPerPage:       envInt("PEXELS_PER_PAGE", 30),
		PexelsMinWidth:      envInt("PEXELS_MIN_WIDTH", 1920),

		WikimediaEnabled:      envBool("WIKIMEDIA_ENABLED", true),
		WikimediaUserAgent:    envStr("WIKIMEDIA_USER_AGENT", ""),
		OpenverseEnabled:      envBool("OPENVERSE_ENABLED", true),
		OpenverseClientID:     envStr("OPENVERSE_CLIENT_ID", ""),
		OpenverseClientSecret: envStr("OPENVERSE_CLIENT_SECRET", ""),
		UnsplashAccessKey:     envStr("UNSPLASH_ACCESS_KEY", ""),

		DefaultFormat:  strings.ToLower(envStr("DEFAULT_FORMAT", "webp")),
		EnabledFormats: parseList(envStr("ENABLED_FORMATS", "webp,avif")),
		WebPQuality:    envInt("WEBP_QUALITY", 85),
		AVIFQuality:    envInt("AVIF_QUALITY", 60),
		AVIFSpeed:      envInt("AVIF_SPEED", 6),
		AVIFTimeoutSec: envInt("AVIF_TIMEOUT_SEC", 20),
		MinDim:         envInt("MIN_DIM", 50),
		MaxDim:         envInt("MAX_DIM", 4000),

		LocalRenderCacheTTLHours: envInt("LOCAL_RENDER_CACHE_TTL_HOURS", 168),
		InitialPrefetchCount:     envInt("INITIAL_PREFETCH_COUNT", 5),
		PerVisitRefetch:          envInt("PER_VISIT_REFETCH", 1),
		DailyTopupIncrement:      envInt("DAILY_TOPUP_INCREMENT", 10),
		DailyTopupMaxPerType:     envInt("DAILY_TOPUP_MAX_PER_TYPE", 1000),

		TopupTypesMinPerType: envInt("TOPUP_TYPES_MIN_PER_TYPE", 5),
		TopupTypesMaxPerType: envInt("TOPUP_TYPES_MAX_PER_TYPE", 10),

		LogLevel:     strings.ToLower(envStr("LOG_LEVEL", "info")),
		AllowedTypes: parseList(envStr("ALLOWED_TYPES", "")),
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) R2Enabled() bool {
	return c.R2Endpoint != "" && c.R2AccessKeyID != "" && c.R2SecretAccessKey != "" && c.R2Bucket != ""
}

// FormatEnabled reports whether the operator allows a given output format
// (independent of whether the encoder binary is installed).
func (c *Config) FormatEnabled(f string) bool {
	if len(c.EnabledFormats) == 0 {
		return true
	}
	f = strings.ToLower(strings.TrimSpace(f))
	for _, x := range c.EnabledFormats {
		if strings.ToLower(strings.TrimSpace(x)) == f {
			return true
		}
	}
	return false
}

func (c *Config) AbsImagesDir() string { return absify(c.ImagesDir) }
func (c *Config) AbsDBPath() string    { return absify(c.DBPath) }
func (c *Config) AbsTemplatesDir() string {
	if c.TemplatesDir == "" {
		return ""
	}
	return absify(c.TemplatesDir)
}

func (c *Config) validate() error {
	if c.MinDim < 1 || c.MaxDim < c.MinDim {
		return fmt.Errorf("invalid MIN_DIM/MAX_DIM: %d / %d", c.MinDim, c.MaxDim)
	}
	if c.WebPQuality < 1 || c.WebPQuality > 100 {
		return fmt.Errorf("WEBP_QUALITY must be 1..100")
	}
	if c.AVIFQuality < 1 || c.AVIFQuality > 100 {
		return fmt.Errorf("AVIF_QUALITY must be 1..100")
	}
	if c.DefaultFormat != "webp" && c.DefaultFormat != "avif" {
		return fmt.Errorf("DEFAULT_FORMAT must be webp or avif")
	}
	return nil
}

func envStr(k, def string) string {
	if v, ok := os.LookupEnv(k); ok && v != "" {
		return v
	}
	return def
}

func envInt(k string, def int) int {
	if v := os.Getenv(k); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envBool(k string, def bool) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(k)))
	switch v {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	}
	return def
}

func parseList(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func absify(p string) string {
	if p == "" {
		return ""
	}
	if filepath.IsAbs(p) {
		return p
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	return abs
}
