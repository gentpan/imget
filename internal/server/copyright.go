package server

import (
	"strconv"
	"strings"
)

type CopyrightScenario struct {
	Use    string
	Advice string
}

type CopyrightDetails struct {
	PageLabel      string
	ImageLabel     string
	PageCopy       string
	ImageCopy      string
	ScenarioHeader string
	AdviceHeader   string
	Scenarios      []CopyrightScenario
	Warning        string
	ModalIntro     string
	ModalBullets   []string
}

type copyrightText struct {
	PageLabel      string
	ImageLabel     string
	PageCopy       string
	ImageCopy      string
	ScenarioHeader string
	AdviceHeader   string
	Scenarios      []CopyrightScenario
	Warning        string
	EmailNotice    string
	ModalIntro     string
	ModalBullets   []string
}

var copyrightTexts = map[string]copyrightText{
	"zh": {
		PageLabel: "页面文案版权", ImageLabel: "图片版权",
		PageCopy:       "{{site}} 页面中的文字说明、界面设计与站点说明内容，版权归 {{site}} 所有，年份为 {{year}}。",
		ImageCopy:      "本服务提供图片调用、尺寸生成与格式转换能力。通过 {{name}} 访问到的图片，并不归本站所有；图片权利通常归原作者、原始上传者、来源平台或合法权利人所有。",
		ScenarioHeader: "使用场景", AdviceHeader: "建议",
		Scenarios: []CopyrightScenario{
			{"普通文章配图、站点横幅", "建议保留图片来源记录，并确认来源平台允许该类展示用途。"},
			{"商业页面、客户项目、广告页面", "请单独核对商业授权、人物肖像、品牌标识与广告投放限制。"},
			{"人物、动漫、插画类图片", "请重点确认人物权利、作品授权、二次创作规则及商业适用范围。"},
			{"原图下载与再次分发", "请先核对来源平台条款；除非来源明确允许，否则不应视为可自由再分发素材。"},
		},
		Warning:     "使用本服务即表示你理解并同意：图片相关权利不因经过本服务处理而发生转移；若使用行为涉及版权、肖像、商标、隐私、平台规范或当地法律问题，应以原始来源条款和适用法律为准。",
		EmailNotice: "版权处理请发送邮件至 {{email}}，并附上页面地址、相关说明与权利证明。",
		ModalIntro:  "通过 {{name}} 访问到的图片，并不归本站所有。本站提供的是图片调用、尺寸处理、格式转换与访问分发能力，不出售图片版权，也不提供版权转让。",
		ModalBullets: []string{
			"图片的著作权、肖像权、商标权及其他相关权利，通常归原作者、原始上传者、来源平台或其合法权利人所有。",
			"用户在下载、展示、转载、商用或再次分发图片前，应自行确认图片来源、授权范围与适用条件。",
			"本站当前不以商业图库或版权素材销售为定位，主要用于工具服务与非商业内容辅助场景。",
			"如权利人认为相关内容存在侵权、误用或不当展示情况，可通过页脚邮箱提交说明与权利证明，我们会尽快处理。",
		},
	},
	"en": {
		PageLabel: "Page copy copyright", ImageLabel: "Image copyright",
		PageCopy:       "The written copy, interface design, and site documentation on {{site}} are copyrighted by {{site}} for {{year}}.",
		ImageCopy:      "This service provides image delivery, resizing, and format conversion. Images accessed through {{name}} are not owned by this site; rights usually belong to the original author, uploader, source platform, or lawful rights holder.",
		ScenarioHeader: "Use case", AdviceHeader: "Recommendation",
		Scenarios: []CopyrightScenario{
			{"Article images and site banners", "Keep source records and confirm that the source platform permits this kind of display."},
			{"Commercial pages, client work, and advertising", "Review commercial licensing, model rights, trademarks, and advertising restrictions separately."},
			{"People, anime, and illustration images", "Pay close attention to personality rights, work licenses, derivative-work rules, and commercial permissions."},
			{"Original downloads and redistribution", "Check the source platform terms first; do not treat images as freely redistributable unless the source explicitly allows it."},
		},
		Warning:     "By using this service, you understand and agree that image rights do not transfer because an image is processed by this service. If your use involves copyright, likeness, trademark, privacy, platform rules, or local law, the original source terms and applicable law control.",
		EmailNotice: "For copyright handling, email {{email}} with the page URL, explanation, and proof of rights.",
		ModalIntro:  "Images accessed through {{name}} are not owned by this site. The service provides image calls, resizing, format conversion, and delivery; it does not sell image copyrights or transfer rights.",
		ModalBullets: []string{
			"Copyright, likeness, trademark, and related rights usually belong to the original author, uploader, source platform, or lawful rights holder.",
			"Before downloading, displaying, reposting, using commercially, or redistributing images, users should verify the source, license scope, and applicable conditions.",
			"This site is not positioned as a commercial stock-photo or licensed-assets marketplace; it is mainly a tool and non-commercial content aid.",
			"Rights holders can submit an explanation and proof of rights through the footer email if content appears infringing, misused, or improperly displayed.",
		},
	},
	"ja": {
		PageLabel: "ページ文言の著作権", ImageLabel: "画像の著作権",
		PageCopy:       "{{site}} の説明文、UI デザイン、サイト説明コンテンツの著作権は {{year}} 年時点で {{site}} に帰属します。",
		ImageCopy:      "本サービスは画像配信、リサイズ、形式変換を提供します。{{name}} 経由で表示される画像は本站の所有物ではなく、通常は原作者、アップロード者、提供元プラットフォーム、または正当な権利者に権利があります。",
		ScenarioHeader: "利用場面", AdviceHeader: "推奨事項",
		Scenarios: []CopyrightScenario{
			{"記事画像、サイトバナー", "画像の出典記録を残し、その用途での表示が提供元により許可されているか確認してください。"},
			{"商用ページ、クライアント案件、広告", "商用ライセンス、肖像権、商標、広告利用制限を個別に確認してください。"},
			{"人物、アニメ、イラスト画像", "人物権利、作品ライセンス、二次創作ルール、商用利用可否を重点的に確認してください。"},
			{"原画像のダウンロードと再配布", "提供元の規約を先に確認してください。明示的に許可されない限り、自由に再配布できる素材とは見なさないでください。"},
		},
		Warning:     "本サービスを利用することにより、画像関連の権利は本サービスで処理されても移転しないことを理解し同意したものとします。著作権、肖像、商標、プライバシー、プラットフォーム規約、または現地法に関わる利用では、元の出典条件と適用法が優先されます。",
		EmailNotice: "著作権対応は {{email}} へ、ページ URL、説明、権利証明を添えて連絡してください。",
		ModalIntro:  "{{name}} 経由で表示される画像は本站の所有物ではありません。本サービスは画像呼び出し、サイズ処理、形式変換、配信を提供するもので、画像著作権の販売や譲渡は行いません。",
		ModalBullets: []string{
			"画像の著作権、肖像権、商標権、その他関連権利は通常、原作者、アップロード者、提供元、または正当な権利者に帰属します。",
			"ダウンロード、表示、転載、商用利用、再配布の前に、出典、許諾範囲、適用条件を確認してください。",
			"本站は商用素材販売やライセンス素材市場ではなく、主にツールサービスと非商用コンテンツ補助を目的としています。",
			"権利者が侵害、誤用、不適切表示を認識した場合、フッターのメールへ説明と権利証明を送付してください。",
		},
	},
	"de": {
		PageLabel: "Copyright der Seitentexte", ImageLabel: "Bildrechte",
		PageCopy:       "Texte, Interface-Design und Seitenerläuterungen auf {{site}} sind im Jahr {{year}} urheberrechtlich {{site}} zugeordnet.",
		ImageCopy:      "Dieser Dienst stellt Bildauslieferung, Größenanpassung und Formatkonvertierung bereit. Über {{name}} abgerufene Bilder gehören nicht dieser Website; Rechte liegen in der Regel bei Urhebern, Uploadenden, Quellplattformen oder rechtmäßigen Rechteinhabern.",
		ScenarioHeader: "Nutzungsszenario", AdviceHeader: "Empfehlung",
		Scenarios: []CopyrightScenario{
			{"Artikelbilder und Website-Banner", "Quellnachweise aufbewahren und prüfen, ob die Plattform diese Darstellung erlaubt."},
			{"Kommerzielle Seiten, Kundenprojekte und Werbung", "Kommerzielle Lizenzen, Persönlichkeitsrechte, Marken und Werbebeschränkungen separat prüfen."},
			{"Personen-, Anime- und Illustrationsbilder", "Persönlichkeitsrechte, Werklizenzen, Regeln für Bearbeitungen und kommerzielle Nutzung besonders prüfen."},
			{"Originaldownload und Weiterverbreitung", "Zuerst die Bedingungen der Quelle prüfen; ohne ausdrückliche Erlaubnis nicht als frei weiterverteilbares Material behandeln."},
		},
		Warning:     "Mit der Nutzung dieses Dienstes verstehen und akzeptieren Sie, dass Bildrechte durch Verarbeitung über diesen Dienst nicht übertragen werden. Bei Urheberrecht, Abbildungen, Marken, Datenschutz, Plattformregeln oder lokalem Recht gelten die ursprünglichen Quellbedingungen und anwendbares Recht.",
		EmailNotice: "Für Copyright-Anliegen senden Sie eine E-Mail an {{email}} mit Seiten-URL, Erläuterung und Rechtenachweis.",
		ModalIntro:  "Über {{name}} abgerufene Bilder gehören nicht dieser Website. Der Dienst bietet Bildabruf, Größenverarbeitung, Formatkonvertierung und Auslieferung, verkauft aber keine Bildrechte und überträgt keine Rechte.",
		ModalBullets: []string{
			"Urheberrechte, Persönlichkeitsrechte, Markenrechte und sonstige verwandte Rechte liegen in der Regel bei Urhebern, Uploadenden, Quellplattformen oder Rechteinhabern.",
			"Vor Download, Anzeige, Repost, kommerzieller Nutzung oder Weiterverbreitung sollten Quelle, Lizenzumfang und Bedingungen geprüft werden.",
			"Diese Website ist keine kommerzielle Bilddatenbank oder Lizenzplattform, sondern vor allem ein Tool und Hilfsmittel für nichtkommerzielle Inhalte.",
			"Rechteinhaber können über die Footer-E-Mail eine Erläuterung und einen Rechtenachweis einreichen, wenn Inhalte rechtsverletzend, missbräuchlich oder unangemessen angezeigt werden.",
		},
	},
	"ru": {
		PageLabel: "Авторские права на текст страницы", ImageLabel: "Права на изображения",
		PageCopy:       "Тексты, дизайн интерфейса и описания сайта {{site}} защищены авторским правом {{site}} за {{year}} год.",
		ImageCopy:      "Сервис предоставляет доставку изображений, изменение размера и конвертацию формата. Изображения, доступные через {{name}}, не принадлежат этому сайту; права обычно принадлежат автору, загрузившему пользователю, исходной платформе или законному правообладателю.",
		ScenarioHeader: "Сценарий использования", AdviceHeader: "Рекомендация",
		Scenarios: []CopyrightScenario{
			{"Иллюстрации для статей и баннеры сайта", "Сохраняйте сведения об источнике и проверяйте, разрешает ли платформа такое отображение."},
			{"Коммерческие страницы, клиентские проекты и реклама", "Отдельно проверьте коммерческую лицензию, права на изображение людей, товарные знаки и рекламные ограничения."},
			{"Изображения людей, аниме и иллюстрации", "Особенно проверяйте права личности, лицензии произведений, правила производных работ и коммерческого использования."},
			{"Скачивание оригиналов и повторное распространение", "Сначала проверьте условия источника; не считайте материал свободным для распространения без явного разрешения."},
		},
		Warning:     "Используя сервис, вы понимаете и соглашаетесь, что права на изображения не переходят из-за обработки этим сервисом. Если использование затрагивает авторские права, изображение личности, товарные знаки, приватность, правила платформы или местное право, применяются условия исходного источника и действующее законодательство.",
		EmailNotice: "Для вопросов авторских прав отправьте письмо на {{email}} с URL страницы, пояснением и подтверждением прав.",
		ModalIntro:  "Изображения, доступные через {{name}}, не принадлежат этому сайту. Сервис предоставляет вызов изображений, обработку размеров, конвертацию форматов и доставку; он не продает авторские права и не передает права.",
		ModalBullets: []string{
			"Авторские права, права на изображение личности, товарные знаки и другие связанные права обычно принадлежат автору, загрузившему пользователю, исходной платформе или правообладателю.",
			"Перед скачиванием, показом, публикацией, коммерческим использованием или повторным распространением проверяйте источник, объем лицензии и условия.",
			"Сайт не является коммерческой фотобиблиотекой или рынком лицензированных материалов; он в основном служит инструментом и помощником для некоммерческого контента.",
			"Правообладатели могут отправить объяснение и подтверждение прав через email в футере, если контент нарушает права, используется ошибочно или отображается ненадлежащим образом.",
		},
	},
	"pt": {
		PageLabel: "Direitos do texto da página", ImageLabel: "Direitos das imagens",
		PageCopy:       "Os textos, o design da interface e as explicações do site {{site}} são protegidos por direitos autorais de {{site}} em {{year}}.",
		ImageCopy:      "Este serviço fornece entrega, redimensionamento e conversão de imagens. As imagens acessadas por {{name}} não pertencem a este site; os direitos normalmente pertencem ao autor, remetente, plataforma de origem ou titular legítimo.",
		ScenarioHeader: "Caso de uso", AdviceHeader: "Recomendação",
		Scenarios: []CopyrightScenario{
			{"Imagens de artigos e banners de site", "Guarde registros da fonte e confirme que a plataforma permite esse tipo de exibição."},
			{"Páginas comerciais, projetos de clientes e anúncios", "Revise separadamente licença comercial, direitos de imagem, marcas e restrições de publicidade."},
			{"Imagens de pessoas, anime e ilustrações", "Verifique direitos de personalidade, licenças da obra, regras de obra derivada e permissão comercial."},
			{"Download de originais e redistribuição", "Confira primeiro os termos da fonte; não trate como material livre para redistribuição sem permissão explícita."},
		},
		Warning:     "Ao usar este serviço, você entende e concorda que direitos de imagem não são transferidos por processamento neste serviço. Se o uso envolver direitos autorais, imagem pessoal, marcas, privacidade, regras de plataforma ou leis locais, valem os termos da fonte original e a lei aplicável.",
		EmailNotice: "Para questões de direitos autorais, envie email para {{email}} com URL da página, explicação e prova de direitos.",
		ModalIntro:  "As imagens acessadas por {{name}} não pertencem a este site. O serviço fornece chamada de imagem, processamento de tamanho, conversão de formato e entrega; não vende direitos autorais nem transfere direitos.",
		ModalBullets: []string{
			"Direitos autorais, direitos de imagem, marcas e outros direitos relacionados normalmente pertencem ao autor, remetente, plataforma de origem ou titular legítimo.",
			"Antes de baixar, exibir, republicar, usar comercialmente ou redistribuir imagens, verifique a fonte, o escopo da licença e as condições aplicáveis.",
			"Este site não é um banco comercial de fotos nem um mercado de ativos licenciados; ele é principalmente uma ferramenta e apoio para conteúdo não comercial.",
			"Titulares de direitos podem enviar explicação e prova de direitos pelo email do rodapé se o conteúdo parecer infrator, mal utilizado ou exibido indevidamente.",
		},
	},
	"es": {
		PageLabel: "Copyright del texto de la página", ImageLabel: "Derechos de imagen",
		PageCopy:       "Los textos, el diseño de la interfaz y las explicaciones del sitio {{site}} están protegidos por copyright de {{site}} en {{year}}.",
		ImageCopy:      "Este servicio ofrece entrega, redimensionado y conversión de imágenes. Las imágenes accesibles mediante {{name}} no pertenecen a este sitio; los derechos suelen pertenecer al autor, usuario que subió la imagen, plataforma de origen o titular legítimo.",
		ScenarioHeader: "Caso de uso", AdviceHeader: "Recomendación",
		Scenarios: []CopyrightScenario{
			{"Imágenes de artículos y banners del sitio", "Conserva registros de la fuente y confirma que la plataforma permite este tipo de visualización."},
			{"Páginas comerciales, proyectos de clientes y publicidad", "Revisa por separado licencia comercial, derechos de imagen, marcas y restricciones publicitarias."},
			{"Imágenes de personas, anime e ilustraciones", "Verifica derechos de personalidad, licencias de la obra, reglas de obras derivadas y permisos comerciales."},
			{"Descarga de originales y redistribución", "Consulta primero los términos de la fuente; no lo trates como material libremente redistribuible salvo permiso explícito."},
		},
		Warning:     "Al usar este servicio, entiendes y aceptas que los derechos de imagen no se transfieren por ser procesados por este servicio. Si el uso implica copyright, imagen personal, marcas, privacidad, normas de plataforma o leyes locales, rigen los términos de la fuente original y la ley aplicable.",
		EmailNotice: "Para asuntos de copyright, envía un email a {{email}} con la URL de la página, explicación y prueba de derechos.",
		ModalIntro:  "Las imágenes accesibles mediante {{name}} no pertenecen a este sitio. El servicio ofrece llamadas de imagen, procesamiento de tamaño, conversión de formato y entrega; no vende copyright ni transfiere derechos.",
		ModalBullets: []string{
			"El copyright, derechos de imagen, marcas y otros derechos relacionados suelen pertenecer al autor, usuario que subió la imagen, plataforma de origen o titular legítimo.",
			"Antes de descargar, mostrar, republicar, usar comercialmente o redistribuir imágenes, verifica la fuente, el alcance de la licencia y las condiciones aplicables.",
			"Este sitio no es una biblioteca comercial de fotos ni un mercado de activos con licencia; es principalmente una herramienta y apoyo para contenido no comercial.",
			"Los titulares de derechos pueden enviar una explicación y prueba de derechos mediante el email del pie de página si el contenido parece infringir derechos, usarse indebidamente o mostrarse de forma incorrecta.",
		},
	},
	"fr": {
		PageLabel: "Droits du texte de la page", ImageLabel: "Droits des images",
		PageCopy:       "Les textes, la conception d'interface et les explications du site {{site}} sont protégés par les droits de {{site}} pour {{year}}.",
		ImageCopy:      "Ce service fournit livraison, redimensionnement et conversion d'images. Les images accessibles via {{name}} n'appartiennent pas à ce site; les droits appartiennent généralement à l'auteur, à l'utilisateur ayant téléversé l'image, à la plateforme source ou au titulaire légitime.",
		ScenarioHeader: "Cas d'utilisation", AdviceHeader: "Recommandation",
		Scenarios: []CopyrightScenario{
			{"Images d'articles et bannières de site", "Conservez les informations de source et vérifiez que la plateforme autorise ce type d'affichage."},
			{"Pages commerciales, projets clients et publicité", "Vérifiez séparément licence commerciale, droit à l'image, marques et restrictions publicitaires."},
			{"Images de personnes, anime et illustrations", "Vérifiez les droits de la personne, les licences d'œuvre, les règles d'œuvres dérivées et les autorisations commerciales."},
			{"Téléchargement d'originaux et redistribution", "Consultez d'abord les conditions de la source; ne considérez pas l'image comme librement redistribuable sans autorisation explicite."},
		},
		Warning:     "En utilisant ce service, vous comprenez et acceptez que les droits d'image ne sont pas transférés par le traitement via ce service. Si l'usage concerne copyright, image de personne, marques, confidentialité, règles de plateforme ou droit local, les conditions de la source d'origine et la loi applicable prévalent.",
		EmailNotice: "Pour les demandes de copyright, envoyez un email à {{email}} avec l'URL de la page, une explication et une preuve de droits.",
		ModalIntro:  "Les images accessibles via {{name}} n'appartiennent pas à ce site. Le service fournit appel d'image, traitement de taille, conversion de format et livraison; il ne vend pas de droits d'image et ne transfère pas de droits.",
		ModalBullets: []string{
			"Copyright, droit à l'image, marques et autres droits liés appartiennent généralement à l'auteur, à l'utilisateur ayant téléversé l'image, à la plateforme source ou au titulaire légitime.",
			"Avant téléchargement, affichage, republication, usage commercial ou redistribution, vérifiez la source, le périmètre de licence et les conditions applicables.",
			"Ce site n'est pas une banque commerciale de photos ni une place de marché d'actifs sous licence; il sert principalement d'outil et d'aide au contenu non commercial.",
			"Les titulaires de droits peuvent envoyer une explication et une preuve via l'email du pied de page si un contenu semble contrefaisant, mal utilisé ou affiché de manière inappropriée.",
		},
	},
	"ko": {
		PageLabel: "페이지 문구 저작권", ImageLabel: "이미지 저작권",
		PageCopy:       "{{site}} 의 설명 문구, 인터페이스 디자인, 사이트 안내 콘텐츠의 저작권은 {{year}}년 기준 {{site}} 에 있습니다.",
		ImageCopy:      "이 서비스는 이미지 호출, 크기 조정, 형식 변환 기능을 제공합니다. {{name}} 를 통해 접근하는 이미지는 이 사이트의 소유가 아니며, 권리는 보통 원작자, 업로드한 사용자, 출처 플랫폼 또는 정당한 권리자에게 있습니다.",
		ScenarioHeader: "사용 상황", AdviceHeader: "권장 사항",
		Scenarios: []CopyrightScenario{
			{"일반 글 이미지와 사이트 배너", "이미지 출처 기록을 보관하고 출처 플랫폼이 해당 표시 용도를 허용하는지 확인하세요."},
			{"상업 페이지, 고객 프로젝트, 광고", "상업 라이선스, 초상권, 상표, 광고 제한을 별도로 확인하세요."},
			{"인물, 애니메이션, 일러스트 이미지", "인물 권리, 작품 라이선스, 2차 창작 규칙, 상업적 사용 가능 범위를 특히 확인하세요."},
			{"원본 다운로드와 재배포", "먼저 출처 플랫폼 약관을 확인하세요. 명시적 허가가 없으면 자유롭게 재배포 가능한 소재로 보지 마세요."},
		},
		Warning:     "이 서비스를 사용하면 이미지 관련 권리가 이 서비스의 처리로 이전되지 않는다는 점을 이해하고 동의한 것으로 봅니다. 사용이 저작권, 초상, 상표, 개인정보, 플랫폼 규정 또는 현지 법률과 관련되면 원본 출처 조건과 적용 법률이 우선합니다.",
		EmailNotice: "저작권 처리는 {{email}} 로 페이지 URL, 설명, 권리 증빙을 보내 주세요.",
		ModalIntro:  "{{name}} 를 통해 접근하는 이미지는 이 사이트의 소유가 아닙니다. 이 서비스는 이미지 호출, 크기 처리, 형식 변환, 전송 기능을 제공하며 이미지 저작권을 판매하거나 권리를 이전하지 않습니다.",
		ModalBullets: []string{
			"이미지의 저작권, 초상권, 상표권 및 기타 관련 권리는 보통 원작자, 업로드한 사용자, 출처 플랫폼 또는 정당한 권리자에게 있습니다.",
			"이미지를 다운로드, 표시, 재게시, 상업적 사용 또는 재배포하기 전에 출처, 라이선스 범위, 적용 조건을 확인해야 합니다.",
			"이 사이트는 상업용 이미지 라이브러리나 라이선스 자산 판매처가 아니며, 주로 도구 서비스와 비상업 콘텐츠 보조 용도입니다.",
			"권리자는 콘텐츠가 침해, 오용 또는 부적절하게 표시되었다고 판단되면 푸터 이메일로 설명과 권리 증빙을 제출할 수 있습니다.",
		},
	},
}

