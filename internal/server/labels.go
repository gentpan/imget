package server

import (
	"os"
	"path/filepath"
	"strings"

	"imget/internal/encoder"
)

// typeChineseLabel mirrors imageRouter.php :: getTypeChineseLabel().
var typeChineseLabel = map[string]string{
	"banner":       "通用横幅",
	"landscape":    "风景山水",
	"beauty":       "人物美图",
	"anime":        "动漫插画",
	"city":         "城市建筑",
	"nature":       "自然风光",
	"car":          "汽车",
	"game":         "游戏电竞",
	"food":         "美食",
	"animal":       "动物",
	"travel":       "旅行",
	"space":        "星空宇宙",
	"tech":         "科技",
	"business":     "商务",
	"sports":       "运动",
	"architecture": "建筑设计",
}

func TypeChineseLabel(typ string) string {
	if v, ok := typeChineseLabel[strings.ToLower(typ)]; ok {
		return v
	}
	return typ
}

// FormatImageFormatLabel mirrors PHP's formatImageFormatLabel().
//
//	webp -> WebP, avif -> AVIF, jpg/jpeg -> JPG, png -> PNG, gif -> GIF.
func FormatImageFormatLabel(formatOrExt string) string {
	v := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(formatOrExt)), ".")
	switch v {
	case "webp":
		return "WebP"
	case "avif":
		return "AVIF"
	case "jpg", "jpeg":
		return "JPG"
	case "png":
		return "PNG"
	case "gif":
		return "GIF"
	}
	return strings.ToUpper(v)
}

// FileMeta is the structural equivalent of PHP's getFileMeta() return array.
type FileMeta struct {
	Width     int
	Height    int
	Extension string
	SizeHuman string
	SizeBytes int64
}

// GetFileMeta inspects the file at absPath and returns dimensions + size.
// Uses libvips through the encoder package — handles every format we encode
// (WebP, AVIF, JPG, PNG, GIF) including AVIF, which Go's std lib can't decode.
func GetFileMeta(absPath string) FileMeta {
	m := FileMeta{
		Extension: strings.TrimPrefix(strings.ToLower(filepath.Ext(absPath)), "."),
	}
	if fi, err := os.Stat(absPath); err == nil {
		m.SizeBytes = fi.Size()
		m.SizeHuman = humanBytes(fi.Size())
	}
	if encoder.SupportedExt(m.Extension) {
		if w, h, err := encoder.Probe(absPath); err == nil {
			m.Width = w
			m.Height = h
		}
	}
	return m
}
