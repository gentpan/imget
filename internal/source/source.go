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

// Chain queries providers in order and returns the first non-empty result.
// Errors from earlier providers are joined into the final error iff every
// provider failed.
func Chain(ctx context.Context, providers []Provider, req Request) ([]string, error) {
	var firstErrs []error
	for _, p := range providers {
		urls, err := p.FetchURLs(ctx, req)
		if err != nil {
			firstErrs = append(firstErrs, err)
			continue
		}
		if len(urls) > 0 {
			return urls, nil
		}
	}
	if len(firstErrs) > 0 {
		return nil, errors.Join(firstErrs...)
	}
	return nil, nil
}

// FetchAll queries EVERY provider and returns the union of their candidate URLs
// (de-duplicated, order preserved). Unlike Chain — which stops at the first
// non-empty provider and is right for latency-sensitive serve-time prefetch —
// this maximises the fresh-candidate pool and is used by the bulk topup job so
// each provider contributes instead of being shadowed by an earlier one. An
// error is returned only when every provider failed AND none produced URLs.
func FetchAll(ctx context.Context, providers []Provider, req Request) ([]string, error) {
	seen := make(map[string]struct{})
	out := make([]string, 0, len(providers)*req.Count)
	var errs []error
	for _, p := range providers {
		urls, err := p.FetchURLs(ctx, req)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		for _, u := range urls {
			if u == "" {
				continue
			}
			if _, dup := seen[u]; dup {
				continue
			}
			seen[u] = struct{}{}
			out = append(out, u)
		}
	}
	if len(out) == 0 && len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return out, nil
}
