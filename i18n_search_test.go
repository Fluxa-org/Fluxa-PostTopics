package feedrank

import (
	"reflect"
	"strings"
	"testing"
)

func TestExtractMentions(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []string
	}{
		{"none", "no mentions here", nil},
		{"basic", "hey @Alice look", []string{"alice"}},
		{"multiple deduped", "@bob and @carol and @bob", []string{"bob", "carol"}},
		{"unicode handle", "cc @山田太郎 さん", []string{"山田太郎"}},
		{"email not fully matched", "mail me a@b.com", []string{"b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExtractMentions(tt.text); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ExtractMentions(%q) = %v, want %v", tt.text, got, tt.want)
			}
		})
	}
}

func TestMapQuery(t *testing.T) {
	canonical := map[string]bool{"music": true, "food": true, "travel": true}
	aliases := DefaultAliases()
	tests := []struct {
		name  string
		query string
		want  []string
	}{
		{"japanese query", "ラーメン 東京", []string{"food"}},
		{"spanish query", "recetas de cocina", []string{"food"}},
		{"mixed languages", "music と 旅行", []string{"music", "travel"}},
		{"punctuation split", "musique,voyage!", []string{"music", "travel"}},
		{"nothing maps", "asdf qwer", nil},
		{"empty", "", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MapQuery(tt.query, canonical, aliases, 0); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("MapQuery(%q) = %v, want %v", tt.query, got, tt.want)
			}
		})
	}
}

func TestMultilingualAliases(t *testing.T) {
	canonical := map[string]bool{"music": true, "food": true, "sports": true, "pets": true}
	aliases := DefaultAliases()
	for _, tc := range []struct{ tag, want string }{
		{"음악", "music"}, {"音乐", "music"}, {"musica", "music"}, {"musique", "music"},
		{"musik", "music"}, {"संगीत", "music"}, {"amnhac", "music"},
		{"맛집", "food"}, {"美食", "food"}, {"comida", "food"}, {"essen", "food"},
		{"futebol", "sports"}, {"क्रिकेट", "sports"}, {"bongda", "sports"},
		{"宠物", "pets"}, {"chien", "pets"},
	} {
		got := MapTopics([]string{tc.tag}, canonical, aliases, 0)
		if len(got) != 1 || got[0] != tc.want {
			t.Fatalf("alias %q → %v, want [%s]", tc.tag, got, tc.want)
		}
	}
	for k, v := range aliases {
		if k != strings.ToLower(k) {
			t.Fatalf("alias key %q must be lowercase", k)
		}
		if v == "" {
			t.Fatalf("alias %q has empty target", k)
		}
	}
}

func TestLanguageMismatchPenalty(t *testing.T) {
	cfg := DefaultConfig()
	user := UserContext{UserID: "me", Languages: map[string]bool{"ja": true}}
	mk := func(id, lang string) Candidate {
		c := cand(id, "a-"+id, SourceTrending, 1, withLikes(100))
		c.Language = lang
		return c
	}

	match := score(cfg, user, mk("match", "ja"), testNow)
	mismatch := score(cfg, user, mk("mismatch", "de"), testNow)
	unknown := score(cfg, user, mk("unknown", ""), testNow)

	if mismatch.Breakdown.Penalty != cfg.LanguageMismatchPenalty {
		t.Fatalf("mismatch penalty = %v, want %v", mismatch.Breakdown.Penalty, cfg.LanguageMismatchPenalty)
	}
	if match.Breakdown.Penalty != 1 || unknown.Breakdown.Penalty != 1 {
		t.Fatalf("matching/unknown language must not be penalized: %v / %v",
			match.Breakdown.Penalty, unknown.Breakdown.Penalty)
	}

	// No viewer languages → matching disabled.
	anon := score(cfg, UserContext{UserID: "me"}, mk("any", "de"), testNow)
	if anon.Breakdown.Penalty != 1 {
		t.Fatalf("empty Languages must disable matching: %v", anon.Breakdown.Penalty)
	}

	if _, err := ConfigFromJSON([]byte(`{"language_mismatch_penalty":2}`)); err == nil {
		t.Fatal("language_mismatch_penalty > 1 must be rejected")
	}
}

func TestTrendingTopicsTerm(t *testing.T) {
	cfg := DefaultConfig()
	user := UserContext{
		UserID:         "me",
		TrendingTopics: map[string]float64{"music": 0.8, "food": 2.0},
	}

	plain := score(cfg, user, cand("plain", "a1", SourceTopic, 1, withLikes(100)), testNow)
	hot := score(cfg, user, cand("hot", "a2", SourceTopic, 1, withLikes(100), withTopics("music")), testNow)
	over := score(cfg, user, cand("over", "a3", SourceTopic, 1, withLikes(100), withTopics("food")), testNow)

	if hot.Breakdown.Trending != 0.8 {
		t.Fatalf("trending term = %v, want 0.8", hot.Breakdown.Trending)
	}
	if hot.Score <= plain.Score {
		t.Fatalf("search-trending topic should outrank plain twin: %v <= %v", hot.Score, plain.Score)
	}
	if !contains(hot.Reasons, "trending_search") {
		t.Fatalf("missing trending_search reason: %v", hot.Reasons)
	}
	if over.Breakdown.Trending != 1 {
		t.Fatalf("trending heat must clamp to 1, got %v", over.Breakdown.Trending)
	}
	if plain.Breakdown.Trending != 0 || contains(plain.Reasons, "trending_search") {
		t.Fatalf("untagged post must not get the trending term: %+v", plain.Breakdown)
	}

	if _, err := ConfigFromJSON([]byte(`{"weights":{"engagement":0.3,"affinity":0.3,"topic":0.2,"social_proof":0.1,"trending":-1}}`)); err == nil {
		t.Fatal("negative trending weight must be rejected")
	}
}

func TestSearchInterestsJoinTopicTerm(t *testing.T) {
	cfg := DefaultConfig()
	base := UserContext{UserID: "me"}
	searcher := UserContext{UserID: "me", SearchInterests: []string{"pets"}}
	c := cand("p1", "a1", SourceTopic, 1, withLikes(100), withTopics("pets"))

	without := score(cfg, base, c, testNow)
	with := score(cfg, searcher, c, testNow)

	if without.Breakdown.Topic != 0 {
		t.Fatalf("no interests should mean no topic score: %v", without.Breakdown.Topic)
	}
	if with.Breakdown.Topic != 1 {
		t.Fatalf("search interests must join the topic term: %v", with.Breakdown.Topic)
	}
	if !contains(with.Reasons, "topic:pets") {
		t.Fatalf("expected topic:pets reason, got %v", with.Reasons)
	}

	// Interests and SearchInterests merge (dedup happens inside topicScore).
	both := UserContext{UserID: "me", Interests: []string{"pets"}, SearchInterests: []string{"pets", "music"}}
	merged := score(cfg, both, c, testNow)
	if merged.Breakdown.Topic != 0.5 {
		t.Fatalf("merged interests Jaccard = %v, want 0.5", merged.Breakdown.Topic)
	}
}
