package source

import (
	"context"
	"fmt"
	"math/rand"
)

// Picsum is a fallback provider that yields procedural picsum.photos URLs.
// It never errors and always returns Count URLs.
type Picsum struct{}

func NewPicsum() *Picsum { return &Picsum{} }

func (p *Picsum) Name() string { return "picsum" }

func (p *Picsum) FetchURLs(ctx context.Context, req Request) ([]string, error) {
	// Picsum is a procedural placeholder source (Lorem Picsum) — it has no
	// concept of category, so it must only fire when the caller specifies a
	// concrete target size (i.e. serve-time render request with explicit
	// W/H). For bulk-ingest paths like topup-types where W/H are 0, returning
	// nil lets source.Chain skip Picsum so we don't pollute category pools
	// with off-topic placeholder images.
	if req.Width <= 0 || req.Height <= 0 {
		return nil, nil
	}

	count := req.Count
	if count <= 0 {
		count = 1
	}
	if count > 200 {
		count = 200
	}

	out := make([]string, 0, count)
	for i := 0; i < count; i++ {
		seed := rand.Int63()
		out = append(out, fmt.Sprintf("https://picsum.photos/seed/%d/%d/%d", seed, req.Width, req.Height))
	}
	return out, nil
}
