package source

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// BingConfig configures the Bing daily-wallpaper provider. Bing publishes one
// curated high-resolution scenic photo per day and exposes the recent archive
// (no API key). It has no keyword search, so it only contributes to the
// "landscape" bucket — the images are always scenic wallpapers.
type BingConfig struct {
	Market        string // e.g. "en-US", "zh-CN"; empty = en-US
	MinIntervalMS int
	CooldownSec   int
	HTTPClient    *http.Client
}

type Bing struct {
	cfg BingConfig
	hc  *http.Client
	lim *Limiter
}

func NewBing(cfg BingConfig) *Bing {
	if cfg.Market == "" {
		cfg.Market = "en-US"
	}
	if cfg.MinIntervalMS == 0 {
		cfg.MinIntervalMS = 500
	}
	if cfg.CooldownSec == 0 {
		cfg.CooldownSec = 120
	}
	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: 20 * time.Second}
	}
	return &Bing{cfg: cfg, hc: hc, lim: NewLimiter(cfg.MinIntervalMS, cfg.CooldownSec)}
}

func (b *Bing) Name() string { return "bing" }

func (b *Bing) Configured() bool { return true }

// FetchURLs returns the recent Bing daily images (UHD) as full-resolution JPEG
// URLs. Only scenic buckets are served; other categories get nil so the same
// wallpapers don't leak into unrelated pools.
func (b *Bing) FetchURLs(ctx context.Context, req Request) ([]string, error) {
	if NormalizeType(req.Type) != "landscape" {
		return nil, nil
	}
	if err := b.lim.Wait(ctx); err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf(
		"https://www.bing.com/HPImageArchive.aspx?format=js&idx=0&n=8&mkt=%s", b.cfg.Market)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("User-Agent", "imget-go/1.0")

	resp, err := b.hc.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("bing: request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 500))
		return nil, fmt.Errorf("bing: HTTP %d: %s", resp.StatusCode, string(body))
	}

	var payload struct {
		Images []struct {
			URLBase string `json:"urlbase"`
			URL     string `json:"url"`
		} `json:"images"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("bing: decode: %w", err)
	}

	out := make([]string, 0, len(payload.Images))
	for _, im := range payload.Images {
		// Prefer the UHD (4K) rendition built from urlbase; fall back to the
		// default 1920x1080 url. Both are relative to www.bing.com.
		var path string
		if im.URLBase != "" {
			path = im.URLBase + "_UHD.jpg"
		} else if im.URL != "" {
			path = im.URL
		} else {
			continue
		}
		if !strings.HasPrefix(path, "http") {
			path = "https://www.bing.com" + path
		}
		out = append(out, path)
	}
	return out, nil
}
