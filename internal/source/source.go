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
	Count   int // hint: max URLs we'd like back
	Page    int // 1-based pagination for upstream APIs that support it
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
