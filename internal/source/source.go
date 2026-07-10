package source

import (
	"context"
	"errors"
)

// Request describes what kind of images the caller wants.
type Request struct {
	Type    string // canonical type slug (e.g. "banner")
	Keyword string // explicit search query, possibly empty
	Width   int
	Height  int
	Count   int    // hint: max URLs we'd like back
	Page    int    // 1-based pagination for upstream APIs that support it
	Order   string // upstream order hint, e.g. Pixabay "latest" or "popular" (default empty = provider default)
}

// Provider is the unified interface for an image-URL source.
type Provider interface {
	Name() string

	// FetchURLs returns up to req.Count candidate source URLs.
	// Returning fewer than req.Count is fine (or even zero, with nil err
	// meaning the source had nothing to offer).
	FetchURLs(ctx context.Context, req Request) ([]string, error)
}

// Candidate is a source URL tagged with the provider it came from, so callers
// can record provenance for each downloaded image.
type Candidate struct {
	URL      string
	Provider string
}

// Chain queries providers in order and returns the first non-empty result,
// tagged with that provider's name. Errors from earlier providers are joined
// into the final error iff every provider failed.
func Chain(ctx context.Context, providers []Provider, req Request) ([]Candidate, error) {
	var firstErrs []error
	for _, p := range providers {
		urls, err := p.FetchURLs(ctx, req)
		if err != nil {
			firstErrs = append(firstErrs, err)
			continue
		}
		if len(urls) > 0 {
			return tag(urls, p.Name()), nil
		}
	}
	if len(firstErrs) > 0 {
		return nil, errors.Join(firstErrs...)
	}
	return nil, nil
}

func tag(urls []string, provider string) []Candidate {
	out := make([]Candidate, 0, len(urls))
	for _, u := range urls {
		out = append(out, Candidate{URL: u, Provider: provider})
	}
	return out
}

// FetchAll queries EVERY provider and returns the union of their candidate URLs
// (de-duplicated, order preserved). Unlike Chain — which stops at the first
// non-empty provider and is right for latency-sensitive serve-time prefetch —
// this maximises the fresh-candidate pool and is used by the bulk topup job so
// each provider contributes instead of being shadowed by an earlier one. An
// error is returned only when every provider failed AND none produced URLs.
func FetchAll(ctx context.Context, providers []Provider, req Request) ([]Candidate, error) {
	// Gather each provider's results separately, then round-robin interleave
	// them. Concatenating instead would let the first provider (Pexels) fill the
	// caller's whole quota before any other source is reached — so Wikimedia,
	// Openverse, Bing, etc. would never actually contribute. Interleaving gives
	// every source a fair share of the download budget.
	lists := make([][]Candidate, 0, len(providers))
	var errs []error
	maxLen := 0
	for _, p := range providers {
		urls, err := p.FetchURLs(ctx, req)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if len(urls) == 0 {
			continue
		}
		name := p.Name()
		cs := make([]Candidate, 0, len(urls))
		for _, u := range urls {
			if u != "" {
				cs = append(cs, Candidate{URL: u, Provider: name})
			}
		}
		if len(cs) > 0 {
			lists = append(lists, cs)
			if len(cs) > maxLen {
				maxLen = len(cs)
			}
		}
	}

	seen := make(map[string]struct{})
	out := make([]Candidate, 0, len(providers)*maxLen)
	for i := 0; i < maxLen; i++ {
		for _, list := range lists {
			if i >= len(list) {
				continue
			}
			c := list[i]
			if _, dup := seen[c.URL]; dup {
				continue
			}
			seen[c.URL] = struct{}{}
			out = append(out, c)
		}
	}
	if len(out) == 0 && len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return out, nil
}
