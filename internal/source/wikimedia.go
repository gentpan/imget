package source

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// WikimediaConfig configures the Wikimedia Commons provider. Commons exposes
// ~100M freely-licensed media files through the MediaWiki API and needs no API
// key — only a descriptive User-Agent (Wikimedia policy). It is the highest-
// volume no-key source, so it's a natural backstop once the keyed providers
// (Pexels/Pixabay) exhaust their accessible window for a keyword.
type WikimediaConfig struct {
	UserAgent     string // required by Wikimedia; falls back to a sane default
	MinIntervalMS int
	CooldownSec   int
	PerPage       int
	HTTPClient    *http.Client
}

type Wikimedia struct {
	cfg WikimediaConfig
	hc  *http.Client
	lim *Limiter
}

func NewWikimedia(cfg WikimediaConfig) *Wikimedia {
	if cfg.MinIntervalMS == 0 {
		cfg.MinIntervalMS = 500
	}
	if cfg.CooldownSec == 0 {
		cfg.CooldownSec = 120
	}
	if cfg.PerPage == 0 {
		cfg.PerPage = 40
	}
	if strings.TrimSpace(cfg.UserAgent) == "" {
		cfg.UserAgent = "imget-go/1.0 (https://img.et; image aggregation)"
	}
	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: 20 * time.Second}
	}
	return &Wikimedia{cfg: cfg, hc: hc, lim: NewLimiter(cfg.MinIntervalMS, cfg.CooldownSec)}
}

func (w *Wikimedia) Name() string { return "wikimedia" }

// Configured is always true — Commons needs no credentials.
func (w *Wikimedia) Configured() bool { return true }

// FetchURLs queries the Commons MediaWiki API, searching the File namespace and
// returning direct upload.wikimedia.org URLs for raster images.
func (w *Wikimedia) FetchURLs(ctx context.Context, req Request) ([]string, error) {
	if err := w.lim.Wait(ctx); err != nil {
		return nil, err
	}

	per := req.Count
	if per <= 0 {
		per = w.cfg.PerPage
	}
	if per < 5 {
		per = 5
	}
	if per > 50 { // Commons search generator caps gsrlimit at 50 for most users.
		per = 50
	}

	q := strings.TrimSpace(ResolveKeyword(req.Type, req.Keyword))
	if q == "" {
		q = NormalizeType(req.Type)
	}

	v := url.Values{}
	v.Set("action", "query")
	v.Set("format", "json")
	v.Set("generator", "search")
	v.Set("gsrsearch", q)
	v.Set("gsrnamespace", "6") // File:
	v.Set("gsrlimit", strconv.Itoa(per))
	if req.Page > 1 {
		v.Set("gsroffset", strconv.Itoa((req.Page-1)*per))
	}
	v.Set("prop", "imageinfo")
	v.Set("iiprop", "url|size|mime")

	endpoint := "https://commons.wikimedia.org/w/api.php?" + v.Encode()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("User-Agent", w.cfg.UserAgent)

	resp, err := w.hc.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("wikimedia: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		w.lim.MarkCooldown()
		return nil, fmt.Errorf("wikimedia: 429 rate limited")
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 500))
		return nil, fmt.Errorf("wikimedia: HTTP %d: %s", resp.StatusCode, string(body))
	}

	var payload struct {
		Query struct {
			Pages map[string]struct {
				ImageInfo []struct {
					URL    string `json:"url"`
					Width  int    `json:"width"`
					Height int    `json:"height"`
					Mime   string `json:"mime"`
				} `json:"imageinfo"`
			} `json:"pages"`
		} `json:"query"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("wikimedia: decode: %w", err)
	}

	out := make([]string, 0, len(payload.Query.Pages))
	for _, page := range payload.Query.Pages {
		if len(page.ImageInfo) == 0 {
			continue
		}
		ii := page.ImageInfo[0]
		if !wikimediaWantMime(ii.Mime) {
			continue
		}
		if req.Width > 0 && ii.Width > 0 && ii.Width < req.Width {
			continue
		}
		if ii.URL != "" {
			out = append(out, ii.URL)
		}
	}
	return out, nil
}

// wikimediaWantMime keeps only the raster formats the pipeline can transcode,
// dropping SVG/TIFF/PDF/GIF and other non-photographic assets Commons returns.
func wikimediaWantMime(mime string) bool {
	switch strings.ToLower(strings.TrimSpace(mime)) {
	case "image/jpeg", "image/png", "image/webp":
		return true
	}
	return false
}
