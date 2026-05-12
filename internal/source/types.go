// Package source defines image categories + provider selection (Pixabay/Picsum).
package source

import (
	"regexp"
	"strings"
)

// AllowedTypes is the canonical type set. The first 16 entries preserve the
// original PHP-compat order so existing profile keys keep working; entries
// after that were added for the 50-category expansion.
var AllowedTypes = []string{
	// Original 16 — PHP-compat order
	"banner", "landscape", "beauty", "anime", "city", "nature",
	"car", "game", "food", "animal", "travel", "space",
	"tech", "business", "sports", "architecture",

	// Natural & outdoors (+7)
	"ocean", "mountain", "forest", "desert", "sunset", "sky", "flower",

	// People & lifestyle (+7)
	"portrait", "fashion", "wedding", "yoga", "fitness", "baby", "kids",

	// Pets & wildlife (+4)
	"pet", "dog", "cat", "wildlife",

	// Food & drinks (+4)
	"coffee", "dessert", "drink", "fruit",

	// Visual & artistic (+7)
	"abstract", "minimal", "pattern", "texture", "wallpaper", "vintage", "background",

	// Places & culture (+5)
	"interior", "street", "nightlife", "concert", "dance",
}

var allowedSet = func() map[string]struct{} {
	m := make(map[string]struct{}, len(AllowedTypes))
	for _, t := range AllowedTypes {
		m[t] = struct{}{}
	}
	return m
}()

var typeSlugRe = regexp.MustCompile(`[^a-z0-9_-]+`)

// NormalizeType lowercases, strips invalid characters, and falls back to "banner".
func NormalizeType(t string) string {
	t = strings.ToLower(t)
	t = typeSlugRe.ReplaceAllString(t, "")
	if t == "" {
		return "banner"
	}
	if _, ok := allowedSet[t]; !ok {
		return "banner"
	}
	return t
}

// IsAllowedType is true if t is exactly in the canonical list (no normalization).
func IsAllowedType(t string) bool {
	_, ok := allowedSet[strings.ToLower(t)]
	return ok
}

// typeKeywordMap maps a category to the implicit search keywords used when the
// caller did not pass an explicit keyword. Phrases are tuned to bias Pexels +
// Pixabay results toward the visual we want for that label.
var typeKeywordMap = map[string]string{
	// Original 16
	"banner":       "",
	"landscape":    "landscape nature mountain lake",
	"beauty":       "portrait woman beauty fashion",
	"anime":        "anime illustration manga art",
	"city":         "city skyline urban architecture street",
	"nature":       "nature forest ocean sky plants",
	"car":          "car vehicle supercar racing automotive",
	"game":         "gaming esports virtual world neon",
	"food":         "food dessert drink restaurant cuisine",
	"animal":       "animal pet wildlife nature",
	"travel":       "travel destination vacation resort beach",
	"space":        "space galaxy universe stars nebula",
	"tech":         "technology digital futuristic device ai",
	"business":     "business office team meeting workspace",
	"sports":       "sports fitness workout stadium competition",
	"architecture": "architecture interior building modern design",

	// Natural & outdoors
	"ocean":    "ocean sea beach wave coast",
	"mountain": "mountain peak alpine snow ridge",
	"forest":   "forest woodland trees mist canopy",
	"desert":   "desert dune sand canyon",
	"sunset":   "sunset sunrise golden hour sky",
	"sky":      "sky clouds blue atmosphere",
	"flower":   "flower bloom floral garden petal",

	// People & lifestyle
	"portrait": "portrait person face headshot studio",
	"fashion":  "fashion model style outfit editorial",
	"wedding":  "wedding bride ceremony celebration romance",
	"yoga":     "yoga meditation wellness mindful stretch",
	"fitness":  "fitness gym workout exercise strength",
	"baby":     "baby infant newborn smile portrait",
	"kids":     "kids children playing happy family",

	// Pets & wildlife
	"pet":      "pet animal companion cute fluffy",
	"dog":      "dog puppy canine retriever portrait",
	"cat":      "cat kitten feline portrait curious",
	"wildlife": "wildlife wild safari animal nature",

	// Food & drinks
	"coffee":  "coffee espresso cafe latte cup",
	"dessert": "dessert cake pastry sweet bakery",
	"drink":   "drink cocktail beverage juice glass",
	"fruit":   "fruit fresh tropical organic colorful",

	// Visual & artistic
	"abstract":   "abstract art shapes colorful modern",
	"minimal":    "minimal minimalist clean simple modern",
	"pattern":    "pattern repeat geometric design wallpaper",
	"texture":    "texture surface material grunge close up",
	"wallpaper":  "wallpaper background scenic colorful",
	"vintage":    "vintage retro analog film nostalgic",
	"background": "background gradient backdrop solid abstract",

	// Places & culture
	"interior":  "interior design room living modern",
	"street":    "street photography urban candid city",
	"nightlife": "nightlife neon city lights nocturnal",
	"concert":   "concert stage music performance crowd",
	"dance":     "dance dancer performance motion studio",
}

