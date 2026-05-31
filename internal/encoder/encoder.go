// Package encoder wraps libvips (via govips) for the entire image-rendering
// pipeline: decode, resize-with-crop, and encode to WebP / AVIF.
//
// libvips is faster and more memory-efficient than shelling out to cwebp +
// avifenc, supports decoding AVIF (which Go's std lib does not), and centralizes
// all image format handling in a single dependency.
//
// Initialization: vips.Startup must be called exactly once at process startup
// (see Init() below) and vips.Shutdown at exit.
package encoder

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/davidbyttow/govips/v2/vips"
)

type Format string

const (
	FormatWebP Format = "webp"
	FormatAVIF Format = "avif"
)

// Options controls encoder quality.
type Options struct {
	WebPQuality int // 1..100
	AVIFQuality int // 1..100 (lower = smaller)
	AVIFSpeed   int // 0..9, higher = faster encode but larger output
}

type Encoder struct {
	opts Options
}

var initOnce sync.Once

// Init must be called once before any encoder method. Safe to call multiple
// times; subsequent calls are no-ops.
//
// Performs a tiny no-op decode to "warm up" libvips (force its internal
// thread pool + cache structures to initialize) so the first real request
// doesn't pay a 30-50ms cold-start penalty.
func Init() {
	initOnce.Do(func() {
		vips.LoggingSettings(nil, vips.LogLevelError) // suppress info/debug spam
		vips.Startup(nil)                             // default config

		// Warmup: decode an embedded 1x1 PNG. We don't care about the
		// result — only side effects (thread pool init, JIT loading).
		if img, err := vips.NewImageFromBuffer(warmupPNG); err == nil {
			img.Close()
		}
	})
}

// warmupPNG is a 1x1 transparent PNG (67 bytes). Used by Init() to force
// libvips to load its decoder modules at startup instead of on first request.
var warmupPNG = []byte{
	0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
	0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4,
	0x89, 0x00, 0x00, 0x00, 0x0D, 0x49, 0x44, 0x41,
	0x54, 0x78, 0x9C, 0x63, 0x00, 0x01, 0x00, 0x00,
	0x05, 0x00, 0x01, 0x0D, 0x0A, 0x2D, 0xB4, 0x00,
	0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44, 0xAE,
	0x42, 0x60, 0x82,
}

// Shutdown releases libvips resources. Call at process exit.
func Shutdown() { vips.Shutdown() }

func New(opts Options) (*Encoder, error) {
	Init()
	if opts.WebPQuality == 0 {
		opts.WebPQuality = 85
	}
	if opts.AVIFQuality == 0 {
		opts.AVIFQuality = 60
	}
	if opts.AVIFSpeed == 0 {
		opts.AVIFSpeed = 6
	}
	return &Encoder{opts: opts}, nil
}

// Available reports whether the requested format can be produced. With libvips
// linked against libwebp + libheif/libaom, both are always available.
func (e *Encoder) Available(f Format) bool {
	switch f {
	case FormatWebP, FormatAVIF:
		return true
	}
	return false
}

// RenderToFile decodes srcPath, center-crops/resizes to (width, height) and
// encodes the result into dstPath in the chosen format. This is the single
// hot-path call site for the image pipeline.
func (e *Encoder) RenderToFile(ctx context.Context, srcPath, dstPath string, width, height int, format Format) error {
	if width <= 0 || height <= 0 {
		return fmt.Errorf("invalid dims %dx%d", width, height)
	}
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return err
	}

	img, err := vips.NewImageFromFile(srcPath)
	if err != nil {
		return fmt.Errorf("decode %s: %w", srcPath, err)
	}
	defer img.Close()

	// Thumbnail does load + scale + crop in one operation; vips uses Lanczos
	// by default. InterestingCentre = center crop (matches the PHP behaviour).
	if err := img.ThumbnailWithSize(width, height, vips.InterestingCentre, vips.SizeBoth); err != nil {
		return fmt.Errorf("thumbnail: %w", err)
	}

	tmp := dstPath + ".part"
	defer os.Remove(tmp)

	if err := e.exportTo(img, format, tmp); err != nil {
		return fmt.Errorf("encode %s: %w", format, err)
	}
	if err := os.Rename(tmp, dstPath); err != nil {
		return err
	}
	_ = ctx // ctx kept for API parity; libvips itself isn't context-aware
	return nil
}

// Probe returns (width, height) for the given file by loading just its header.
// Works for every format libvips understands, including AVIF.
func Probe(srcPath string) (int, int, error) {
	img, err := vips.NewImageFromFile(srcPath)
	if err != nil {
		return 0, 0, err
	}
	defer img.Close()
	return img.Width(), img.Height(), nil
}

func (e *Encoder) exportTo(img *vips.ImageRef, format Format, dstPath string) error {
	switch format {
	case FormatWebP:
		buf, _, err := img.ExportWebp(&vips.WebpExportParams{
			Quality:         e.opts.WebPQuality,
			ReductionEffort: 4, // 0..6, 4 is a good speed/size tradeoff
			Lossless:        false,
		})
		if err != nil {
			return err
		}
		return os.WriteFile(dstPath, buf, 0o644)
	case FormatAVIF:
		buf, _, err := img.ExportAvif(&vips.AvifExportParams{
			Quality:  e.opts.AVIFQuality,
			Speed:    e.opts.AVIFSpeed,
			Lossless: false,
		})
		if err != nil {
			return err
		}
		return os.WriteFile(dstPath, buf, 0o644)
	}
	return errors.New("encoder: unsupported format")
}

// SupportedExt indicates whether libvips can decode the given file extension.
// (Used by GetFileMeta as a quick gate before invoking Probe.)
func SupportedExt(ext string) bool {
	ext = strings.ToLower(strings.TrimPrefix(ext, "."))
	switch ext {
	case "jpg", "jpeg", "png", "gif", "webp", "avif", "tiff", "tif", "bmp", "heic", "heif":
		return true
	}
	return false
}
