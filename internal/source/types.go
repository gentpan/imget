// Package source defines image categories + provider selection (Pixabay/Picsum).
package source

import (
	"regexp"
	"strings"
)

// AllowedTypes is the curated 20-category set. The first 16 entries preserve
// the original PHP-compat order so existing profile keys keep working; the
// 4 trailing entries (wedding, kids, abstract, concert) were added when the
// pool grew large enough to justify them as standalone slots.
//
// Narrower sub-categories like dog/cat/coffee/mountain/etc were intentionally
// folded back into broader parents (animal / food / landscape …) via the
// keyword map below — the goal is "20 buckets a user can scan in one glance,"
// not 50 buckets that overlap. Operators who want a precise word should use
// the search box (it lets the caller pass an arbitrary `keyword=` override).
var AllowedTypes = []string{
	// Original 16 — PHP-compat order
	"banner", "landscape", "beauty", "anime", "city", "nature",
	"car", "game", "food", "animal", "travel", "space",
	"tech", "business", "sports", "architecture",

	// Curated additions (4)
	"wedding", "kids", "abstract", "concert",
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
// caller did not pass an explicit keyword. Each phrase is intentionally
// multi-term so the upstream APIs return a varied mix — e.g. "animal" pulls
// dogs, cats, pets, and wildlife together instead of forcing the user to
// choose a narrower sub-bucket.
var typeKeywordMap = map[string]string{
	"banner":       "",
	"landscape":    "landscape mountain ocean sunset desert forest sky",
	"beauty":       "portrait woman beauty fashion model",
	"anime":        "anime illustration manga art",
	"city":         "city skyline urban street nightlife",
	"nature":       "nature forest ocean sky plants flower",
	"car":          "car vehicle supercar racing automotive",
	"game":         "gaming esports virtual world neon",
	"food":         "food coffee dessert drink fruit cuisine restaurant",
	"animal":       "animal pet dog cat wildlife",
	"travel":       "travel destination vacation resort beach",
	"space":        "space galaxy universe stars nebula",
	"tech":         "technology digital futuristic device ai",
	"business":     "business office team meeting workspace",
	"sports":       "sports fitness yoga workout stadium",
	"architecture": "architecture interior building modern design",
	"wedding":      "wedding bride ceremony celebration romance",
	"kids":         "kids children baby family playing",
	"abstract":     "abstract pattern texture minimal wallpaper vintage",
	"concert":      "concert stage music performance dance",
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
// Empty string means "no category filter".
var pixabayCategoryMap = map[string]string{
	"beauty":       "people",
	"city":         "places",
	"nature":       "nature",
	"food":         "food",
	"travel":       "places",
	"sports":       "sports",
	"business":     "business",
	"architecture": "buildings",
	"wedding":      "people",
	"kids":         "people",
	"abstract":     "backgrounds",
	"concert":      "music",
	"animal":       "animals",
}

// PixabayCategory returns the Pixabay category for the given type, or "".
func PixabayCategory(typ string) string {
	return pixabayCategoryMap[NormalizeType(typ)]
}

// pixabayImageTypeMap selects between Pixabay's 'photo' and 'illustration'.
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
	"wedding":      "photo",
	"kids":         "photo",
	"abstract":     "illustration",
	"concert":      "photo",
}

func PixabayImageType(typ string) string {
	if v, ok := pixabayImageTypeMap[NormalizeType(typ)]; ok {
		return v
	}
	return "photo"
}
