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
	"sync"
	"time"
)

// OpenverseConfig configures the Openverse provider. Openverse indexes 600M+
// openly-licensed images and works anonymously, but the anonymous throttle is
// low; supplying ClientID/ClientSecret (a free registered application) raises
// the daily quota by ~100x. When creds are set the provider fetches and caches
// an OAuth2 bearer token automatically.
type OpenverseConfig struct {
	ClientID      string
	ClientSecret  string
	MinIntervalMS int
	CooldownSec   int
	PerPage       int
	HTTPClient    *http.Client
}

type Openverse struct {
	cfg OpenverseConfig
	hc  *http.Client
	lim *Limiter

	tokMu      sync.Mutex
	token      string
	tokenUntil time.Time
}

func NewOpenverse(cfg OpenverseConfig) *Openverse {
	if cfg.MinIntervalMS == 0 {
		cfg.MinIntervalMS = 1200
	}
	if cfg.CooldownSec == 0 {
		cfg.CooldownSec = 300
	}
	if cfg.PerPage == 0 {
		cfg.PerPage = 20
	}
	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: 20 * time.Second}
	}
	return &Openverse{cfg: cfg, hc: hc, lim: NewLimiter(cfg.MinIntervalMS, cfg.CooldownSec)}
}

func (o *Openverse) Name() string { return "openverse" }

// Configured is always true — Openverse serves anonymous requests. Credentials
// only raise the rate limit.
func (o *Openverse) Configured() bool { return true }

// FetchURLs queries /v1/images and returns each result's direct `url`.
func (o *Openverse) FetchURLs(ctx context.Context, req Request) ([]string, error) {
	if err := o.lim.Wait(ctx); err != nil {
		return nil, err
	}

	per := req.Count
	if per <= 0 {
		per = o.cfg.PerPage
	}
	if per < 5 {
		per = 5
	}
	if per > 40 {
		per = 40
	}

	q := strings.TrimSpace(ResolveKeyword(req.Type, req.Keyword))
	if q == "" {
		q = NormalizeType(req.Type)
	}

	v := url.Values{}
	v.Set("q", q)
	v.Set("page_size", strconv.Itoa(per))
	if req.Page > 0 {
		v.Set("page", strconv.Itoa(req.Page))
	}
	v.Set("mature", "false")

	endpoint := "https://api.openverse.org/v1/images/?" + v.Encode()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("User-Agent", "imget-go/1.0")
	if tok := o.bearer(ctx); tok != "" {
		httpReq.Header.Set("Authorization", "Bearer "+tok)
	}

	resp, err := o.hc.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openverse: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		o.lim.MarkCooldown()
		return nil, fmt.Errorf("openverse: 429 rate limited")
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 500))
		return nil, fmt.Errorf("openverse: HTTP %d: %s", resp.StatusCode, string(body))
	}

	var payload struct {
		Results []struct {
			URL      string `json:"url"`
			Width    int    `json:"width"`
			Height   int    `json:"height"`
			FileType string `json:"filetype"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("openverse: decode: %w", err)
	}

	out := make([]string, 0, len(payload.Results))
	for _, r := range payload.Results {
		if r.URL == "" {
			continue
		}
		if !openverseWantType(r.FileType) {
			continue
		}
		if req.Width > 0 && r.Width > 0 && r.Width < req.Width {
			continue
		}
		out = append(out, r.URL)
	}
	return out, nil
}

func openverseWantType(ft string) bool {
	switch strings.ToLower(strings.TrimSpace(ft)) {
	case "", "jpg", "jpeg", "png", "webp":
		// Empty filetype is common and usually a JPEG; keep it — downloadOne
		// still validates the actual bytes and content-type on save.
		return true
	}
	return false
}

// bearer returns a cached OAuth2 token, fetching a fresh one when credentials
// are set and the cache is empty/expired. Returns "" when no creds are set
// (anonymous access) or the token exchange fails (falls back to anonymous).
func (o *Openverse) bearer(ctx context.Context) string {
	if o.cfg.ClientID == "" || o.cfg.ClientSecret == "" {
		return ""
	}
	o.tokMu.Lock()
	defer o.tokMu.Unlock()
	if o.token != "" && time.Now().Before(o.tokenUntil) {
		return o.token
	}

	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", o.cfg.ClientID)
	form.Set("client_secret", o.cfg.ClientSecret)

	tokReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.openverse.org/v1/auth_tokens/token/", strings.NewReader(form.Encode()))
	if err != nil {
		return ""
	}
	tokReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	tokReq.Header.Set("Accept", "application/json")

	resp, err := o.hc.Do(tokReq)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	var tok struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil || tok.AccessToken == "" {
		return ""
	}
	o.token = tok.AccessToken
	ttl := tok.ExpiresIn
	if ttl <= 0 {
		ttl = 3600
	}
	// Refresh a minute early to avoid using a token mid-expiry.
	o.tokenUntil = time.Now().Add(time.Duration(ttl-60) * time.Second)
	return o.token
}
