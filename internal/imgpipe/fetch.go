package imgpipe

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"imget/internal/mediatype"
	"imget/internal/source"
)

// FetchRequest captures parameters for a one-shot fetch (used by handlers and CLI).
type FetchRequest struct {
	Type    string
	Keyword string
	Width   int
	Height  int
	Count   int    // how many fresh originals to download
	Page    int    // upstream pagination (0 = let provider default)
	Order   string // upstream order hint (e.g. "latest"); empty = provider default
}

// FetchToLocal downloads up to req.Count new images into images/original/{type}/,
// dedupes by SHA1, and returns the list of saved relative paths.
func (p *Pipeline) FetchToLocal(ctx context.Context, req FetchRequest) ([]string, error) {
	typ := source.NormalizeType(req.Type)
	count := req.Count
	if count <= 0 {
		count = 5
	}
	if count > 1000 {
		count = 1000
	}

	srcReq := source.Request{
		Type:    typ,
		Keyword: req.Keyword,
		Width:   req.Width,
		Height:  req.Height,
		Count:   count + 5, // ask for a few extras to absorb dedupe drops
		Page:    req.Page,
		Order:   req.Order,
	}

	urls, err := source.Chain(ctx, p.sources, srcReq)
	if err != nil {
		return nil, err
	}
	if len(urls) == 0 {
		return nil, errors.New("no source URLs returned")
	}

	dstDir := filepath.Join(p.cfg.AbsImagesDir(), "original", typ)
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return nil, err
	}

	saved := make([]string, 0, count)
	seen := map[string]struct{}{}

	for _, u := range urls {
		if len(saved) >= count {
			break
		}
		rel, err := p.downloadOne(ctx, u, dstDir, typ, seen)
		if err != nil {
			p.log.Debug("download skipped", "url", u, "err", err)
			continue
		}
		if rel == "" {
			continue
		}
		saved = append(saved, rel)
		// Queue upload of the freshly saved original.
		p.QueueUpload(rel)
	}

	if len(saved) == 0 {
		return nil, errors.New("all candidate downloads failed or duplicated")
	}
	return saved, nil
}

func (p *Pipeline) downloadOne(ctx context.Context, u, dstDir, typ string, seen map[string]struct{}) (string, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(reqCtx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("User-Agent", "imget-go/1.0")

	resp, err := p.hc.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	tmp, err := os.CreateTemp(dstDir, ".dl-*")
	if err != nil {
		return "", err
	}
	tmpName := tmp.Name()
	hasher := sha1.New()

	w := io.MultiWriter(tmp, hasher)
	limited := io.LimitReader(resp.Body, 50<<20) // 50MB cap
	n, copyErr := io.Copy(w, limited)
	closeErr := tmp.Close()

	if copyErr != nil || closeErr != nil {
		_ = os.Remove(tmpName)
		if copyErr != nil {
			return "", copyErr
		}
		return "", closeErr
	}
	if n < 1024 {
		_ = os.Remove(tmpName)
		return "", fmt.Errorf("file too small (%d bytes)", n)
	}

	digest := hex.EncodeToString(hasher.Sum(nil))
	if _, dup := seen[digest]; dup {
		_ = os.Remove(tmpName)
		return "", errors.New("duplicate within batch")
	}
	seen[digest] = struct{}{}

	ext := guessExtension(resp.Header.Get("Content-Type"), u)
	finalName := digest + ext
	finalPath := filepath.Join(dstDir, finalName)
	if _, err := os.Stat(finalPath); err == nil {
		_ = os.Remove(tmpName)
		return "", errors.New("already exists")
	}
	if err := os.Rename(tmpName, finalPath); err != nil {
		_ = os.Remove(tmpName)
		return "", err
	}

	return filepath.ToSlash(filepath.Join("original", typ, finalName)), nil
}

func guessExtension(contentType, urlStr string) string {
	if ext := mediatype.ExtForContentType(contentType); ext != "" {
		return ext
	}
	// Fall back to URL suffix.
	if i := strings.LastIndex(urlStr, "."); i > 0 {
		ext := strings.ToLower(urlStr[i:])
		if len(ext) <= 5 {
			// Strip query string if present.
			if q := strings.IndexAny(ext, "?#"); q > 0 {
				ext = ext[:q]
			}
			switch ext {
			case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".avif":
				if ext == ".jpeg" {
					return ".jpg"
				}
				return ext
			}
		}
	}
	return ".jpg"
}
