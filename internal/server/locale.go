package server

import "strings"

type LanguageLink struct {
	Code    string
	Label   string
	Native  string
	Flag    string
	URL     string
	Current bool
}

type LocalizedHome struct {
	Code                  string
	HTMLLang              string
	OGLocale              string
	Name                  string
	Title                 string
	Description           string
	Keywords              string
	HeaderAria            string
	FeatureAria           string
	FeatureSize           string
	FeatureType           string
	FeatureFixed          string
	FeatureFormat         string
	FeatureCDN            string
	UsageTitle            string
	UsageLead             string
	SummaryBase           string
	SummaryFixed          string
	SummaryType           string
	SummarySlot           string
	SummaryFormat         string
	GeneratorTitle        string
	GeneratorLead         string
	WidthLabel            string
	HeightLabel           string
	TypeLabel             string
	DefaultType           string
	FormatLabel           string
	DefaultFormat         string
	ModeLabel             string
	RandomMode            string
	FixedMode             string
	SlotLabel             string
	KeywordLabel          string
	CopyAddress           string
	OpenTest              string
	ExamplesTitle         string
	HTMLExampleTitle      string
	MarkdownExampleTitle  string
	CSSExampleTitle       string
	WallpaperTitle        string
	WallpaperLead         string
	FormatSectionTitle    string
	FormatLead            string
	TypesTitle            string
	TypesLead             string
	SamplesTitle          string
	RandomSample          string
	TypedSample           string
	FixedSample           string
	FixedTypedSample      string
	AVIFSample            string
	WebPSample            string
	SampleLink            string
	CopyrightTitle        string
	CopyrightCopy         string
	PrivacyTitle          string
	PrivacyCopy           string
	FooterStatsAria       string
	ImagesLabel           string
	StorageLabel          string
	FooterInfo            string
	UsageModalTitle       string
	CopyrightModalTitle   string
	PrivacyModalTitle     string
	CloseModalAria        string
	LanguageSwitcherLabel string
}

