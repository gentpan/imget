package source

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// PexelsConfig configures the Pexels provider. Pexels returns the truly
// original-resolution URL (`src.original`), so it's preferred over Pixabay
// (which on the free tier caps at 1280px) when the goal is 4K+ originals.
type PexelsConfig struct {
	APIKey        string
	MinIntervalMS int
	CooldownSec   int
	PerPage       int
	MinWidth      int // drop hits with `width` below this; 0 = no filter
	HTTPClient    *http.Client
}

type Pexels struct {
	cfg PexelsConfig
	hc  *http.Client
	lim *Limiter
}

func NewPexels(cfg PexelsConfig) *Pexels {
	if cfg.MinIntervalMS == 0 {
		cfg.MinIntervalMS = 1000
	}
	if cfg.CooldownSec == 0 {
		cfg.CooldownSec = 300
	}
	if cfg.PerPage == 0 {
		cfg.PerPage = 30
	}
	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: 20 * time.Second}
	}
	return &Pexels{cfg: cfg, hc: hc, lim: NewLimiter(cfg.MinIntervalMS, cfg.CooldownSec)}
}

func (p *Pexels) Name() string { return "pexels" }

func (p *Pexels) Configured() bool { return p.cfg.APIKey != "" }

// FetchURLs queries /v1/search and returns the literal `src.original` URL
// for each hit (the uncompressed, unresized original asset).
func (p *Pexels) FetchURLs(ctx context.Context, req Request) ([]string, error) {
	if !p.Configured() {
		return nil, nil
	}
	if err := p.lim.Wait(ctx); err != nil {
		return nil, err
	}

	per := req.Count
	if per <= 0 {
		per = p.cfg.PerPage
	}
	if per < 3 {
		per = 3
	}
	if per > 80 {
		per = 80
	}

	q := url.Values{}
	q.Set("query", strings.TrimSpace(ResolveKeyword(req.Type, req.Keyword)))
	q.Set("per_page", strconv.Itoa(per))
	if req.Page > 0 {
		q.Set("page", strconv.Itoa(req.Page))
	}
	if o := pexelsOrientation(req.Type); o != "" {
		q.Set("orientation", o)
	}
	// We deliberately don't set Pexels' `size` filter (large=24MP, medium=12MP).
	// Pexels' default upload bar is already high — most assets are 4K-8K — and
	// we rely on PEXELS_MIN_WIDTH post-filtering for the precise floor. Keeping
	// the request unfiltered maximizes the result set and lets us page deeper
	// before hitting empty results.

	endpoint := "https://api.pexels.com/v1/search?" + q.Encode()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", p.cfg.APIKey)
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("User-Agent", "imget-go/1.0")

	resp, err := p.hc.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("pexels: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		p.lim.MarkCooldown()
		return nil, errors.New("pexels: 429 rate limited")
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 500))
		return nil, fmt.Errorf("pexels: HTTP %d: %s", resp.StatusCode, string(body))
	}

	var payload struct {
		Photos []struct {
			Width  int            `json:"width"`
			Height int            `json:"height"`
			Src    map[string]any `json:"src"`
		} `json:"photos"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("pexels: decode: %w", err)
	}

	out := make([]string, 0, len(payload.Photos))
	for _, hit := range payload.Photos {
		if p.cfg.MinWidth > 0 && hit.Width < p.cfg.MinWidth {
			continue
		}
		if u := pickPexelsURL(hit.Src); u != "" {
			out = append(out, u)
		}
	}
	return out, nil
}

func pickPexelsURL(src map[string]any) string {
	for _, key := range []string{"original", "large2x", "large"} {
		if v, ok := src[key].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

// pexelsOrientation maps category → Pexels orientation hint where it
// improves relevance. Empty means "any" (Pexels default = mixed).
func pexelsOrientation(typ string) string {
	switch NormalizeType(typ) {
	case "beauty":
		return "portrait"
	case "banner", "landscape", "city", "nature", "travel", "architecture", "space":
		return "landscape"
	}
	return ""
}
