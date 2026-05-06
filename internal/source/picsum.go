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
	count := req.Count
	if count <= 0 {
		count = 1
	}
	if count > 200 {
		count = 200
	}

	w := req.Width
	if w <= 0 {
		w = 1920
	}
	h := req.Height
	if h <= 0 {
		h = 1080
	}

	out := make([]string, 0, count)
	for i := 0; i < count; i++ {
		seed := rand.Int63()
		out = append(out, fmt.Sprintf("https://picsum.photos/seed/%d/%d/%d", seed, w, h))
	}
	return out, nil
}