var supportedLocalizedHomes = []LocalizedHome{
	{
		Code: "zh", HTMLLang: "zh-CN", OGLocale: "zh_CN", Name: "中文",
		Title:       "{{site}} - 动态占位图与随机图片服务",
		Description: "{{name}} 提供按尺寸、分类、固定参数生成图片的服务，支持 WebP 和 AVIF 输出。",
		Keywords:    "{{name}},随机图片,占位图,动态图片,图片裁剪,WebP,AVIF",
		HeaderAria:  "首页", FeatureAria: "核心功能",
		FeatureSize: "支持尺寸裁剪", FeatureType: "支持分类取图", FeatureFixed: "支持固定参数取图", FeatureFormat: "支持 WebP / AVIF", FeatureCDN: "全球 CDN 加速",
		UsageTitle: "使用方式", UsageLead: "直接在域名 {{host}} 后写上尺寸即可使用，默认会直接返回图片文件。你可以追加 type、r、s、format 来控制图片类型、固定规则、多图位置和输出格式。",
		SummaryBase: "基础格式", SummaryFixed: "固定参数", SummaryType: "类型参数", SummarySlot: "多图参数", SummaryFormat: "格式参数",
		GeneratorTitle: "在线生成地址", GeneratorLead: "输入尺寸并选择类型、格式和方式，系统会实时生成可访问地址。",
		WidthLabel: "宽度", HeightLabel: "高度", TypeLabel: "类型", DefaultType: "默认", FormatLabel: "格式", DefaultFormat: "webp（默认）", ModeLabel: "方式", RandomMode: "随机", FixedMode: "固定 r", SlotLabel: "s 值", KeywordLabel: "自定义关键词",
		CopyAddress: "复制地址", OpenTest: "访问测试",
		ExamplesTitle: "代码引用", HTMLExampleTitle: "HTML — 随机头图", MarkdownExampleTitle: "Markdown — 文章配图", CSSExampleTitle: "CSS — 背景图",
		WallpaperTitle: "壁纸模式（原始分辨率）", WallpaperLead: "如果你只想直接拿到一张高清原图，用分类短链即可。",
		FormatSectionTitle: "格式", FormatLead: "当前服务器支持 WebP 和 AVIF 输出，所有格式都由当前域名直接输出。",
		TypesTitle: "类型", TypesLead: "支持通过 type 参数按分类输出图片。下面是全部分类卡片，点击即可查看对应原始高清图。",
		SamplesTitle: "示例图片", RandomSample: "随机图", TypedSample: "指定类型随机图", FixedSample: "固定图", FixedTypedSample: "固定类型图", AVIFSample: "AVIF 输出", WebPSample: "WebP 输出", SampleLink: "示例链接",
		CopyrightTitle: "版权说明", CopyrightCopy: "本服务提供图片调用、尺寸生成与格式转换能力。图片权利通常归原作者、来源平台或合法权利人所有；使用前请自行确认授权范围。",
		PrivacyTitle: "隐私说明", PrivacyCopy: "站点仅保留维持服务运行所需的基础请求信息与统计信息，用于缓存优化、故障排查和访问分析。",
		FooterStatsAria: "图库统计", ImagesLabel: "张图片", StorageLabel: "占用空间", FooterInfo: "Info", UsageModalTitle: "使用说明", CopyrightModalTitle: "版权说明", PrivacyModalTitle: "隐私说明", CloseModalAria: "关闭说明弹窗", LanguageSwitcherLabel: "语言",
	},
	{
		Code: "en", HTMLLang: "en", OGLocale: "en_US", Name: "English",
		Title:       "{{site}} - Dynamic placeholder and random image service",
		Description: "{{name}} generates images by size, category, stable keys, and modern output formats such as WebP and AVIF.",
		Keywords:    "{{name}},random images,placeholder images,dynamic images,image crop,WebP,AVIF",
		HeaderAria:  "Home", FeatureAria: "Core features",
		FeatureSize: "Size-based crops", FeatureType: "Category images", FeatureFixed: "Stable r parameter", FeatureFormat: "WebP / AVIF", FeatureCDN: "Global CDN",
		UsageTitle: "How To Use", UsageLead: "Add a size after {{host}} and the URL returns an image file. Use type, r, s, and format to control category, stable selection, slots, and output format.",
		SummaryBase: "Base path", SummaryFixed: "Stable key", SummaryType: "Category", SummarySlot: "Slot", SummaryFormat: "Format",
		GeneratorTitle: "URL Builder", GeneratorLead: "Enter dimensions and choose a type, format, and mode. The page builds a ready-to-use image URL instantly.",
		WidthLabel: "Width", HeightLabel: "Height", TypeLabel: "Type", DefaultType: "Default", FormatLabel: "Format", DefaultFormat: "webp (default)", ModeLabel: "Mode", RandomMode: "Random", FixedMode: "Fixed r", SlotLabel: "s value", KeywordLabel: "Custom keyword",
		CopyAddress: "Copy URL", OpenTest: "Open test",
		ExamplesTitle: "Code Examples", HTMLExampleTitle: "HTML - random hero image", MarkdownExampleTitle: "Markdown - article image", CSSExampleTitle: "CSS - background image",
		WallpaperTitle: "Wallpaper Mode (original resolution)", WallpaperLead: "For wallpapers, desktop rotation, or raw high-resolution images, use the category short link.",
		FormatSectionTitle: "Formats", FormatLead: "The server can output WebP and AVIF directly from the same domain.",
		TypesTitle: "Types", TypesLead: "Use the type parameter to request a category. Click any card to open a full-resolution original.",
		SamplesTitle: "Sample Images", RandomSample: "Random image", TypedSample: "Typed random image", FixedSample: "Fixed image", FixedTypedSample: "Fixed typed image", AVIFSample: "AVIF output", WebPSample: "WebP output", SampleLink: "Sample link",
		CopyrightTitle: "Copyright", CopyrightCopy: "This service provides image delivery, resizing, and format conversion. Image rights usually belong to the original author, source platform, or rights holder; verify permissions before use.",
		PrivacyTitle: "Privacy", PrivacyCopy: "The site stores only basic technical request and analytics data needed for operation, cache optimization, troubleshooting, and service analysis.",
		FooterStatsAria: "Library stats", ImagesLabel: "images", StorageLabel: "stored", FooterInfo: "Info", UsageModalTitle: "Usage", CopyrightModalTitle: "Copyright", PrivacyModalTitle: "Privacy", CloseModalAria: "Close dialog", LanguageSwitcherLabel: "Language",
	},
	{
		Code: "ja", HTMLLang: "ja", OGLocale: "ja_JP", Name: "日本語",
		Title: "{{site}} - 動的プレースホルダーとランダム画像サービス", Description: "{{name}} はサイズ、カテゴリ、固定パラメータ、WebP/AVIF 形式で画像を生成します。", Keywords: "{{name}},ランダム画像,プレースホルダー,画像生成,WebP,AVIF",
		HeaderAria: "ホーム", FeatureAria: "主な機能", FeatureSize: "サイズ指定", FeatureType: "カテゴリ画像", FeatureFixed: "固定 r パラメータ", FeatureFormat: "WebP / AVIF", FeatureCDN: "グローバル CDN",
		UsageTitle: "使い方", UsageLead: "{{host}} の後ろにサイズを付けるだけで画像ファイルを返します。type、r、s、format でカテゴリ、固定選択、スロット、形式を指定できます。",
		SummaryBase: "基本形式", SummaryFixed: "固定キー", SummaryType: "カテゴリ", SummarySlot: "スロット", SummaryFormat: "形式", GeneratorTitle: "URL 生成", GeneratorLead: "サイズ、タイプ、形式、方式を選ぶと利用可能な画像 URL を即時生成します。",
		WidthLabel: "幅", HeightLabel: "高さ", TypeLabel: "タイプ", DefaultType: "デフォルト", FormatLabel: "形式", DefaultFormat: "webp（既定）", ModeLabel: "方式", RandomMode: "ランダム", FixedMode: "固定 r", SlotLabel: "s 値", KeywordLabel: "カスタムキーワード",
		CopyAddress: "URL をコピー", OpenTest: "開いて確認", ExamplesTitle: "コード例", HTMLExampleTitle: "HTML - ランダム画像", MarkdownExampleTitle: "Markdown - 記事画像", CSSExampleTitle: "CSS - 背景画像",
		WallpaperTitle: "壁紙モード（元解像度）", WallpaperLead: "高解像度の元画像が必要な場合はカテゴリ短縮リンクを使います。", FormatSectionTitle: "形式", FormatLead: "WebP と AVIF を同じドメインから直接出力できます。",
		TypesTitle: "タイプ", TypesLead: "type パラメータでカテゴリ別画像を取得できます。カードをクリックすると元解像度画像を開きます。", SamplesTitle: "サンプル画像", RandomSample: "ランダム画像", TypedSample: "カテゴリ付きランダム", FixedSample: "固定画像", FixedTypedSample: "固定カテゴリ画像", AVIFSample: "AVIF 出力", WebPSample: "WebP 出力", SampleLink: "サンプルリンク",
		CopyrightTitle: "著作権", CopyrightCopy: "本サービスは画像配信、リサイズ、形式変換を提供します。画像の権利は通常、原作者または提供元に帰属します。利用前に権利を確認してください。", PrivacyTitle: "プライバシー", PrivacyCopy: "運用、キャッシュ最適化、障害調査、分析に必要な最小限の技術情報のみを保存します。",
		FooterStatsAria: "ライブラリ統計", ImagesLabel: "枚の画像", StorageLabel: "保存容量", FooterInfo: "情報", UsageModalTitle: "使い方", CopyrightModalTitle: "著作権", PrivacyModalTitle: "プライバシー", CloseModalAria: "閉じる", LanguageSwitcherLabel: "言語",
	},
	{
		Code: "de", HTMLLang: "de", OGLocale: "de_DE", Name: "Deutsch",
		Title: "{{site}} - Dynamischer Platzhalter- und Zufallsbilddienst", Description: "{{name}} erzeugt Bilder nach Größe, Kategorie, stabilem Schlüssel und WebP/AVIF-Ausgabe.", Keywords: "{{name}},Zufallsbilder,Platzhalterbilder,WebP,AVIF",
		HeaderAria: "Startseite", FeatureAria: "Kernfunktionen", FeatureSize: "Größenzuschnitt", FeatureType: "Kategorien", FeatureFixed: "Stabiler r-Parameter", FeatureFormat: "WebP / AVIF", FeatureCDN: "Globales CDN",
		UsageTitle: "Verwendung", UsageLead: "Füge nach {{host}} eine Größe an, und die URL liefert direkt eine Bilddatei. Mit type, r, s und format steuerst du Kategorie, feste Auswahl, Slots und Ausgabeformat.",
		SummaryBase: "Basis", SummaryFixed: "Fester Schlüssel", SummaryType: "Kategorie", SummarySlot: "Slot", SummaryFormat: "Format", GeneratorTitle: "URL-Generator", GeneratorLead: "Gib Maße ein und wähle Typ, Format und Modus. Die Seite erzeugt sofort eine nutzbare Bild-URL.",
		WidthLabel: "Breite", HeightLabel: "Höhe", TypeLabel: "Typ", DefaultType: "Standard", FormatLabel: "Format", DefaultFormat: "webp (Standard)", ModeLabel: "Modus", RandomMode: "Zufällig", FixedMode: "Festes r", SlotLabel: "s-Wert", KeywordLabel: "Eigenes Keyword",
		CopyAddress: "URL kopieren", OpenTest: "Test öffnen", ExamplesTitle: "Codebeispiele", HTMLExampleTitle: "HTML - zufälliges Hero-Bild", MarkdownExampleTitle: "Markdown - Artikelbild", CSSExampleTitle: "CSS - Hintergrundbild",
		WallpaperTitle: "Wallpaper-Modus (Originalauflösung)", WallpaperLead: "Für Wallpaper oder rohe hochauflösende Bilder nutze den Kategorie-Kurzlink.", FormatSectionTitle: "Formate", FormatLead: "Der Server liefert WebP und AVIF direkt über dieselbe Domain.",
		TypesTitle: "Typen", TypesLead: "Mit dem type-Parameter forderst du Kategorien an. Klicke eine Karte, um ein Originalbild zu öffnen.", SamplesTitle: "Beispielbilder", RandomSample: "Zufallsbild", TypedSample: "Zufallsbild mit Typ", FixedSample: "Festes Bild", FixedTypedSample: "Festes Bild mit Typ", AVIFSample: "AVIF-Ausgabe", WebPSample: "WebP-Ausgabe", SampleLink: "Beispiellink",
		CopyrightTitle: "Copyright", CopyrightCopy: "Der Dienst liefert Bilder, Zuschnitt und Formatkonvertierung. Rechte liegen üblicherweise bei Urhebern, Plattformen oder Rechteinhabern; prüfe die Erlaubnis vor der Nutzung.", PrivacyTitle: "Datenschutz", PrivacyCopy: "Gespeichert werden nur technische Basisdaten für Betrieb, Cache-Optimierung, Fehleranalyse und Statistik.",
		FooterStatsAria: "Bibliotheksstatistik", ImagesLabel: "Bilder", StorageLabel: "Speicher", FooterInfo: "Info", UsageModalTitle: "Nutzung", CopyrightModalTitle: "Copyright", PrivacyModalTitle: "Datenschutz", CloseModalAria: "Dialog schließen", LanguageSwitcherLabel: "Sprache",
	},
	{
		Code: "ru", HTMLLang: "ru", OGLocale: "ru_RU", Name: "Русский",
		Title: "{{site}} - динамические плейсхолдеры и случайные изображения", Description: "{{name}} создает изображения по размеру, категории, фиксированному ключу и форматам WebP/AVIF.", Keywords: "{{name}},случайные изображения,плейсхолдеры,WebP,AVIF",
		HeaderAria: "Главная", FeatureAria: "Основные функции", FeatureSize: "Обрезка по размеру", FeatureType: "Категории", FeatureFixed: "Фиксированный r", FeatureFormat: "WebP / AVIF", FeatureCDN: "Глобальный CDN",
		UsageTitle: "Как использовать", UsageLead: "Добавьте размер после {{host}}, и URL вернет файл изображения. Параметры type, r, s и format управляют категорией, стабильным выбором, слотами и форматом.",
		SummaryBase: "Базовый путь", SummaryFixed: "Фиксированный ключ", SummaryType: "Категория", SummarySlot: "Слот", SummaryFormat: "Формат", GeneratorTitle: "Генератор URL", GeneratorLead: "Введите размеры и выберите тип, формат и режим. Страница сразу создаст рабочий URL.",
		WidthLabel: "Ширина", HeightLabel: "Высота", TypeLabel: "Тип", DefaultType: "По умолчанию", FormatLabel: "Формат", DefaultFormat: "webp (по умолчанию)", ModeLabel: "Режим", RandomMode: "Случайно", FixedMode: "Фикс. r", SlotLabel: "Значение s", KeywordLabel: "Ключевое слово",
		CopyAddress: "Копировать URL", OpenTest: "Открыть тест", ExamplesTitle: "Примеры кода", HTMLExampleTitle: "HTML - случайное изображение", MarkdownExampleTitle: "Markdown - изображение статьи", CSSExampleTitle: "CSS - фон",
		WallpaperTitle: "Режим обоев (оригинальное разрешение)", WallpaperLead: "Для обоев и исходных изображений высокого разрешения используйте короткую ссылку категории.", FormatSectionTitle: "Форматы", FormatLead: "Сервер напрямую отдает WebP и AVIF с того же домена.",
		TypesTitle: "Типы", TypesLead: "Параметр type выбирает категорию. Нажмите карточку, чтобы открыть оригинал.", SamplesTitle: "Примеры изображений", RandomSample: "Случайное", TypedSample: "Случайное по типу", FixedSample: "Фиксированное", FixedTypedSample: "Фиксированное по типу", AVIFSample: "AVIF", WebPSample: "WebP", SampleLink: "Ссылка",
		CopyrightTitle: "Авторские права", CopyrightCopy: "Сервис предоставляет доставку, изменение размера и конвертацию формата. Права обычно принадлежат авторам, платформам или правообладателям; проверьте разрешения перед использованием.", PrivacyTitle: "Конфиденциальность", PrivacyCopy: "Хранятся только базовые технические данные для работы, кэша, диагностики и аналитики.",
		FooterStatsAria: "Статистика", ImagesLabel: "изображений", StorageLabel: "хранилище", FooterInfo: "Инфо", UsageModalTitle: "Использование", CopyrightModalTitle: "Права", PrivacyModalTitle: "Приватность", CloseModalAria: "Закрыть", LanguageSwitcherLabel: "Язык",
	},
	{
		Code: "pt", HTMLLang: "pt", OGLocale: "pt_PT", Name: "Português",
		Title: "{{site}} - serviço dinâmico de placeholders e imagens aleatórias", Description: "{{name}} gera imagens por tamanho, categoria, chave fixa e formatos WebP/AVIF.", Keywords: "{{name}},imagens aleatórias,placeholder,WebP,AVIF",
		HeaderAria: "Início", FeatureAria: "Recursos principais", FeatureSize: "Recorte por tamanho", FeatureType: "Categorias", FeatureFixed: "Parâmetro r fixo", FeatureFormat: "WebP / AVIF", FeatureCDN: "CDN global",
		UsageTitle: "Como usar", UsageLead: "Adicione um tamanho após {{host}} e a URL retorna um arquivo de imagem. Use type, r, s e format para controlar categoria, seleção fixa, slots e formato.",
		SummaryBase: "Caminho base", SummaryFixed: "Chave fixa", SummaryType: "Categoria", SummarySlot: "Slot", SummaryFormat: "Formato", GeneratorTitle: "Gerador de URL", GeneratorLead: "Informe dimensões e escolha tipo, formato e modo. A página cria uma URL pronta para uso.",
		WidthLabel: "Largura", HeightLabel: "Altura", TypeLabel: "Tipo", DefaultType: "Padrão", FormatLabel: "Formato", DefaultFormat: "webp (padrão)", ModeLabel: "Modo", RandomMode: "Aleatório", FixedMode: "r fixo", SlotLabel: "Valor s", KeywordLabel: "Palavra-chave",
		CopyAddress: "Copiar URL", OpenTest: "Abrir teste", ExamplesTitle: "Exemplos de código", HTMLExampleTitle: "HTML - imagem aleatória", MarkdownExampleTitle: "Markdown - imagem de artigo", CSSExampleTitle: "CSS - imagem de fundo",
		WallpaperTitle: "Modo wallpaper (resolução original)", WallpaperLead: "Para wallpapers ou imagens brutas em alta resolução, use o link curto da categoria.", FormatSectionTitle: "Formatos", FormatLead: "O servidor entrega WebP e AVIF diretamente pelo mesmo domínio.",
		TypesTitle: "Tipos", TypesLead: "Use o parâmetro type para solicitar uma categoria. Clique em um cartão para abrir um original.", SamplesTitle: "Imagens de exemplo", RandomSample: "Imagem aleatória", TypedSample: "Aleatória por tipo", FixedSample: "Imagem fixa", FixedTypedSample: "Fixa por tipo", AVIFSample: "Saída AVIF", WebPSample: "Saída WebP", SampleLink: "Link de exemplo",
		CopyrightTitle: "Direitos autorais", CopyrightCopy: "O serviço fornece entrega, redimensionamento e conversão de imagens. Os direitos normalmente pertencem ao autor, plataforma ou titular; verifique permissões antes de usar.", PrivacyTitle: "Privacidade", PrivacyCopy: "Guardamos apenas dados técnicos básicos necessários para operação, cache, diagnóstico e análise.",
		FooterStatsAria: "Estatísticas", ImagesLabel: "imagens", StorageLabel: "armazenado", FooterInfo: "Info", UsageModalTitle: "Uso", CopyrightModalTitle: "Direitos", PrivacyModalTitle: "Privacidade", CloseModalAria: "Fechar", LanguageSwitcherLabel: "Idioma",
	},
	{
		Code: "es", HTMLLang: "es", OGLocale: "es_ES", Name: "Español",
		Title: "{{site}} - servicio dinámico de placeholders e imágenes aleatorias", Description: "{{name}} genera imágenes por tamaño, categoría, clave fija y formatos WebP/AVIF.", Keywords: "{{name}},imágenes aleatorias,placeholder,WebP,AVIF",
		HeaderAria: "Inicio", FeatureAria: "Funciones principales", FeatureSize: "Recorte por tamaño", FeatureType: "Categorías", FeatureFixed: "Parámetro r fijo", FeatureFormat: "WebP / AVIF", FeatureCDN: "CDN global",
		UsageTitle: "Cómo usar", UsageLead: "Añade un tamaño después de {{host}} y la URL devuelve una imagen. Usa type, r, s y format para controlar categoría, selección fija, slots y formato.",
		SummaryBase: "Ruta base", SummaryFixed: "Clave fija", SummaryType: "Categoría", SummarySlot: "Slot", SummaryFormat: "Formato", GeneratorTitle: "Generador de URL", GeneratorLead: "Introduce dimensiones y elige tipo, formato y modo. La página genera una URL lista para usar.",
		WidthLabel: "Ancho", HeightLabel: "Alto", TypeLabel: "Tipo", DefaultType: "Predeterminado", FormatLabel: "Formato", DefaultFormat: "webp (predeterminado)", ModeLabel: "Modo", RandomMode: "Aleatorio", FixedMode: "r fijo", SlotLabel: "Valor s", KeywordLabel: "Palabra clave",
		CopyAddress: "Copiar URL", OpenTest: "Abrir prueba", ExamplesTitle: "Ejemplos de código", HTMLExampleTitle: "HTML - imagen aleatoria", MarkdownExampleTitle: "Markdown - imagen de artículo", CSSExampleTitle: "CSS - fondo",
		WallpaperTitle: "Modo wallpaper (resolución original)", WallpaperLead: "Para wallpapers o imágenes originales de alta resolución, usa el enlace corto de categoría.", FormatSectionTitle: "Formatos", FormatLead: "El servidor entrega WebP y AVIF directamente desde el mismo dominio.",
		TypesTitle: "Tipos", TypesLead: "Usa el parámetro type para solicitar una categoría. Haz clic en una tarjeta para abrir un original.", SamplesTitle: "Imágenes de ejemplo", RandomSample: "Imagen aleatoria", TypedSample: "Aleatoria por tipo", FixedSample: "Imagen fija", FixedTypedSample: "Fija por tipo", AVIFSample: "Salida AVIF", WebPSample: "Salida WebP", SampleLink: "Enlace de ejemplo",
		CopyrightTitle: "Copyright", CopyrightCopy: "El servicio ofrece entrega, redimensionado y conversión de imágenes. Los derechos suelen pertenecer al autor, plataforma o titular; verifica permisos antes de usar.", PrivacyTitle: "Privacidad", PrivacyCopy: "Solo guardamos datos técnicos básicos necesarios para operación, caché, diagnóstico y análisis.",
		FooterStatsAria: "Estadísticas", ImagesLabel: "imágenes", StorageLabel: "almacenado", FooterInfo: "Info", UsageModalTitle: "Uso", CopyrightModalTitle: "Copyright", PrivacyModalTitle: "Privacidad", CloseModalAria: "Cerrar", LanguageSwitcherLabel: "Idioma",
	},
	{
		Code: "fr", HTMLLang: "fr", OGLocale: "fr_FR", Name: "Français",
		Title: "{{site}} - service dynamique de placeholders et d'images aléatoires", Description: "{{name}} génère des images par taille, catégorie, clé fixe et formats WebP/AVIF.", Keywords: "{{name}},images aléatoires,placeholder,WebP,AVIF",
		HeaderAria: "Accueil", FeatureAria: "Fonctions clés", FeatureSize: "Recadrage par taille", FeatureType: "Catégories", FeatureFixed: "Paramètre r fixe", FeatureFormat: "WebP / AVIF", FeatureCDN: "CDN mondial",
		UsageTitle: "Utilisation", UsageLead: "Ajoutez une taille après {{host}} et l'URL renvoie une image. Utilisez type, r, s et format pour contrôler catégorie, sélection fixe, emplacements et format.",
		SummaryBase: "Chemin de base", SummaryFixed: "Clé fixe", SummaryType: "Catégorie", SummarySlot: "Slot", SummaryFormat: "Format", GeneratorTitle: "Générateur d'URL", GeneratorLead: "Saisissez les dimensions et choisissez type, format et mode. La page crée une URL prête à l'emploi.",
		WidthLabel: "Largeur", HeightLabel: "Hauteur", TypeLabel: "Type", DefaultType: "Défaut", FormatLabel: "Format", DefaultFormat: "webp (défaut)", ModeLabel: "Mode", RandomMode: "Aléatoire", FixedMode: "r fixe", SlotLabel: "Valeur s", KeywordLabel: "Mot-clé",
		CopyAddress: "Copier l'URL", OpenTest: "Ouvrir le test", ExamplesTitle: "Exemples de code", HTMLExampleTitle: "HTML - image aléatoire", MarkdownExampleTitle: "Markdown - image d'article", CSSExampleTitle: "CSS - image de fond",
		WallpaperTitle: "Mode wallpaper (résolution originale)", WallpaperLead: "Pour des wallpapers ou des images originales haute résolution, utilisez le lien court de catégorie.", FormatSectionTitle: "Formats", FormatLead: "Le serveur fournit WebP et AVIF directement depuis le même domaine.",
		TypesTitle: "Types", TypesLead: "Utilisez le paramètre type pour demander une catégorie. Cliquez sur une carte pour ouvrir un original.", SamplesTitle: "Images d'exemple", RandomSample: "Image aléatoire", TypedSample: "Aléatoire par type", FixedSample: "Image fixe", FixedTypedSample: "Fixe par type", AVIFSample: "Sortie AVIF", WebPSample: "Sortie WebP", SampleLink: "Lien d'exemple",
		CopyrightTitle: "Copyright", CopyrightCopy: "Le service fournit livraison, redimensionnement et conversion d'images. Les droits appartiennent généralement à l'auteur, à la plateforme ou au titulaire; vérifiez les autorisations avant usage.", PrivacyTitle: "Confidentialité", PrivacyCopy: "Nous conservons seulement les données techniques nécessaires au fonctionnement, au cache, au diagnostic et à l'analyse.",
		FooterStatsAria: "Statistiques", ImagesLabel: "images", StorageLabel: "stockage", FooterInfo: "Info", UsageModalTitle: "Utilisation", CopyrightModalTitle: "Copyright", PrivacyModalTitle: "Confidentialité", CloseModalAria: "Fermer", LanguageSwitcherLabel: "Langue",
	},
	{
		Code: "ko", HTMLLang: "ko", OGLocale: "ko_KR", Name: "한국어",
		Title: "{{site}} - 동적 플레이스홀더 및 랜덤 이미지 서비스", Description: "{{name}} 는 크기, 카테고리, 고정 키, WebP/AVIF 형식으로 이미지를 생성합니다.", Keywords: "{{name}},랜덤 이미지,플레이스홀더,WebP,AVIF",
		HeaderAria: "홈", FeatureAria: "주요 기능", FeatureSize: "크기별 크롭", FeatureType: "카테고리 이미지", FeatureFixed: "고정 r 파라미터", FeatureFormat: "WebP / AVIF", FeatureCDN: "글로벌 CDN",
		UsageTitle: "사용 방법", UsageLead: "{{host}} 뒤에 크기를 붙이면 이미지 파일을 바로 반환합니다. type, r, s, format 으로 카테고리, 고정 선택, 슬롯, 출력 형식을 제어할 수 있습니다.",
		SummaryBase: "기본 경로", SummaryFixed: "고정 키", SummaryType: "카테고리", SummarySlot: "슬롯", SummaryFormat: "형식", GeneratorTitle: "URL 생성기", GeneratorLead: "크기와 타입, 형식, 모드를 선택하면 즉시 사용할 수 있는 이미지 URL을 만듭니다.",
		WidthLabel: "너비", HeightLabel: "높이", TypeLabel: "타입", DefaultType: "기본", FormatLabel: "형식", DefaultFormat: "webp (기본)", ModeLabel: "모드", RandomMode: "랜덤", FixedMode: "고정 r", SlotLabel: "s 값", KeywordLabel: "사용자 키워드",
		CopyAddress: "URL 복사", OpenTest: "테스트 열기", ExamplesTitle: "코드 예시", HTMLExampleTitle: "HTML - 랜덤 히어로 이미지", MarkdownExampleTitle: "Markdown - 글 이미지", CSSExampleTitle: "CSS - 배경 이미지",
		WallpaperTitle: "월페이퍼 모드 (원본 해상도)", WallpaperLead: "고해상도 원본 이미지가 필요하면 카테고리 짧은 링크를 사용하세요.", FormatSectionTitle: "형식", FormatLead: "서버는 같은 도메인에서 WebP와 AVIF를 직접 출력합니다.",
		TypesTitle: "타입", TypesLead: "type 파라미터로 카테고리 이미지를 요청할 수 있습니다. 카드를 클릭하면 원본 이미지를 엽니다.", SamplesTitle: "샘플 이미지", RandomSample: "랜덤 이미지", TypedSample: "타입 랜덤 이미지", FixedSample: "고정 이미지", FixedTypedSample: "고정 타입 이미지", AVIFSample: "AVIF 출력", WebPSample: "WebP 출력", SampleLink: "샘플 링크",
		CopyrightTitle: "저작권", CopyrightCopy: "이 서비스는 이미지 전달, 리사이즈, 형식 변환을 제공합니다. 이미지 권리는 보통 원작자, 출처 플랫폼 또는 권리자에게 있으므로 사용 전 권한을 확인하세요.", PrivacyTitle: "개인정보", PrivacyCopy: "서비스 운영, 캐시 최적화, 문제 해결, 분석에 필요한 기본 기술 정보만 저장합니다.",
		FooterStatsAria: "라이브러리 통계", ImagesLabel: "이미지", StorageLabel: "저장 용량", FooterInfo: "정보", UsageModalTitle: "사용법", CopyrightModalTitle: "저작권", PrivacyModalTitle: "개인정보", CloseModalAria: "닫기", LanguageSwitcherLabel: "언어",
	},
}

