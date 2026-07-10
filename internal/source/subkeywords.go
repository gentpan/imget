package source

import "strings"

// typeSubKeywords expands each of the 20 top-level categories into a large set
// of narrower search phrases. These are used ONLY to widen the fetch surface:
// the topup job cycles through them so every run queries the upstream APIs with
// a fresh phrase, unlocking a new ~500-1k result window per phrase instead of
// re-sampling the same "latest" page for the parent keyword.
//
// Images fetched via a sub-keyword are still stored under the PARENT type
// (fetch writes to original/{type}/…, never keyed by keyword), so the public
// site keeps exactly the 20 buckets in AllowedTypes — sub-keywords never surface
// as categories on the homepage. This is the "汇总在总的" behaviour: sub-keywords
// feed volume, the parent bucket owns the result.
var typeSubKeywords = map[string][]string{
	"banner": {
		"abstract background", "gradient background", "geometric background",
		"bokeh background", "minimal background", "texture background",
		"color splash", "light streaks", "bubbles background", "smoke background",
		"paper texture", "marble texture", "wave pattern", "particle background",
		"blurred background", "neon background", "pastel background",
		"dark background", "watercolor background", "grid background",
	},
	"landscape": {
		"mountain range", "ocean sunset", "desert dunes", "forest path",
		"green valley", "lake reflection", "waterfall", "snowy peaks",
		"rolling hills", "coastal cliffs", "rice terraces", "canyon",
		"aurora borealis", "misty forest", "sand beach", "river bend",
		"volcano", "glacier", "meadow flowers", "starry night sky",
		"autumn forest", "tropical island", "grassland", "sunrise horizon",
	},
	"beauty": {
		"fashion model", "portrait woman", "makeup close up", "hair style",
		"skincare", "beauty salon", "cosmetics", "elegant dress",
		"studio portrait", "natural beauty", "glamour", "nail art",
		"perfume", "jewelry model", "spa relaxation", "lipstick",
		"bridal makeup", "street fashion", "male model", "editorial fashion",
	},
	"anime": {
		"anime girl", "anime landscape", "manga art", "chibi character",
		"anime city", "fantasy illustration", "cyberpunk anime", "kawaii art",
		"anime sky", "digital painting character", "cel shading art",
		"anime school", "mecha robot", "anime sword", "magical girl",
		"vaporwave art", "anime portrait", "fantasy warrior", "sci-fi illustration",
		"pixel art character",
	},
	"city": {
		"city skyline", "night street", "urban architecture", "neon city",
		"downtown", "city traffic", "rooftop view", "subway station",
		"crosswalk", "skyscraper", "old town street", "city bridge",
		"street market", "alley", "harbor city", "financial district",
		"city park", "european street", "asian street", "aerial city",
	},
	"nature": {
		"forest trees", "wildflowers", "close up leaf", "morning dew",
		"jungle", "bamboo forest", "cherry blossom", "sunflower field",
		"mushroom", "moss rocks", "rainforest", "spring bloom",
		"lavender field", "palm trees", "pine forest", "coral reef",
		"waterlily", "autumn leaves", "green plants", "botanical",
	},
	"car": {
		"sports car", "classic car", "supercar", "luxury car",
		"vintage car", "car interior", "electric car", "muscle car",
		"race car", "off road vehicle", "motorcycle", "car headlight",
		"drift car", "car wheel", "convertible", "car dashboard",
		"formula racing", "car on road", "car showroom", "truck",
	},
	"game": {
		"gaming setup", "esports arena", "game controller", "fantasy game world",
		"rpg landscape", "neon gaming", "virtual reality", "retro arcade",
		"game character", "battle scene", "sci-fi game", "gaming keyboard",
		"cyberpunk city game", "dungeon", "space game", "pixel game",
		"racing game", "strategy game map", "gaming pc", "console gaming",
	},
	"food": {
		"coffee cup", "dessert cake", "fresh fruit", "healthy salad",
		"pizza", "sushi", "burger", "breakfast table", "pasta",
		"chocolate", "ice cream", "bread bakery", "smoothie", "steak",
		"vegetables", "seafood", "noodles", "cocktail drink", "pancakes",
		"cheese platter", "street food", "tea ceremony", "barbecue",
	},
	"animal": {
		"dog portrait", "cat close up", "wild lion", "elephant",
		"bird flying", "horse running", "colorful fish", "butterfly",
		"panda", "fox", "owl", "deer", "tiger", "wolf",
		"rabbit", "penguin", "monkey", "sea turtle", "bear",
		"puppy", "kitten", "farm animals", "insect macro", "eagle",
	},
	"travel": {
		"tropical beach resort", "ancient temple", "mountain village",
		"famous landmark", "backpacker trail", "airport terminal",
		"road trip", "cruise ship", "hot air balloon", "safari",
		"snow resort", "historic castle", "island paradise", "desert camp",
		"lighthouse", "vineyard", "national park", "waterfall hike",
		"city travel", "camping tent", "train journey", "boat harbor",
	},
	"space": {
		"galaxy stars", "nebula", "planet surface", "milky way",
		"astronaut", "solar system", "moon surface", "rocket launch",
		"space station", "black hole art", "comet", "deep space",
		"earth from space", "star cluster", "cosmic dust", "supernova",
		"satellite orbit", "alien planet", "night sky stars", "aurora space",
	},
	"tech": {
		"circuit board", "artificial intelligence", "data center", "robot",
		"futuristic interface", "smartphone", "laptop code", "server room",
		"virtual reality headset", "drone", "microchip", "smart home",
		"cyber security", "network cables", "hologram", "3d printing",
		"electric grid", "quantum computing", "wearable tech", "digital abstract",
	},
	"business": {
		"office meeting", "handshake deal", "team collaboration", "startup workspace",
		"business woman", "conference room", "financial chart", "coworking space",
		"laptop desk", "presentation", "corporate building", "networking event",
		"remote work", "business plan", "office coffee", "modern office",
		"entrepreneur", "boardroom", "call center", "workspace desk",
	},
	"sports": {
		"soccer stadium", "basketball court", "running track", "yoga pose",
		"gym workout", "cycling road", "swimming pool", "tennis court",
		"marathon runner", "boxing", "surfing wave", "rock climbing",
		"skiing", "football game", "weight lifting", "skateboarding",
		"martial arts", "golf course", "baseball", "hiking trail",
	},
	"architecture": {
		"modern building", "interior design", "minimalist house", "glass facade",
		"staircase", "gothic cathedral", "concrete brutalism", "loft apartment",
		"skyscraper detail", "bridge structure", "courtyard", "temple architecture",
		"library interior", "museum building", "geometric ceiling", "villa exterior",
		"industrial interior", "arch corridor", "modern kitchen", "hotel lobby",
	},
	"wedding": {
		"bride portrait", "wedding ceremony", "wedding rings", "bridal bouquet",
		"wedding cake", "wedding venue", "couple romance", "wedding dress",
		"reception table", "wedding arch", "groom suit", "wedding flowers",
		"first dance", "wedding invitation", "engagement ring", "wedding toast",
		"outdoor wedding", "wedding decoration", "bridesmaids", "wedding kiss",
	},
	"kids": {
		"children playing", "baby portrait", "toddler smile", "kids drawing",
		"family picnic", "playground", "kids toys", "child reading",
		"newborn baby", "kids party", "children running", "school kids",
		"baby feet", "kids painting", "child laughing", "family beach",
		"kids classroom", "building blocks", "child balloon", "kids sports",
	},
	"abstract": {
		"fluid art", "geometric pattern", "colorful smoke", "minimal shapes",
		"3d render abstract", "liquid metal", "neon lines", "fractal",
		"paint splash", "glass refraction", "wireframe", "holographic",
		"crystal texture", "gradient mesh", "particle wave", "kaleidoscope",
		"metallic surface", "abstract light", "spiral pattern", "vintage texture",
	},
	"concert": {
		"concert crowd", "stage lights", "live band", "dj performance",
		"music festival", "singer microphone", "guitar player", "orchestra",
		"nightclub dance", "rock concert", "electronic music", "drummer",
		"stage smoke", "audience hands", "piano recital", "street musician",
		"laser show", "vinyl record", "recording studio", "saxophone",
	},
}

// SubKeywords returns the expansion list for a type, or nil if none.
func SubKeywords(typ string) []string {
	return typeSubKeywords[NormalizeType(typ)]
}

// PickSubKeyword returns one sub-keyword for typ chosen by idx (callers pass a
// rotating or random index). Falls back to the type's default multi-term phrase
// from typeKeywordMap when the type has no sub-keyword list. Returns "" only if
// the type is unknown and has no default, letting the provider use its own
// default handling.
func PickSubKeyword(typ string, idx int) string {
	subs := typeSubKeywords[NormalizeType(typ)]
	if len(subs) == 0 {
		return strings.TrimSpace(typeKeywordMap[NormalizeType(typ)])
	}
	if idx < 0 {
		idx = -idx
	}
	return subs[idx%len(subs)]
}
