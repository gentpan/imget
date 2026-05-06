// Package source defines image categories + provider selection (Pixabay/Picsum).
package source

import (
	"regexp"
	"strings"
)

// AllowedTypes is the canonical 16-type set, in PHP-compat order.
var AllowedTypes = []string{
	"banner", "landscape", "beauty", "anime", "city", "nature",
	"car", "game", "food", "animal", "travel", "space",
	"tech", "business", "sports", "architecture",
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
// caller did not pass an explicit keyword.
var typeKeywordMap = map[string]string{
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
}

func PixabayImageType(typ string) string {
	if v, ok := pixabayImageTypeMap[NormalizeType(typ)]; ok {
		return v
	}
	return "photo"
}
