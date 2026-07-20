package feedrank

import (
	"reflect"
	"testing"
)

var testCanonical = map[string]bool{
	"tech": true, "music": true, "food": true, "pets": true, "programming": true,
}

func TestExtractHashtags(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []string
	}{
		{"none", "just words", nil},
		{"basic", "loving #Tech today", []string{"tech"}},
		{"japanese", "今日の#ラーメン と #猫 最高", []string{"ラーメン", "猫"}},
		{"dedupe and order", "#music #tech #music", []string{"music", "tech"}},
		{"underscore and digits", "#web3_dev #2026", []string{"web3_dev", "2026"}},
		{"hash alone ignored", "# nothing", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExtractHashtags(tt.text); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ExtractHashtags(%q) = %v, want %v", tt.text, got, tt.want)
			}
		})
	}
}

func TestMapTopics(t *testing.T) {
	aliases := map[string]string{"ラーメン": "food", "猫": "pets", "coding": "programming"}
	tests := []struct {
		name string
		raw  []string
		max  int
		want []string
	}{
		{"canonical passthrough", []string{"tech"}, 0, []string{"tech"}},
		{"alias mapping", []string{"ラーメン", "猫"}, 0, []string{"food", "pets"}},
		{"unmapped dropped", []string{"randomtag", "tech"}, 0, []string{"tech"}},
		{"dedupe after mapping", []string{"coding", "programming"}, 0, []string{"programming"}},
		{"cap respected", []string{"tech", "music", "food"}, 2, []string{"tech", "music"}},
		{"empty raw", nil, 0, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MapTopics(tt.raw, testCanonical, aliases, tt.max); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("MapTopics(%v) = %v, want %v", tt.raw, got, tt.want)
			}
		})
	}
}

func TestPostTopicsFallback(t *testing.T) {
	aliases := map[string]string{"ラーメン": "food"}

	// Hashtag maps → hashtags win.
	got := PostTopics("うまい#ラーメン", []string{"music"}, testCanonical, aliases, 0)
	if !reflect.DeepEqual(got, []string{"food"}) {
		t.Fatalf("hashtags should win: %v", got)
	}

	// No hashtag maps → author interests fallback.
	got = PostTopics("no tags here #unmappable", []string{"music", "tech"}, testCanonical, aliases, 0)
	if !reflect.DeepEqual(got, []string{"music", "tech"}) {
		t.Fatalf("fallback to author interests failed: %v", got)
	}

	// Nothing maps at all.
	if got = PostTopics("plain", []string{"unknown"}, testCanonical, aliases, 0); got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

func TestDefaultAliasesAreCanonical(t *testing.T) {
	// Every alias target must be one of the 30 canonical Fluxa tags, kept as
	// a literal here so the package stays self-contained; adapters wire the
	// live taxonomy (internal/shared/interests.Canonical) at call sites.
	canonical := map[string]bool{
		"tech": true, "programming": true, "ai": true, "gaming": true, "design": true,
		"photography": true, "music": true, "film": true, "tv": true, "sports": true,
		"fitness": true, "travel": true, "food": true, "fashion": true, "beauty": true,
		"art": true, "literature": true, "writing": true, "business": true, "finance": true,
		"crypto": true, "science": true, "nature": true, "pets": true, "politics": true,
		"education": true, "lifestyle": true, "health": true, "comedy": true, "news": true,
	}
	for alias, target := range DefaultAliases() {
		if !canonical[target] {
			t.Fatalf("alias %q maps to non-canonical topic %q", alias, target)
		}
	}
}

func TestMentionBoost(t *testing.T) {
	cfg := DefaultConfig()
	user := UserContext{UserID: "me"}

	plain := cand("plain", "a1", SourceTrending, 1, withLikes(100))
	mention := cand("mention", "a2", SourceTrending, 1, withLikes(100))
	mention.MentionsViewer = true

	sPlain := score(cfg, user, plain, testNow)
	sMention := score(cfg, user, mention, testNow)

	if sMention.Breakdown.Penalty != cfg.MentionBoost {
		t.Fatalf("expected mention boost %v, got %v", cfg.MentionBoost, sMention.Breakdown.Penalty)
	}
	if sMention.Score <= sPlain.Score {
		t.Fatalf("mentioning post should outrank plain twin: %v <= %v", sMention.Score, sPlain.Score)
	}
	if !contains(sMention.Reasons, "mentions_you") {
		t.Fatalf("missing mentions_you reason: %v", sMention.Reasons)
	}

	if _, err := ConfigFromJSON([]byte(`{"mention_boost":0.5}`)); err == nil {
		t.Fatal("mention_boost < 1 must be rejected")
	}
}
