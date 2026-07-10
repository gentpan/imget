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

// UnsplashConfig configures the Unsplash provider. Unsplash returns true
// full-resolution originals (`urls.raw`) but requires a (free) Access Key and
// is disabled when the key is empty. Demo apps are capped at 50 req/hour;
// approved production apps at 5000 req/hour.
type UnsplashConfig struct {
	AccessKey     string
	MinIntervalMS int
	CooldownSec   int
	PerPage       int
	HTTPClient    *http.Client
}

type Unsplash struct {
	cfg UnsplashConfig
	hc  *http.Client
	lim *Limiter
}

func NewUnsplash(cfg UnsplashConfig) *Unsplash {
	if cfg.MinIntervalMS == 0 {
		cfg.MinIntervalMS = 1500
	}
	if cfg.CooldownSec == 0 {
		cfg.CooldownSec = 600
	}
	if cfg.PerPage == 0 {
		cfg.PerPage = 30
	}
	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: 20 * time.Second}
	}
	return &Unsplash{cfg: cfg, hc: hc, lim: NewLimiter(cfg.MinIntervalMS, cfg.CooldownSec)}
}

func (u *Unsplash) Name() string { return "unsplash" }

func (u *Unsplash) Configured() bool { return u.cfg.AccessKey != "" }

// FetchURLs queries /search/photos and returns each hit's `urls.raw` (the
// uncompressed original asset).
func (u *Unsplash) FetchURLs(ctx context.Context, req Request) ([]string, error) {
	if !u.Configured() {
		return nil, nil
	}
	if err := u.lim.Wait(ctx); err != nil {
		return nil, err
	}

	per := req.Count
	if per <= 0 {
		per = u.cfg.PerPage
	}
	if per < 5 {
		per = 5
	}
	if per > 30 {
		per = 30
	}

	q := strings.TrimSpace(ResolveKeyword(req.Type, req.Keyword))
	if q == "" {
		q = NormalizeType(req.Type)
	}

	v := url.Values{}
	v.Set("query", q)
	v.Set("per_page", strconv.Itoa(per))
	if req.Page > 0 {
		v.Set("page", strconv.Itoa(req.Page))
	}
	if o := unsplashOrientation(req.Type); o != "" {
		v.Set("orientation", o)
	}

	endpoint := "https://api.unsplash.com/search/photos?" + v.Encode()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Accept-Version", "v1")
	httpReq.Header.Set("Authorization", "Client-ID "+u.cfg.AccessKey)
	httpReq.Header.Set("User-Agent", "imget-go/1.0")

	resp, err := u.hc.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("unsplash: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusForbidden {
		u.lim.MarkCooldown()
		return nil, fmt.Errorf("unsplash: %d rate limited", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 500))
		return nil, fmt.Errorf("unsplash: HTTP %d: %s", resp.StatusCode, string(body))
	}

	var payload struct {
		Results []struct {
			Width  int               `json:"width"`
			Height int               `json:"height"`
			URLs   map[string]string `json:"urls"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("unsplash: decode: %w", err)
	}

	out := make([]string, 0, len(payload.Results))
	for _, r := range payload.Results {
		if req.Width > 0 && r.Width > 0 && r.Width < req.Width {
			continue
		}
		if uu := pickUnsplashURL(r.URLs); uu != "" {
			out = append(out, uu)
		}
	}
	return out, nil
}

func pickUnsplashURL(urls map[string]string) string {
	for _, key := range []string{"raw", "full", "regular"} {
		if v, ok := urls[key]; ok && v != "" {
			return v
		}
	}
	return ""
}

func unsplashOrientation(typ string) string {
	switch NormalizeType(typ) {
	case "beauty", "wedding":
		return "portrait"
	case "banner", "landscape", "city", "nature", "travel", "architecture", "space":
		return "landscape"
	}
	return ""
}