func localizedHomeFor(code string) (LocalizedHome, bool) {
	code = strings.ToLower(strings.TrimSpace(code))
	for _, l := range supportedLocalizedHomes {
		if l.Code == code {
			return l, true
		}
	}
	return LocalizedHome{}, false
}

func isLocalizedHomeCode(code string) bool {
	_, ok := localizedHomeFor(code)
	return ok
}

func languageLinks(current string, site SiteContext) []LanguageLink {
	current = strings.ToLower(strings.TrimSpace(current))
	links := make([]LanguageLink, 0, len(supportedLocalizedHomes))
	for _, l := range supportedLocalizedHomes {
		u := site.BaseURL + "/" + l.Code + "/"
		if l.Code == "zh" {
			u = site.BaseURL + "/"
		}
		links = append(links, LanguageLink{
			Code:    l.Code,
			Label:   strings.ToUpper(l.Code),
			Native:  l.Name,
			Flag:    languageFlagCode(l.Code),
			URL:     u,
			Current: l.Code == current,
		})
	}
	return links
}

func languageFlagCode(code string) string {
	switch code {
	case "zh":
		return "cn"
	case "en":
		return "us"
	case "ja":
		return "jp"
	case "ko":
		return "kr"
	default:
		return code
	}
}

func localizeText(s LocalizedHome, site SiteContext) LocalizedHome {
	repl := func(v string) string {
		v = strings.ReplaceAll(v, "{{site}}", site.FullName())
		v = strings.ReplaceAll(v, "{{name}}", site.Name)
		v = strings.ReplaceAll(v, "{{host}}", site.BaseHost())
		return v
	}
	s.Title = repl(s.Title)
	s.Description = repl(s.Description)
	s.Keywords = repl(s.Keywords)
	s.UsageLead = repl(s.UsageLead)
	return s
}
