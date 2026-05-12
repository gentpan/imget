package server

import (
	"os"
	"path/filepath"
	"strings"

	"imget/internal/encoder"
)

// typeChineseLabel maps each type slug to its short Chinese display name.
// Kept in sync with source.AllowedTypes (currently 20 entries).
var typeChineseLabel = map[string]string{
	"banner":       "通用横幅",
	"landscape":    "风景山水",
	"beauty":       "人物美图",
	"anime":        "动漫插画",
	"city":         "城市街景",
	"nature":       "自然风光",
	"car":          "汽车",
	"game":         "游戏电竞",
	"food":         "美食",
	"animal":       "动物宠物",
	"travel":       "旅行",
	"space":        "星空宇宙",
	"tech":         "科技",
	"business":     "商务",
	"sports":       "运动健身",
	"architecture": "建筑设计",
	"wedding":      "婚礼",
	"kids":         "儿童家庭",
	"abstract":     "抽象艺术",
	"concert":      "演出舞台",
}

// typeChineseDescription is the longer copy that appears under each card on
// the home page. Each merged sub-category is intentionally mentioned in the
// parent's description so users still see "猫狗" appears under animal etc.
var typeChineseDescription = map[string]string{
	"banner":       "通用横幅、默认头图",
	"landscape":    "山水、海洋、日落、森林、沙漠",
	"beauty":       "人像、肖像、时尚",
	"anime":        "动漫、插画、二次元",
	"city":         "城市、街景、夜生活",
	"nature":       "森林、海洋、天空、花卉植物",
	"car":          "汽车、机车、赛道",
	"game":         "游戏、电竞、虚拟场景",
	"food":         "美食、咖啡、甜点、饮品、水果",
	"animal":       "宠物、狗狗、猫咪、野生动物",
	"travel":       "旅行、目的地、度假氛围",
	"space":        "星空、宇宙、科幻主题",
	"tech":         "科技、数码、未来感",
	"business":     "商务、办公、团队场景",
	"sports":       "运动、健身、瑜伽、赛事",
	"architecture": "建筑、室内、空间设计",
	"wedding":      "婚礼、新人、浪漫仪式",
	"kids":         "儿童、婴儿、家庭欢乐",
	"abstract":     "抽象、纹理、图案、壁纸、复古",
	"concert":      "演出、舞蹈、现场表演",
}

// typeIconClass returns the FontAwesome class string applied to the icon on
// the home page card.
var typeIconClass = map[string]string{
	"banner":       "fa-light fa-panorama",
	"landscape":    "fa-light fa-mountains",
	"beauty":       "fa-light fa-sparkles",
	"anime":        "fa-light fa-stars",
	"city":         "fa-light fa-city",
	"nature":       "fa-light fa-leaf-maple",
	"car":          "fa-light fa-car-side",
	"game":         "fa-light fa-gamepad-modern",
	"food":         "fa-light fa-burger-soda",
	"animal":       "fa-light fa-paw-simple",
	"travel":       "fa-light fa-plane-departure",
	"space":        "fa-light fa-galaxy",
	"tech":         "fa-light fa-microchip-ai",
	"business":     "fa-light fa-briefcase",
	"sports":       "fa-light fa-volleyball",
	"architecture": "fa-light fa-building",
	"wedding":      "fa-light fa-rings-wedding",
	"kids":         "fa-light fa-child",
	"abstract":     "fa-light fa-shapes",
	"concert":      "fa-light fa-microphone-stand",
}

func TypeChineseLabel(typ string) string {
	if v, ok := typeChineseLabel[strings.ToLower(typ)]; ok {
		return v
	}
	return typ
}

// TypeChineseDescription returns the longer card-body copy, or falls back
// to the short label if no description is registered.
func TypeChineseDescription(typ string) string {
	if v, ok := typeChineseDescription[strings.ToLower(typ)]; ok {
		return v
	}
	return TypeChineseLabel(typ)
}

// TypeIconClass returns the FontAwesome class for the type, falling back to
// a neutral image icon when nothing is registered.
func TypeIconClass(typ string) string {
	if v, ok := typeIconClass[strings.ToLower(typ)]; ok {
		return v
	}
	return "fa-light fa-image"
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
