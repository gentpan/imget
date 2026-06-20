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

var typeLocalizedLabel = map[string]map[string]string{
	"en": {
		"banner": "General banner", "landscape": "Landscape", "beauty": "Portraits", "anime": "Anime art", "city": "City", "nature": "Nature", "car": "Cars", "game": "Gaming", "food": "Food", "animal": "Animals", "travel": "Travel", "space": "Space", "tech": "Technology", "business": "Business", "sports": "Sports", "architecture": "Architecture", "wedding": "Wedding", "kids": "Kids and family", "abstract": "Abstract art", "concert": "Concert",
	},
	"ja": {
		"banner": "汎用バナー", "landscape": "風景", "beauty": "ポートレート", "anime": "アニメアート", "city": "都市", "nature": "自然", "car": "車", "game": "ゲーム", "food": "料理", "animal": "動物", "travel": "旅行", "space": "宇宙", "tech": "テクノロジー", "business": "ビジネス", "sports": "スポーツ", "architecture": "建築", "wedding": "ウェディング", "kids": "子どもと家族", "abstract": "抽象アート", "concert": "コンサート",
	},
	"de": {
		"banner": "Allgemeines Banner", "landscape": "Landschaft", "beauty": "Porträts", "anime": "Anime-Kunst", "city": "Stadt", "nature": "Natur", "car": "Autos", "game": "Gaming", "food": "Essen", "animal": "Tiere", "travel": "Reisen", "space": "Weltraum", "tech": "Technologie", "business": "Business", "sports": "Sport", "architecture": "Architektur", "wedding": "Hochzeit", "kids": "Kinder und Familie", "abstract": "Abstrakte Kunst", "concert": "Konzert",
	},
	"ru": {
		"banner": "Общий баннер", "landscape": "Пейзаж", "beauty": "Портреты", "anime": "Аниме-арт", "city": "Город", "nature": "Природа", "car": "Автомобили", "game": "Игры", "food": "Еда", "animal": "Животные", "travel": "Путешествия", "space": "Космос", "tech": "Технологии", "business": "Бизнес", "sports": "Спорт", "architecture": "Архитектура", "wedding": "Свадьба", "kids": "Дети и семья", "abstract": "Абстракция", "concert": "Концерт",
	},
	"pt": {
		"banner": "Banner geral", "landscape": "Paisagem", "beauty": "Retratos", "anime": "Arte anime", "city": "Cidade", "nature": "Natureza", "car": "Carros", "game": "Jogos", "food": "Comida", "animal": "Animais", "travel": "Viagem", "space": "Espaço", "tech": "Tecnologia", "business": "Negócios", "sports": "Esportes", "architecture": "Arquitetura", "wedding": "Casamento", "kids": "Crianças e família", "abstract": "Arte abstrata", "concert": "Concerto",
	},
	"es": {
		"banner": "Banner general", "landscape": "Paisaje", "beauty": "Retratos", "anime": "Arte anime", "city": "Ciudad", "nature": "Naturaleza", "car": "Coches", "game": "Videojuegos", "food": "Comida", "animal": "Animales", "travel": "Viajes", "space": "Espacio", "tech": "Tecnología", "business": "Negocios", "sports": "Deportes", "architecture": "Arquitectura", "wedding": "Boda", "kids": "Niños y familia", "abstract": "Arte abstracto", "concert": "Concierto",
	},
	"fr": {
		"banner": "Bannière générale", "landscape": "Paysage", "beauty": "Portraits", "anime": "Art anime", "city": "Ville", "nature": "Nature", "car": "Voitures", "game": "Jeux vidéo", "food": "Cuisine", "animal": "Animaux", "travel": "Voyage", "space": "Espace", "tech": "Technologie", "business": "Business", "sports": "Sport", "architecture": "Architecture", "wedding": "Mariage", "kids": "Enfants et famille", "abstract": "Art abstrait", "concert": "Concert",
	},
	"ko": {
		"banner": "일반 배너", "landscape": "풍경", "beauty": "인물", "anime": "애니메이션 아트", "city": "도시", "nature": "자연", "car": "자동차", "game": "게임", "food": "음식", "animal": "동물", "travel": "여행", "space": "우주", "tech": "기술", "business": "비즈니스", "sports": "스포츠", "architecture": "건축", "wedding": "웨딩", "kids": "어린이와 가족", "abstract": "추상 예술", "concert": "콘서트",
	},
}

