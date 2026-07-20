package feedrank

import (
	"regexp"
	"strings"
)

// Ranking never consumes raw hashtags — only tags mapped onto the canonical
// taxonomy — so tag spam cannot game the topic term.

var hashtagRe = regexp.MustCompile(`#([\p{L}\p{N}_]+)`)

// ExtractHashtags returns lowercased hashtag bodies in order of first
// appearance, deduplicated. Supports CJK tags.
func ExtractHashtags(text string) []string {
	matches := hashtagRe.FindAllStringSubmatch(text, -1)
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

// DefaultAliases maps common English and Japanese hashtags onto the
// canonical 30-tag taxonomy. Instances extend or replace it freely.
func DefaultAliases() map[string]string {
	return map[string]string{
		// tech / programming / ai
		"テック": "tech", "テクノロジー": "tech", "gadget": "tech", "ガジェット": "tech",
		"プログラミング": "programming", "コード": "programming", "coding": "programming",
		"code": "programming", "開発": "programming", "dev": "programming", "engineering": "programming",
		"人工知能": "ai", "生成ai": "ai", "machinelearning": "ai", "ml": "ai", "llm": "ai",
		// gaming / design / photography
		"ゲーム": "gaming", "game": "gaming", "games": "gaming", "esports": "gaming", "eスポーツ": "gaming",
		"デザイン": "design", "ui": "design", "ux": "design",
		"写真": "photography", "photo": "photography", "camera": "photography", "カメラ": "photography",
		// music / film / tv
		"音楽": "music", "曲": "music", "song": "music", "バンド": "music", "band": "music",
		"映画": "film", "movie": "film", "movies": "film", "cinema": "film",
		"テレビ": "tv", "ドラマ": "tv", "drama": "tv", "アニメ": "tv", "anime": "tv",
		// sports / fitness
		"スポーツ": "sports", "サッカー": "sports", "soccer": "sports", "football": "sports",
		"野球": "sports", "baseball": "sports", "バスケ": "sports", "basketball": "sports",
		"筋トレ": "fitness", "フィットネス": "fitness", "workout": "fitness", "gym": "fitness",
		"ジム": "fitness", "ランニング": "fitness", "running": "fitness",
		// travel / food / fashion / beauty
		"旅行": "travel", "旅": "travel", "trip": "travel", "観光": "travel",
		"グルメ": "food", "料理": "food", "ごはん": "food", "レシピ": "food",
		"recipe": "food", "cooking": "food", "ラーメン": "food", "カフェ": "food",
		"ファッション": "fashion", "コーデ": "fashion", "ootd": "fashion", "style": "fashion",
		"美容": "beauty", "コスメ": "beauty", "メイク": "beauty", "makeup": "beauty", "skincare": "beauty",
		// art / literature / writing
		"アート": "art", "絵": "art", "イラスト": "art", "illustration": "art", "drawing": "art",
		"文学": "literature", "小説": "literature", "novel": "literature", "book": "literature",
		"books": "literature", "読書": "literature", "reading": "literature",
		"執筆": "writing", "ライティング": "writing", "ブログ": "writing", "blog": "writing",
		"詩": "writing", "poetry": "writing",
		// business / finance / crypto
		"ビジネス": "business", "起業": "business", "スタートアップ": "business",
		"startup": "business", "マーケティング": "business", "marketing": "business",
		"金融": "finance", "投資": "finance", "investing": "finance", "株": "finance",
		"stocks": "finance", "経済": "finance",
		"仮想通貨": "crypto", "ビットコイン": "crypto", "bitcoin": "crypto", "btc": "crypto",
		"eth": "crypto", "nft": "crypto", "web3": "crypto",
		// science / nature / pets
		"科学": "science", "サイエンス": "science", "宇宙": "science", "space": "science",
		"physics": "science", "astronomy": "science",
		"自然": "nature", "アウトドア": "nature", "outdoor": "nature", "キャンプ": "nature",
		"camping": "nature", "登山": "nature", "hiking": "nature",
		"ペット": "pets", "犬": "pets", "いぬ": "pets", "dog": "pets", "dogs": "pets",
		"猫": "pets", "ねこ": "pets", "cat": "pets", "cats": "pets",
		// politics / education / lifestyle / health / comedy / news
		"政治": "politics", "選挙": "politics", "election": "politics",
		"教育": "education", "勉強": "education", "study": "education", "学習": "education",
		"learning": "education",
		"ライフスタイル":  "lifestyle", "日常": "lifestyle", "daily": "lifestyle", "vlog": "lifestyle",
		"健康": "health", "メンタル": "health", "mentalhealth": "health", "wellness": "health",
		"お笑い": "comedy", "funny": "comedy", "meme": "comedy", "ミーム": "comedy", "ネタ": "comedy",
		"ニュース": "news", "速報": "news", "breaking": "news",
	}
}
