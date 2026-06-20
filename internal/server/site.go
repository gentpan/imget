package server

import (
	"strings"
	"time"

	"imget/internal/config"
	"imget/internal/encoder"
	"imget/internal/source"
)

// assetVersion is the single source of truth for the static-asset cache-buster
// (?v=...). Bump it whenever main.min.css/js change; templates read it via
// {{ .Site.AssetVersion }} instead of hardcoding the value per <link>/<script>.
const assetVersion = "2026062005"

// SiteContext is the slice of config that every page template needs.
// It is built once at server startup and reused for every render.
type SiteContext struct {
	// AssetVersion is the static-asset cache-buster appended to CSS/JS URLs.
	AssetVersion string

	Name              string
	Tagline           string
	BaseURL           string
	DMCAEmail         string
	CopyrightYear     int
	OGImageURL        string
	FontAwesomeCSSURL string
	AnalyticsScript   string
	AnalyticsID       string

	// Format flags — true iff format is both ENABLED in config AND has a working encoder.
	SupportsWebP bool
	SupportsAvif bool

	// Allowed types (filtered to the operator's whitelist if any).
	Types []string

	// Fetch tuning (rendered into the home-page copy as concrete numbers).
	InitialPrefetchCount int
	PerVisitRefetch      int
	DailyTopupIncrement  int
	DailyTopupMaxPerType int
	TopupTypesMinPerType int
	TopupTypesMaxPerType int
}

// HasAnalytics reports whether to render the analytics <script> tag.
func (s SiteContext) HasAnalytics() bool {
	return s.AnalyticsScript != ""
}

// HasFontAwesome reports whether to render the FontAwesome CSS link.
func (s SiteContext) HasFontAwesome() bool {
	return s.FontAwesomeCSSURL != ""
}

// HasDMCAEmail reports whether to render the DMCA email link in the footer.
func (s SiteContext) HasDMCAEmail() bool {
	return s.DMCAEmail != ""
}

// FullName combines Name + Tagline ("imget" or "imget 图得").
func (s SiteContext) FullName() string {
	if s.Tagline == "" {
		return s.Name
	}
	return s.Name + " " + s.Tagline
}

// BaseHost strips the scheme from BaseURL — e.g. for inline references like
// "直接在域名 <code>img.et</code>".
func (s SiteContext) BaseHost() string {
	u := s.BaseURL
	u = strings.TrimPrefix(u, "https://")
	u = strings.TrimPrefix(u, "http://")
	return strings.TrimRight(u, "/")
}

// OGImage returns the og:image URL. Falls back to /assets/logo.png.
func (s SiteContext) OGImage() string {
	if s.OGImageURL != "" {
		return s.OGImageURL
	}
	return s.BaseURL + "/assets/logo.png"
}

func newSiteContext(cfg *config.Config, enc *encoder.Encoder) SiteContext {
	year := cfg.CopyrightYear
	if year == 0 {
		year = time.Now().Year()
	}

	supportsWebP := cfg.FormatEnabled("webp") && enc != nil && enc.Available(encoder.FormatWebP)
	supportsAvif := cfg.FormatEnabled("avif") && enc != nil && enc.Available(encoder.FormatAVIF)

	types := cfg.AllowedTypes
	if len(types) == 0 {
		types = source.AllowedTypes
	}

	return SiteContext{
		AssetVersion:         assetVersion,
		Name:                 cfg.SiteName,
		Tagline:              cfg.SiteTagline,
		BaseURL:              cfg.SiteBaseURL,
		DMCAEmail:            cfg.DMCAEmail,
		CopyrightYear:        year,
		OGImageURL:           cfg.OGImageURL,
		FontAwesomeCSSURL:    cfg.FontAwesomeCSSURL,
		AnalyticsScript:      cfg.AnalyticsScriptURL,
		AnalyticsID:          cfg.AnalyticsWebsiteID,
		SupportsWebP:         supportsWebP,
		SupportsAvif:         supportsAvif,
		Types:                types,
		InitialPrefetchCount: cfg.InitialPrefetchCount,
		PerVisitRefetch:      cfg.PerVisitRefetch,
		DailyTopupIncrement:  cfg.DailyTopupIncrement,
		DailyTopupMaxPerType: cfg.DailyTopupMaxPerType,
		TopupTypesMinPerType: cfg.TopupTypesMinPerType,
		TopupTypesMaxPerType: cfg.TopupTypesMaxPerType,
	}
}