func localizedCopyrightDetails(code string, site SiteContext) CopyrightDetails {
	code = strings.ToLower(strings.TrimSpace(code))
	text, ok := copyrightTexts[code]
	if !ok {
		text = copyrightTexts["en"]
	}
	repl := func(v string) string {
		v = strings.ReplaceAll(v, "{{site}}", site.FullName())
		v = strings.ReplaceAll(v, "{{name}}", site.Name)
		v = strings.ReplaceAll(v, "{{year}}", strconv.Itoa(site.CopyrightYear))
		v = strings.ReplaceAll(v, "{{email}}", site.DMCAEmail)
		return v
	}
	warning := repl(text.Warning)
	if site.HasDMCAEmail() && text.EmailNotice != "" {
		warning += " " + repl(text.EmailNotice)
	}
	scenarios := make([]CopyrightScenario, 0, len(text.Scenarios))
	for _, s := range text.Scenarios {
		scenarios = append(scenarios, CopyrightScenario{Use: repl(s.Use), Advice: repl(s.Advice)})
	}
	bullets := make([]string, 0, len(text.ModalBullets))
	for _, b := range text.ModalBullets {
		bullets = append(bullets, repl(b))
	}
	return CopyrightDetails{
		PageLabel:      repl(text.PageLabel),
		ImageLabel:     repl(text.ImageLabel),
		PageCopy:       repl(text.PageCopy),
		ImageCopy:      repl(text.ImageCopy),
		ScenarioHeader: repl(text.ScenarioHeader),
		AdviceHeader:   repl(text.AdviceHeader),
		Scenarios:      scenarios,
		Warning:        warning,
		ModalIntro:     repl(text.ModalIntro),
		ModalBullets:   bullets,
	}
}