// ResolveKeyword returns the explicit keyword if non-empty, otherwise the
// type-default phrase from typeKeywordMap.
func ResolveKeyword(typ, explicit string) string {
	explicit = strings.TrimSpace(explicit)
	if explicit != "" {
		return explicit
	}
	return typeKeywordMap[NormalizeType(typ)]
}

// pixabayCategoryMap maps a category to a Pixabay 'category' query value.
// Empty string means "no category filter". Only types whose Pixabay category
// improves relevance are listed; others rely on the keyword alone.
var pixabayCategoryMap = map[string]string{
	// Original 16
	"beauty":       "people",
	"city":         "places",
	"nature":       "nature",
	"food":         "food",
	"travel":       "places",
	"sports":       "sports",
	"business":     "business",
	"architecture": "buildings",

	// Natural & outdoors
	"ocean":    "nature",
	"mountain": "nature",
	"forest":   "nature",
	"desert":   "nature",
	"sunset":   "nature",
	"sky":      "nature",
	"flower":   "nature",

	// People & lifestyle
	"portrait": "people",
	"fashion":  "fashion",
	"wedding":  "people",
	"yoga":     "health",
	"fitness":  "sports",
	"baby":     "people",
	"kids":     "people",

	// Pets & wildlife
	"pet":      "animals",
	"dog":      "animals",
	"cat":      "animals",
	"wildlife": "animals",

	// Food & drinks
	"coffee":  "food",
	"dessert": "food",
	"drink":   "food",
	"fruit":   "food",

	// Visual & artistic
	"abstract":   "backgrounds",
	"minimal":    "backgrounds",
	"pattern":    "backgrounds",
	"texture":    "backgrounds",
	"wallpaper":  "backgrounds",
	"background": "backgrounds",

	// Places & culture
	"interior":  "buildings",
	"street":    "places",
	"nightlife": "places",
	"concert":   "music",
	"dance":     "music",
}

// PixabayCategory returns the Pixabay category for the given type, or "".
func PixabayCategory(typ string) string {
	return pixabayCategoryMap[NormalizeType(typ)]
}

// pixabayImageTypeMap selects between Pixabay's 'photo' and 'illustration'.
// New types all default to "photo" unless explicitly listed — photos pull
// the largest curated set and look natural at 4K+.
var pixabayImageTypeMap = map[string]string{
	"banner":       "photo",
	"landscape":    "photo",
	"beauty":       "photo",
	"anime":        "illustration",
	"city":         "photo",
	"nature":       "photo",
	"car":          "photo",
	"game":         "illustration",
	"food":         "photo",
	"animal":       "photo",
	"travel":       "photo",
	"space":        "illustration",
	"tech":         "illustration",
	"business":     "photo",
	"sports":       "photo",
	"architecture": "photo",

	// Visual & artistic categories often look best as illustration on Pixabay.
	"abstract":   "illustration",
	"pattern":    "illustration",
	"texture":    "illustration",
	"wallpaper":  "illustration",
	"background": "illustration",
}

func PixabayImageType(typ string) string {
	if v, ok := pixabayImageTypeMap[NormalizeType(typ)]; ok {
		return v
	}
	return "photo"
}
