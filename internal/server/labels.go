package server

import (
	"os"
	"path/filepath"
	"strings"

	"imget/internal/encoder"
)

// typeChineseLabel maps each type slug to its short Chinese display name.
var typeChineseLabel = map[string]string{
	// Original 16
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

	// Natural & outdoors
	"ocean":    "海洋",
	"mountain": "山脉",
	"forest":   "森林",
	"desert":   "沙漠",
	"sunset":   "日落",
	"sky":      "天空",
	"flower":   "花卉",

	// People & lifestyle
	"portrait": "人像",
	"fashion":  "时尚",
	"wedding":  "婚礼",
	"yoga":     "瑜伽冥想",
	"fitness":  "健身",
	"baby":     "婴儿",
	"kids":     "儿童",

	// Pets & wildlife
	"pet":      "宠物",
	"dog":      "狗狗",
	"cat":      "猫咪",
	"wildlife": "野生动物",

	// Food & drinks
	"coffee":  "咖啡",
	"dessert": "甜品",
	"drink":   "饮品",
	"fruit":   "水果",

	// Visual & artistic
	"abstract":   "抽象艺术",
	"minimal":    "极简风格",
	"pattern":    "图案纹样",
	"texture":    "材质纹理",
	"wallpaper":  "壁纸",
	"vintage":    "复古",
	"background": "纯背景",

	// Places & culture
	"interior":  "室内空间",
	"street":    "街拍",
	"nightlife": "夜生活",
	"concert":   "演出",
	"dance":     "舞蹈",
}

// typeChineseDescription is the longer copy that appears under each card on
// the home page. Keep it short — 4-12 Chinese characters works best for the grid.
var typeChineseDescription = map[string]string{
	// Original 16
	"banner":       "通用横幅、默认头图",
	"landscape":    "风景、山水、自然场景",
	"beauty":       "人物、人像、美图",
	"anime":        "动漫、插画、二次元",
	"city":         "城市、建筑、街景",
	"nature":       "森林、海洋、天空、植物",
	"car":          "汽车、机车、赛道",
	"game":         "游戏、电竞、虚拟场景",
	"food":         "美食、甜点、饮品",
	"animal":       "动物、萌宠、野生生态",
	"travel":       "旅行、目的地、度假氛围",
	"space":        "星空、宇宙、科幻主题",
	"tech":         "科技、数码、未来感",
	"business":     "商务、办公、团队场景",
	"sports":       "运动、健身、赛事画面",
	"architecture": "建筑、室内、空间设计",

	// Natural & outdoors
	"ocean":    "海洋、海岸、波浪",
	"mountain": "山脉、峰顶、雪山",
	"forest":   "森林、树林、雾林",
	"desert":   "沙漠、戈壁、沙丘",
	"sunset":   "日落、日出、黄金时刻",
	"sky":      "天空、云朵、辽阔氛围",
	"flower":   "花卉、植物、花园",

	// People & lifestyle
	"portrait": "肖像、面部特写、影棚",
	"fashion":  "时尚、模特、潮流穿搭",
	"wedding":  "婚礼、新人、浪漫仪式",
	"yoga":     "瑜伽、冥想、舒缓氛围",
	"fitness":  "健身、力量、运动训练",
	"baby":     "婴儿、新生、可爱瞬间",
	"kids":     "儿童、家庭、欢乐时光",

	// Pets & wildlife
	"pet":      "宠物、伴侣动物、萌宠",
	"dog":      "狗狗、犬类、毛孩子",
	"cat":      "猫咪、喵星人、慵懒姿态",
	"wildlife": "野生动物、自然生态、纪实",

	// Food & drinks
	"coffee":  "咖啡、拿铁、咖啡馆",
	"dessert": "甜点、烘焙、蛋糕",
	"drink":   "饮品、鸡尾酒、果汁",
	"fruit":   "水果、新鲜食材、色彩",

	// Visual & artistic
	"abstract":   "抽象、色彩、艺术构图",
	"minimal":    "极简、留白、克制美学",
	"pattern":    "图案、几何、重复美感",
	"texture":    "纹理、材质、表面细节",
	"wallpaper":  "壁纸、桌面、屏保",
	"vintage":    "复古、胶片、怀旧氛围",
	"background": "纯色、渐变、底图背景",

	// Places & culture
	"interior":  "室内、家居、空间陈设",
	"street":    "街拍、城市纪实、人文",
	"nightlife": "夜生活、霓虹、城市灯火",
	"concert":   "演出、舞台、现场",
	"dance":     "舞蹈、身体表达、动态",
}

// typeIconClass returns the FontAwesome class string applied to the icon on
// the home page card. Uses the same "fa-light" set already loaded by the
// FontAwesome stylesheet.
var typeIconClass = map[string]string{
	// Original 16
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

	// Natural & outdoors
	"ocean":    "fa-light fa-water",
	"mountain": "fa-light fa-mountain",
	"forest":   "fa-light fa-trees",
	"desert":   "fa-light fa-cactus",
	"sunset":   "fa-light fa-sunset",
	"sky":      "fa-light fa-clouds",
	"flower":   "fa-light fa-flower",

	// People & lifestyle
	"portrait": "fa-light fa-user",
	"fashion":  "fa-light fa-shirt",
	"wedding":  "fa-light fa-rings-wedding",
	"yoga":     "fa-light fa-spa",
	"fitness":  "fa-light fa-dumbbell",
	"baby":     "fa-light fa-baby",
	"kids":     "fa-light fa-child",

	// Pets & wildlife
	"pet":      "fa-light fa-paw",
	"dog":      "fa-light fa-dog",
	"cat":      "fa-light fa-cat",
	"wildlife": "fa-light fa-feather",

	// Food & drinks
	"coffee":  "fa-light fa-mug-hot",
	"dessert": "fa-light fa-cake-candles",
	"drink":   "fa-light fa-martini-glass",
	"fruit":   "fa-light fa-apple-whole",

	// Visual & artistic
	"abstract":   "fa-light fa-shapes",
	"minimal":    "fa-light fa-circle-half-stroke",
	"pattern":    "fa-light fa-grid",
	"texture":    "fa-light fa-grip-lines",
	"wallpaper":  "fa-light fa-image-landscape",
	"vintage":    "fa-light fa-camera-retro",
	"background": "fa-light fa-square",

	// Places & culture
	"interior":  "fa-light fa-couch",
	"street":    "fa-light fa-road",
	"nightlife": "fa-light fa-moon-stars",
	"concert":   "fa-light fa-microphone-stand",
	"dance":     "fa-light fa-music",
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
