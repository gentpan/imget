package imgpipe

import (
	"context"
	"crypto/sha1"
	"fmt"
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
	start := time.Now()
	err := p.enc.RenderToFile(ctx, srcPath, dstPath, w, h, encoder.Format(format))
	metrics.ObserveRender(format, time.Since(start), err)
	return err
}

// makeFallbackOriginal writes a procedural gradient PNG into images/original/{type}/
// and returns its relative path. Used when no real image source is available.
//
// We keep this on the std lib (image/png) because the gradient is built
// pixel-by-pixel — there's no benefit from libvips here, and avoiding vips
// imports keeps the fallback path independent of the encoder.
func (p *Pipeline) makeFallbackOriginal(typ string) (string, error) {
	dir := filepath.Join(p.cfg.AbsImagesDir(), "original", typ)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	name := "fallback-" + typ + ".png"
	abs := filepath.Join(dir, name)
	if fi, err := os.Stat(abs); err == nil && fi.Size() > 0 {
		return filepath.ToSlash(filepath.Join("original", typ, name)), nil
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
	return filepath.ToSlash(filepath.Join("original", typ, name)), nil
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

var _ = fmt.Sprintf // keep fmt import even if unused after future edits
