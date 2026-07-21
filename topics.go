package feedrank

import (
	"regexp"
	"strings"
	"unicode"
)

// Ranking never consumes raw hashtags — only tags mapped onto the canonical
// taxonomy — so tag spam cannot game the topic term.

var (
	hashtagRe = regexp.MustCompile(`#([\p{L}\p{N}_]+)`)
	mentionRe = regexp.MustCompile(`@([\p{L}\p{N}_]+)`)
)

func extractTagged(re *regexp.Regexp, text string) []string {
	matches := re.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(matches))
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		tag := strings.ToLower(m[1])
		if tag == "" || seen[tag] {
			continue
		}
		seen[tag] = true
		out = append(out, tag)
	}
	return out
}

// ExtractHashtags returns lowercased hashtag bodies in order of first
// appearance, deduplicated. Supports CJK and other Unicode tags.
func ExtractHashtags(text string) []string {
	return extractTagged(hashtagRe, text)
}

// ExtractMentions returns lowercased @-handles in order of first appearance,
// deduplicated. Handle syntax is platform-specific; callers should
// re-validate against their own username rules before use.
func ExtractMentions(text string) []string {
	return extractTagged(mentionRe, text)
}

// MapTopics maps raw tags onto the canonical taxonomy via identity or alias;
// unmapped tags are dropped. Deduplicated, order-preserving, capped at max
// (0 = no cap).
func MapTopics(raw []string, canonical map[string]bool, aliases map[string]string, max int) []string {
	if len(raw) == 0 || len(canonical) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(raw))
	var out []string
	for _, tag := range raw {
		topic := tag
		if !canonical[topic] {
			topic = aliases[tag]
		}
		if topic == "" || !canonical[topic] || seen[topic] {
			continue
		}
		seen[topic] = true
		out = append(out, topic)
		if max > 0 && len(out) == max {
			break
		}
	}
	return out
}

// PostTopics maps a post's hashtags onto the taxonomy, falling back to the
// author's profile interests when no hashtag maps.
func PostTopics(text string, authorInterests []string, canonical map[string]bool, aliases map[string]string, max int) []string {
	if topics := MapTopics(ExtractHashtags(text), canonical, aliases, max); len(topics) > 0 {
		return topics
	}
	return MapTopics(authorInterests, canonical, aliases, max)
}

// MapQuery maps a free-text search query onto canonical topics: the query is
// lowercased, split on non-letter/digit runs, and each token is mapped like a
// hashtag. Unsegmented CJK compounds match only when a token equals an alias.
// Use it to turn per-user search history into SearchInterests and aggregate
// search logs into TrendingTopics.
func MapQuery(query string, canonical map[string]bool, aliases map[string]string, max int) []string {
	tokens := strings.FieldsFunc(strings.ToLower(query), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r) && r != '_'
	})
	return MapTopics(tokens, canonical, aliases, max)
}

// DefaultAliases maps common hashtags in ten language groups (en, ja, ko, zh,
// es, hi, vi, fr, de, pt) onto the canonical 30-tag taxonomy. Instances
// extend or replace it freely.
func DefaultAliases() map[string]string {
	out := make(map[string]string, 512)
	for _, m := range []map[string]string{
		aliasesEN, aliasesJA, aliasesKO, aliasesZH, aliasesES,
		aliasesHI, aliasesVI, aliasesFR, aliasesDE, aliasesPT,
	} {
		for k, v := range m {
			out[k] = v
		}
	}
	return out
}

var aliasesEN = map[string]string{
	"gadget": "tech", "technology": "tech",
	"coding": "programming", "code": "programming", "dev": "programming", "engineering": "programming",
	"machinelearning": "ai", "ml": "ai", "llm": "ai",
	"game": "gaming", "games": "gaming", "esports": "gaming",
	"ui": "design", "ux": "design",
	"photo": "photography", "camera": "photography",
	"song": "music", "band": "music",
	"movie": "film", "movies": "film", "cinema": "film",
	"drama": "tv", "anime": "tv", "series": "tv",
	"soccer": "sports", "football": "sports", "baseball": "sports", "basketball": "sports", "sport": "sports",
	"workout": "fitness", "gym": "fitness", "running": "fitness",
	"trip":   "travel",
	"recipe": "food", "cooking": "food",
	"ootd": "fashion", "style": "fashion",
	"makeup": "beauty", "skincare": "beauty",
	"illustration": "art", "drawing": "art",
	"novel": "literature", "book": "literature", "books": "literature", "reading": "literature",
	"blog": "writing", "poetry": "writing",
	"startup": "business", "marketing": "business",
	"investing": "finance", "stocks": "finance",
	"bitcoin": "crypto", "btc": "crypto", "eth": "crypto", "nft": "crypto", "web3": "crypto",
	"space": "science", "physics": "science", "astronomy": "science",
	"outdoor": "nature", "camping": "nature", "hiking": "nature",
	"dog": "pets", "dogs": "pets", "cat": "pets", "cats": "pets",
	"election": "politics",
	"study":    "education", "learning": "education",
	"daily": "lifestyle", "vlog": "lifestyle",
	"mentalhealth": "health", "wellness": "health",
	"funny": "comedy", "meme": "comedy", "humor": "comedy",
	"breaking": "news",
}