var typeLocalizedDescription = map[string]map[string]string{
	"en": {
		"banner": "Flexible headers and default placeholders", "landscape": "Mountains, oceans, sunsets, forests, and deserts", "beauty": "Portraits, fashion, and people-focused imagery", "anime": "Anime, manga, illustration, and stylized art", "city": "Skylines, streets, nightlife, and urban scenes", "nature": "Forests, oceans, skies, flowers, and plants", "car": "Cars, motorcycles, supercars, and racing", "game": "Gaming, esports, neon, and virtual worlds", "food": "Cuisine, coffee, desserts, drinks, and fruit", "animal": "Pets, dogs, cats, and wildlife", "travel": "Destinations, resorts, beaches, and vacations", "space": "Stars, galaxies, nebulae, and sci-fi themes", "tech": "Digital devices, AI, futuristic scenes, and hardware", "business": "Offices, teams, meetings, and workspaces", "sports": "Fitness, yoga, workouts, stadiums, and events", "architecture": "Buildings, interiors, modern spaces, and design", "wedding": "Brides, ceremonies, romance, and celebration", "kids": "Children, babies, family moments, and play", "abstract": "Patterns, textures, minimal wallpapers, and vintage art", "concert": "Stages, music, dance, and live performance",
	},
	"ja": {
		"banner": "ヘッダーや標準プレースホルダー向け", "landscape": "山、海、夕日、森、砂漠", "beauty": "ポートレート、ファッション、人物写真", "anime": "アニメ、漫画、イラスト、スタイル化アート", "city": "スカイライン、街路、夜景、都市風景", "nature": "森、海、空、花、植物", "car": "車、バイク、スーパーカー、レース", "game": "ゲーム、eスポーツ、ネオン、仮想世界", "food": "料理、コーヒー、デザート、飲み物、果物", "animal": "ペット、犬、猫、野生動物", "travel": "旅行先、リゾート、ビーチ、休暇", "space": "星、銀河、星雲、SFテーマ", "tech": "デバイス、AI、未来感、ハードウェア", "business": "オフィス、チーム、会議、ワークスペース", "sports": "フィットネス、ヨガ、運動、スタジアム、競技", "architecture": "建物、インテリア、現代空間、デザイン", "wedding": "花嫁、式典、ロマンス、祝福", "kids": "子ども、赤ちゃん、家族、遊び", "abstract": "パターン、質感、ミニマル壁紙、レトロ", "concert": "ステージ、音楽、ダンス、ライブ演出",
	},
	"de": {
		"banner": "Flexible Header und Standard-Platzhalter", "landscape": "Berge, Meere, Sonnenuntergänge, Wälder und Wüsten", "beauty": "Porträts, Mode und menschenzentrierte Motive", "anime": "Anime, Manga, Illustration und stilisierte Kunst", "city": "Skylines, Straßen, Nachtleben und urbane Szenen", "nature": "Wälder, Meere, Himmel, Blumen und Pflanzen", "car": "Autos, Motorräder, Supersportwagen und Rennen", "game": "Gaming, Esports, Neon und virtuelle Welten", "food": "Küche, Kaffee, Desserts, Getränke und Obst", "animal": "Haustiere, Hunde, Katzen und Wildtiere", "travel": "Reiseziele, Resorts, Strände und Urlaub", "space": "Sterne, Galaxien, Nebel und Sci-Fi-Themen", "tech": "Digitale Geräte, KI, Zukunftsszenen und Hardware", "business": "Büros, Teams, Meetings und Arbeitsräume", "sports": "Fitness, Yoga, Training, Stadien und Events", "architecture": "Gebäude, Innenräume, moderne Räume und Design", "wedding": "Bräute, Zeremonien, Romantik und Feier", "kids": "Kinder, Babys, Familienmomente und Spiel", "abstract": "Muster, Texturen, minimalistische Wallpaper und Retro", "concert": "Bühnen, Musik, Tanz und Live-Auftritte",
	},
	"ru": {
		"banner": "Универсальные шапки и стандартные плейсхолдеры", "landscape": "Горы, океаны, закаты, леса и пустыни", "beauty": "Портреты, мода и изображения людей", "anime": "Аниме, манга, иллюстрации и стилизованное искусство", "city": "Скайлайны, улицы, ночная жизнь и городские сцены", "nature": "Леса, океаны, небо, цветы и растения", "car": "Авто, мотоциклы, суперкары и гонки", "game": "Игры, киберспорт, неон и виртуальные миры", "food": "Кухня, кофе, десерты, напитки и фрукты", "animal": "Питомцы, собаки, кошки и дикая природа", "travel": "Направления, курорты, пляжи и отпуск", "space": "Звезды, галактики, туманности и sci-fi", "tech": "Устройства, ИИ, футуризм и оборудование", "business": "Офисы, команды, встречи и рабочие места", "sports": "Фитнес, йога, тренировки, стадионы и события", "architecture": "Здания, интерьеры, современные пространства и дизайн", "wedding": "Невесты, церемонии, романтика и праздник", "kids": "Дети, младенцы, семейные моменты и игры", "abstract": "Узоры, текстуры, минималистичные обои и ретро", "concert": "Сцены, музыка, танцы и живые выступления",
	},
	"pt": {
		"banner": "Cabeçalhos flexíveis e placeholders padrão", "landscape": "Montanhas, oceanos, pôr do sol, florestas e desertos", "beauty": "Retratos, moda e imagens com pessoas", "anime": "Anime, mangá, ilustração e arte estilizada", "city": "Skylines, ruas, vida noturna e cenas urbanas", "nature": "Florestas, oceanos, céu, flores e plantas", "car": "Carros, motos, supercarros e corridas", "game": "Jogos, esports, neon e mundos virtuais", "food": "Culinária, café, sobremesas, bebidas e frutas", "animal": "Pets, cães, gatos e vida selvagem", "travel": "Destinos, resorts, praias e férias", "space": "Estrelas, galáxias, nebulosas e ficção científica", "tech": "Dispositivos, IA, cenas futuristas e hardware", "business": "Escritórios, equipes, reuniões e espaços de trabalho", "sports": "Fitness, yoga, treinos, estádios e eventos", "architecture": "Prédios, interiores, espaços modernos e design", "wedding": "Noivas, cerimônias, romance e celebração", "kids": "Crianças, bebês, momentos em família e brincadeiras", "abstract": "Padrões, texturas, wallpapers minimalistas e vintage", "concert": "Palcos, música, dança e apresentações ao vivo",
	},
	"es": {
		"banner": "Cabeceras flexibles y placeholders predeterminados", "landscape": "Montañas, océanos, atardeceres, bosques y desiertos", "beauty": "Retratos, moda e imágenes centradas en personas", "anime": "Anime, manga, ilustración y arte estilizado", "city": "Skylines, calles, vida nocturna y escenas urbanas", "nature": "Bosques, océanos, cielos, flores y plantas", "car": "Coches, motos, superdeportivos y carreras", "game": "Videojuegos, esports, neón y mundos virtuales", "food": "Cocina, café, postres, bebidas y frutas", "animal": "Mascotas, perros, gatos y vida salvaje", "travel": "Destinos, resorts, playas y vacaciones", "space": "Estrellas, galaxias, nebulosas y ciencia ficción", "tech": "Dispositivos, IA, escenas futuristas y hardware", "business": "Oficinas, equipos, reuniones y espacios de trabajo", "sports": "Fitness, yoga, entrenamientos, estadios y eventos", "architecture": "Edificios, interiores, espacios modernos y diseño", "wedding": "Novias, ceremonias, romance y celebración", "kids": "Niños, bebés, momentos familiares y juego", "abstract": "Patrones, texturas, wallpapers minimalistas y vintage", "concert": "Escenarios, música, danza y actuaciones en vivo",
	},
	"fr": {
		"banner": "En-têtes flexibles et placeholders par défaut", "landscape": "Montagnes, océans, couchers de soleil, forêts et déserts", "beauty": "Portraits, mode et images centrées sur les personnes", "anime": "Anime, manga, illustration et art stylisé", "city": "Skylines, rues, vie nocturne et scènes urbaines", "nature": "Forêts, océans, ciels, fleurs et plantes", "car": "Voitures, motos, supercars et courses", "game": "Jeux vidéo, esports, néon et mondes virtuels", "food": "Cuisine, café, desserts, boissons et fruits", "animal": "Animaux de compagnie, chiens, chats et faune", "travel": "Destinations, resorts, plages et vacances", "space": "Étoiles, galaxies, nébuleuses et science-fiction", "tech": "Appareils, IA, scènes futuristes et matériel", "business": "Bureaux, équipes, réunions et espaces de travail", "sports": "Fitness, yoga, entraînements, stades et événements", "architecture": "Bâtiments, intérieurs, espaces modernes et design", "wedding": "Mariées, cérémonies, romance et célébration", "kids": "Enfants, bébés, moments en famille et jeu", "abstract": "Motifs, textures, wallpapers minimalistes et vintage", "concert": "Scènes, musique, danse et performances live",
	},
	"ko": {
		"banner": "범용 헤더와 기본 플레이스홀더", "landscape": "산, 바다, 일몰, 숲, 사막", "beauty": "인물, 패션, 사람 중심 이미지", "anime": "애니메이션, 만화, 일러스트, 스타일 아트", "city": "스카이라인, 거리, 야경, 도시 장면", "nature": "숲, 바다, 하늘, 꽃, 식물", "car": "자동차, 오토바이, 슈퍼카, 레이싱", "game": "게임, e스포츠, 네온, 가상 세계", "food": "요리, 커피, 디저트, 음료, 과일", "animal": "반려동물, 강아지, 고양이, 야생동물", "travel": "여행지, 리조트, 해변, 휴가", "space": "별, 은하, 성운, SF 테마", "tech": "디지털 기기, AI, 미래 장면, 하드웨어", "business": "사무실, 팀, 회의, 업무 공간", "sports": "피트니스, 요가, 운동, 경기장, 이벤트", "architecture": "건물, 인테리어, 현대 공간, 디자인", "wedding": "신부, 예식, 로맨스, 축하", "kids": "어린이, 아기, 가족 순간, 놀이", "abstract": "패턴, 질감, 미니멀 배경, 빈티지", "concert": "무대, 음악, 춤, 라이브 공연",
	},
}

func TypeLocalizedLabel(locale, typ string) string {
	typ = strings.ToLower(strings.TrimSpace(typ))
	locale = strings.ToLower(strings.TrimSpace(locale))
	if locale == "zh" {
		return TypeChineseLabel(typ)
	}
	if labels, ok := typeLocalizedLabel[locale]; ok {
		if v, ok := labels[typ]; ok {
			return v
		}
	}
	return TypeChineseLabel(typ)
}

func TypeLocalizedDescription(locale, typ string) string {
	typ = strings.ToLower(strings.TrimSpace(typ))
	locale = strings.ToLower(strings.TrimSpace(locale))
	if locale == "zh" {
		return TypeChineseDescription(typ)
	}
	if descriptions, ok := typeLocalizedDescription[locale]; ok {
		if v, ok := descriptions[typ]; ok {
			return v
		}
	}
	return TypeLocalizedLabel(locale, typ)
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
