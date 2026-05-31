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
	"sync"
	"time"
)

// PixabayConfig configures the Pixabay provider.
type PixabayConfig struct {
	APIKey        string
	BackupAPIKey  string
	MinIntervalMS int
	CooldownSec   int
	PerPage       int
	HTTPClient    *http.Client
}

type Pixabay struct {
	cfg PixabayConfig
	hc  *http.Client
	lim *Limiter

	bkMu         sync.Mutex // guards useBackupKey only
	useBackupKey bool
}

func NewPixabay(cfg PixabayConfig) *Pixabay {
	if cfg.MinIntervalMS == 0 {
		cfg.MinIntervalMS = 900
	}
	if cfg.CooldownSec == 0 {
		cfg.CooldownSec = 300
	}
	if cfg.PerPage == 0 {
		cfg.PerPage = 50
	}
	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: 15 * time.Second}
	}
	return &Pixabay{cfg: cfg, hc: hc, lim: NewLimiter(cfg.MinIntervalMS, cfg.CooldownSec)}
}

func (p *Pixabay) Name() string { return "pixabay" }

func (p *Pixabay) Configured() bool { return p.cfg.APIKey != "" || p.cfg.BackupAPIKey != "" }

// FetchURLs queries Pixabay /api/ for candidate URLs.
func (p *Pixabay) FetchURLs(ctx context.Context, req Request) ([]string, error) {
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
	if per > 200 {
		per = 200
	}

	q := url.Values{}
	q.Set("q", strings.TrimSpace(ResolveKeyword(req.Type, req.Keyword)))
	q.Set("image_type", PixabayImageType(req.Type))
	q.Set("per_page", strconv.Itoa(per))
	q.Set("safesearch", "true")
	if cat := PixabayCategory(req.Type); cat != "" {
		q.Set("category", cat)
	}
	if req.Page > 0 {
		q.Set("page", strconv.Itoa(req.Page))
	}
	if o := strings.ToLower(strings.TrimSpace(req.Order)); o == "latest" || o == "popular" {
		q.Set("order", o)
	}
	if req.Width > 0 {
		q.Set("min_width", strconv.Itoa(req.Width))
	}
	if req.Height > 0 {
		q.Set("min_height", strconv.Itoa(req.Height))
	}

	endpoint := "https://pixabay.com/api/?" + q.Encode()
	apiKey := p.activeKey()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"&key="+apiKey, nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("User-Agent", "imget-go/1.0")

	resp, err := p.hc.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("pixabay: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		p.lim.MarkCooldown()
		// Try backup key once if available.
		if p.cfg.BackupAPIKey != "" {
			p.bkMu.Lock()
			p.useBackupKey = true
			p.bkMu.Unlock()
		}
		return nil, errors.New("pixabay: 429 rate limited")
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 500))
		return nil, fmt.Errorf("pixabay: HTTP %d: %s", resp.StatusCode, string(body))
	}

	var payload struct {
		Hits []map[string]any `json:"hits"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("pixabay: decode: %w", err)
	}

	out := make([]string, 0, len(payload.Hits))
	for _, hit := range payload.Hits {
		if u := pickPixabayURL(hit); u != "" {
			out = append(out, u)
		}
	}
	return out, nil
}

func pickPixabayURL(hit map[string]any) string {
	for _, key := range []string{"imageURL", "fullHDURL", "largeImageURL", "webformatURL", "previewURL"} {
		if v, ok := hit[key].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

func (p *Pixabay) activeKey() string {
	p.bkMu.Lock()
	useBackup := p.useBackupKey
	p.bkMu.Unlock()
	if useBackup && p.cfg.BackupAPIKey != "" {
		return p.cfg.BackupAPIKey
	}
	if p.cfg.APIKey != "" {
		return p.cfg.APIKey
	}
	return p.cfg.BackupAPIKey
}