var aliasesJA = map[string]string{
	"テック": "tech", "テクノロジー": "tech", "ガジェット": "tech",
	"プログラミング": "programming", "コード": "programming", "開発": "programming",
	"人工知能": "ai", "生成ai": "ai",
	"ゲーム": "gaming", "eスポーツ": "gaming",
	"デザイン": "design",
	"写真":   "photography", "カメラ": "photography",
	"音楽": "music", "曲": "music", "バンド": "music",
	"映画":  "film",
	"テレビ": "tv", "ドラマ": "tv", "アニメ": "tv",
	"スポーツ": "sports", "サッカー": "sports", "野球": "sports", "バスケ": "sports",
	"筋トレ": "fitness", "フィットネス": "fitness", "ジム": "fitness", "ランニング": "fitness",
	"旅行": "travel", "旅": "travel", "観光": "travel",
	"グルメ": "food", "料理": "food", "ごはん": "food", "レシピ": "food", "ラーメン": "food", "カフェ": "food",
	"ファッション": "fashion", "コーデ": "fashion",
	"美容": "beauty", "コスメ": "beauty", "メイク": "beauty",
	"アート": "art", "絵": "art", "イラスト": "art",
	"文学": "literature", "小説": "literature", "読書": "literature",
	"執筆": "writing", "ライティング": "writing", "ブログ": "writing", "詩": "writing",
	"ビジネス": "business", "起業": "business", "スタートアップ": "business", "マーケティング": "business",
	"金融": "finance", "投資": "finance", "株": "finance", "経済": "finance",
	"仮想通貨": "crypto", "ビットコイン": "crypto",
	"科学": "science", "サイエンス": "science", "宇宙": "science",
	"自然": "nature", "アウトドア": "nature", "キャンプ": "nature", "登山": "nature",
	"ペット": "pets", "犬": "pets", "いぬ": "pets", "猫": "pets", "ねこ": "pets",
	"政治": "politics", "選挙": "politics",
	"教育": "education", "勉強": "education", "学習": "education",
	"ライフスタイル": "lifestyle", "日常": "lifestyle",
	"健康": "health", "メンタル": "health",
	"お笑い": "comedy", "ミーム": "comedy", "ネタ": "comedy",
	"ニュース": "news", "速報": "news",
}

var aliasesKO = map[string]string{
	"음악": "music", "게임": "gaming", "여행": "travel",
	"맛집": "food", "요리": "food", "먹방": "food",
	"패션": "fashion", "뷰티": "beauty", "화장": "beauty",
	"운동": "fitness", "헬스": "fitness",
	"축구": "sports", "야구": "sports", "농구": "sports",
	"영화": "film", "드라마": "tv",
	"사진": "photography", "그림": "art", "예술": "art",
	"책": "literature", "독서": "literature",
	"뉴스": "news", "정치": "politics",
	"경제": "finance", "주식": "finance", "투자": "finance",
	"과학": "science", "건강": "health",
	"반려동물": "pets", "강아지": "pets", "고양이": "pets",
	"코딩": "programming", "개발자": "programming", "인공지능": "ai",
	"공부": "education", "교육": "education",
	"일상": "lifestyle", "유머": "comedy",
	"자연": "nature", "캠핑": "nature",
	"암호화폐": "crypto", "기술": "tech",
}

var aliasesZH = map[string]string{
	"音乐": "music", "游戏": "gaming", "电竞": "gaming",
	"旅游": "travel", "美食": "food",
	"时尚": "fashion", "美妆": "beauty", "化妆": "beauty",
	"健身": "fitness",
	"足球": "sports", "篮球": "sports", "棒球": "sports",
	"电影": "film", "电视剧": "tv", "综艺": "tv",
	"摄影": "photography", "艺术": "art", "绘画": "art",
	"小说": "literature", "读书": "literature", "阅读": "literature",
	"新闻": "news", "时事": "news",
	"经济": "finance", "股票": "finance", "理财": "finance",
	"宠物": "pets", "狗狗": "pets", "猫咪": "pets",
	"编程": "programming", "程序员": "programming", "人工智能": "ai",
	"学习": "education", "搞笑": "comedy", "幽默": "comedy",
	"加密货币": "crypto", "区块链": "crypto", "科技": "tech",
}

var aliasesES = map[string]string{
	"musica": "music", "música": "music",
	"juegos": "gaming", "videojuegos": "gaming",
	"viaje": "travel", "viajes": "travel", "viajar": "travel",
	"comida": "food", "receta": "food", "recetas": "food", "cocina": "food",
	"gastronomia": "food", "gastronomía": "food",
	"moda": "fashion", "belleza": "beauty", "maquillaje": "beauty",
	"deporte": "sports", "deportes": "sports", "futbol": "sports", "fútbol": "sports",
	"ejercicio": "fitness", "gimnasio": "fitness",
	"cine": "film", "pelicula": "film", "película": "film", "peliculas": "film", "películas": "film",
	"fotografia": "photography", "fotografía": "photography",
	"arte":  "art",
	"libro": "literature", "libros": "literature", "lectura": "literature",
	"noticias": "news",
	"politica": "politics", "política": "politics",
	"economia": "finance", "economía": "finance", "finanzas": "finance",
	"ciencia": "science", "salud": "health",
	"mascota": "pets", "mascotas": "pets", "perro": "pets", "perros": "pets", "gato": "pets", "gatos": "pets",
	"programacion": "programming", "programación": "programming",
	"tecnologia": "tech", "tecnología": "tech",
	"educacion": "education", "educación": "education",
	"naturaleza": "nature", "negocios": "business", "criptomonedas": "crypto",
}

