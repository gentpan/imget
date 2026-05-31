package imgpipe

import (
	"context"
	"crypto/sha1"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"time"

	"imget/internal/encoder"
	"imget/internal/metrics"
)

// renderToFile decodes srcPath, center-crops to (w,h), and encodes into dstPath.
// All steps go through libvips via the encoder package.
func (p *Pipeline) renderToFile(ctx context.Context, srcPath, dstPath string, w, h int, format string) error {
	// Bound concurrent encodes to CPU count (see renderSem in New).
	select {
	case p.renderSem <- struct{}{}:
		defer func() { <-p.renderSem }()
	case <-ctx.Done():
		return ctx.Err()
	}

	// AVIF encoding is CPU-heavy and libvips itself is not context-aware, so a
	// pathological input could pin a worker indefinitely. Enforce AVIF_TIMEOUT_SEC
	// (previously read but never applied) by abandoning the wait on timeout.
	if format == "avif" && p.cfg.AVIFTimeoutSec > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(p.cfg.AVIFTimeoutSec)*time.Second)
		defer cancel()
	}

	start := time.Now()
	err := p.encodeWithDeadline(ctx, srcPath, dstPath, w, h, encoder.Format(format))
	metrics.ObserveRender(format, time.Since(start), err)
	return err
}

// encodeWithDeadline runs the blocking, non-cancellable libvips render on a side
// goroutine so a timed-out AVIF encode returns control to the request instead of
// pinning it. The goroutine runs to completion in the background — the encoder's
// .part temp file is removed on its own deferred cleanup — and a later request
// for the same variant simply finds it cached if it eventually lands.
func (p *Pipeline) encodeWithDeadline(ctx context.Context, srcPath, dstPath string, w, h int, format encoder.Format) error {
	done := make(chan error, 1)
	go func() {
		done <- p.enc.RenderToFile(ctx, srcPath, dstPath, w, h, format)
	}()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// makeFallbackOriginal writes a procedural gradient PNG into images/_fallback/
// and returns its relative path. Used when no real image source is available.
//
// It lives OUTSIDE original/{type}/ on purpose: keeping it in the real source
// pool would let pickLocalOriginal serve the gradient even after real images
// arrive, inflate CountOriginalsForType, and make HasAnyOriginal report a
// category as "ready" when it only has a placeholder.
//
// We keep this on the std lib (image/png) because the gradient is built
// pixel-by-pixel — there's no benefit from libvips here, and avoiding vips
// imports keeps the fallback path independent of the encoder.
func (p *Pipeline) makeFallbackOriginal(typ string) (string, error) {
	dir := filepath.Join(p.cfg.AbsImagesDir(), "_fallback")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	name := typ + ".png"
	abs := filepath.Join(dir, name)
	rel := filepath.ToSlash(filepath.Join("_fallback", name))
	if fi, err := os.Stat(abs); err == nil && fi.Size() > 0 {
		return rel, nil
	}

	const W, H = 1920, 1080
	img := image.NewRGBA(image.Rect(0, 0, W, H))
	c1, c2 := gradientColorsFor(typ)
	for y := 0; y < H; y++ {
		t := float64(y) / float64(H-1)
		r := uint8(lerp(float64(c1.R), float64(c2.R), t))
		g := uint8(lerp(float64(c1.G), float64(c2.G), t))
		b := uint8(lerp(float64(c1.B), float64(c2.B), t))
		row := image.NewUniform(color.RGBA{R: r, G: g, B: b, A: 255})
		for x := 0; x < W; x++ {
			img.Set(x, y, row.C)
		}
	}

	tmp := abs + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return "", err
	}
	if err := (&png.Encoder{CompressionLevel: png.NoCompression}).Encode(f, img); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return "", err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	if err := os.Rename(tmp, abs); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	return rel, nil
}

func gradientColorsFor(typ string) (a, b color.RGBA) {
	h := sha1.Sum([]byte(typ))
	a = color.RGBA{R: h[0], G: h[1], B: h[2], A: 255}
	b = color.RGBA{R: h[3], G: h[4], B: h[5], A: 255}
	if dist(a, b) < 60 {
		b.R = 255 - b.R
		b.G = 255 - b.G
		b.B = 255 - b.B
	}
	return
}

func dist(a, b color.RGBA) float64 {
	dr := float64(a.R) - float64(b.R)
	dg := float64(a.G) - float64(b.G)
	db := float64(a.B) - float64(b.B)
	return math.Sqrt(dr*dr + dg*dg + db*db)
}

func lerp(a, b, t float64) float64 { return a + (b-a)*t }