var aliasesHI = map[string]string{
	"संगीत": "music", "खेल": "sports", "क्रिकेट": "sports",
	"यात्रा": "travel", "खाना": "food", "व्यंजन": "food",
	"फैशन": "fashion", "सुंदरता": "beauty",
	"फिल्म": "film", "बॉलीवुड": "film",
	"फोटोग्राफी": "photography", "कला": "art",
	"किताब": "literature", "पुस्तक": "literature",
	"समाचार": "news", "राजनीति": "politics",
	"विज्ञान": "science", "स्वास्थ्य": "health", "योग": "fitness",
	"शिक्षा":       "education",
	"प्रौद्योगिकी": "tech", "तकनीक": "tech",
	"क्रिप्टो": "crypto", "प्रकृति": "nature", "हास्य": "comedy",
	"वित्त": "finance", "निवेश": "finance",
	"पालतू": "pets", "कुत्ता": "pets", "बिल्ली": "pets",
	"प्रोग्रामिंग": "programming", "गेमिंग": "gaming", "व्यापार": "business",
}

var aliasesVI = map[string]string{
	"amnhac": "music", "nhac": "music",
	"dulich":  "travel",
	"monngon": "food", "amthuc": "food", "anngon": "food",
	"thoitrang": "fashion", "lamdep": "beauty",
	"bongda": "sports", "thethao": "sports",
	"phim": "film", "nhiepanh": "photography", "nghethuat": "art",
	"sach": "literature", "docsach": "literature",
	"tintuc": "news", "chinhtri": "politics",
	"khoahoc": "science", "suckhoe": "health",
	"giaoduc": "education", "congnghe": "tech",
	"thucung": "pets",
	"dautu":   "finance", "taichinh": "finance",
	"thiennhien": "nature", "haihuoc": "comedy",
	"laptrinh": "programming", "tienao": "crypto", "tienso": "crypto",
	"khoinghiep": "business",
}

var aliasesFR = map[string]string{
	"musique": "music",
	"jeux":    "gaming", "jeuxvideo": "gaming",
	"voyage": "travel", "voyages": "travel",
	"cuisine": "food", "recette": "food", "recettes": "food",
	"mode":   "fashion",
	"beaute": "beauty", "beauté": "beauty", "maquillage": "beauty",
	"foot":   "sports",
	"cinéma": "film", "série": "tv", "séries": "tv",
	"photographie": "photography",
	"littérature":  "literature", "litterature": "literature", "livre": "literature", "livres": "literature",
	"actualites": "news", "actualités": "news",
	"politique": "politics",
	"économie":  "finance", "economie": "finance",
	"santé": "health", "sante": "health",
	"animaux": "pets", "chien": "pets", "chat": "pets",
	"programmation": "programming", "technologie": "tech",
	"éducation": "education", "humour": "comedy", "randonnée": "nature",
}

var aliasesDE = map[string]string{
	"musik":  "music",
	"spiele": "gaming", "videospiele": "gaming",
	"reisen": "travel",
	"essen":  "food", "kochen": "food", "rezept": "food", "rezepte": "food",
	"fussball": "sports", "fußball": "sports",
	"kino": "film", "filme": "film", "serien": "tv",
	"fotografie": "photography", "kunst": "art",
	"buch": "literature", "bücher": "literature", "lesen": "literature",
	"nachrichten": "news", "politik": "politics",
	"wirtschaft": "finance", "börse": "finance", "finanzen": "finance",
	"wissenschaft": "science", "gesundheit": "health",
	"haustiere": "pets", "hund": "pets", "katze": "pets",
	"programmieren": "programming", "technik": "tech",
	"bildung": "education", "natur": "nature", "krypto": "crypto",
}

var aliasesPT = map[string]string{
	"futebol": "sports", "esporte": "sports", "esportes": "sports",
	"viagem": "travel", "viagens": "travel",
	"culinaria": "food", "culinária": "food", "receita": "food", "receitas": "food",
	"beleza": "beauty",
	"filmes": "film", "foto": "photography",
	"livro": "literature", "livros": "literature", "leitura": "literature",
	"notícias": "news",
	"saude":    "health", "saúde": "health",
	"animais": "pets", "cachorro": "pets",
	"programacao": "programming", "programação": "programming",
	"educacao": "education", "educação": "education",
	"natureza": "nature",
	"financas": "finance", "finanças": "finance",
	"negocio": "business", "negócios": "business",
}
