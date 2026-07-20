package feedrank

import (
	"math"
	"testing"
)

func TestBuiltinProfilesValidate(t *testing.T) {
	profiles := BuiltinProfiles()
	if len(profiles) < 3 {
		t.Fatalf("expected at least 3 builtin profiles, got %d", len(profiles))
	}
	for name, cfg := range profiles {
		if err := cfg.Validate(); err != nil {
			t.Fatalf("profile %q must validate: %v", name, err)
		}
		if cfg.Version == "" {
			t.Fatalf("profile %q must carry a version", name)
		}
	}
	if profiles["for-you"].Version == profiles["discover"].Version {
		t.Fatal("profiles must have distinct versions")
	}
}

func TestProlificAuthorDamped(t *testing.T) {
	cfg := DefaultConfig() // threshold 10
	user := UserContext{UserID: "me"}

	quiet := cand("quiet", "a1", SourceTrending, 1, withLikes(100))
	quiet.AuthorPostsLast24h = 5
	flood := cand("flood", "a2", SourceTrending, 1, withLikes(100))
	flood.AuthorPostsLast24h = 40

	sQuiet := score(cfg, user, quiet, testNow)
	sFlood := score(cfg, user, flood, testNow)

	if sQuiet.Breakdown.Penalty != 1 {
		t.Fatalf("below-threshold author must not be damped: %v", sQuiet.Breakdown.Penalty)
	}
	// sqrt(10/40) = 0.5
	if math.Abs(sFlood.Breakdown.Penalty-0.5) > 1e-9 {
		t.Fatalf("40 posts/24h at threshold 10 should damp by 0.5, got %v", sFlood.Breakdown.Penalty)
	}
	if sFlood.Score >= sQuiet.Score {
		t.Fatalf("prolific author should rank below quiet twin: %v >= %v", sFlood.Score, sQuiet.Score)
	}

	// Unknown post rate (0) disables the damp.
	unknown := cand("unknown", "a3", SourceTrending, 1, withLikes(100))
	if s := score(cfg, user, unknown, testNow); s.Breakdown.Penalty != 1 {
		t.Fatalf("unknown post rate must not be damped: %v", s.Breakdown.Penalty)
	}
}

func TestMoreLikeThisBoost(t *testing.T) {
	cfg := DefaultConfig()
	user := UserContext{
		UserID:               "me",
		MoreInterestedTopics: map[string]bool{"music": true},
	}
	plain := score(cfg, user, cand("plain", "a1", SourceTrending, 1, withLikes(100)), testNow)
	boosted := score(cfg, user, cand("boosted", "a2", SourceTrending, 1, withLikes(100), withTopics("music")), testNow)

	if boosted.Breakdown.Penalty != cfg.MoreLikeThisBoost {
		t.Fatalf("expected boost multiplier %v, got %v", cfg.MoreLikeThisBoost, boosted.Breakdown.Penalty)
	}
	if boosted.Score <= plain.Score {
		t.Fatalf("boosted topic should outrank plain twin: %v <= %v", boosted.Score, plain.Score)
	}
	if !contains(boosted.Reasons, "more_like_this") {
		t.Fatalf("boosted post should carry more_like_this reason: %v", boosted.Reasons)
	}
}

func TestLabelPenalties(t *testing.T) {
	cfg := DefaultConfig()
	cfg.LabelPenalties = map[string]float64{"sensitive": 0.5, "spam": 0}
	user := UserContext{UserID: "me"}

	mk := func(id string, labels ...string) Candidate {
		c := cand(id, "a-"+id, SourceTrending, 1, withLikes(100))
		c.Labels = labels
		return c
	}

	plain := score(cfg, user, mk("plain"), testNow)
	sensitive := score(cfg, user, mk("sensitive", "sensitive"), testNow)
	spam := score(cfg, user, mk("spam", "spam"), testNow)
	stacked := score(cfg, user, mk("stacked", "sensitive", "sensitive-2"), testNow)
	unknown := score(cfg, user, mk("unknown", "some-other-label"), testNow)

	if sensitive.Score != plain.Score*0.5 {
		t.Fatalf("sensitive label should halve the score: %v vs %v", sensitive.Score, plain.Score)
	}
	if spam.Score != 0 {
		t.Fatalf("spam label should zero the score: %v", spam.Score)
	}
	if unknown.Score != plain.Score {
		t.Fatalf("unconfigured labels must be ignored: %v vs %v", unknown.Score, plain.Score)
	}
	// Only configured labels apply; "sensitive-2" has no entry.
	if stacked.Score != plain.Score*0.5 {
		t.Fatalf("unconfigured label in stack must be ignored: %v", stacked.Score)
	}

	// Stacking two configured labels multiplies.
	cfg.LabelPenalties["graphic"] = 0.5
	double := score(cfg, user, mk("double", "sensitive", "graphic"), testNow)
	if math.Abs(double.Score-plain.Score*0.25) > 1e-12 {
		t.Fatalf("two 0.5 labels should stack to 0.25: %v vs %v", double.Score, plain.Score*0.25)
	}
}

func TestNewKnobsConfigValidation(t *testing.T) {
	invalid := []string{
		`{"more_like_this_boost":0.5}`,
		`{"prolific_damp_threshold":-1}`,
		`{"label_penalties":{"spam":2}}`,
	}
	for _, raw := range invalid {
		if _, err := ConfigFromJSON([]byte(raw)); err == nil {
			t.Fatalf("expected validation error for %s", raw)
		}
	}

	cfg, err := ConfigFromJSON([]byte(`{"label_penalties":{"sensitive":0.4},"prolific_damp_threshold":0}`))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LabelPenalties["sensitive"] != 0.4 || cfg.ProlificDampThreshold != 0 {
		t.Fatalf("overlay failed: %+v", cfg)
	}
}

func TestDiscoverProfileFavorsFreshness(t *testing.T) {
	user := UserContext{UserID: "me"}
	fresh := cand("fresh", "a1", SourceTrending, 1, withLikes(50))
	older := cand("older", "a2", SourceTrending, 12, withLikes(200))

	// Under Discover (tau 4h) the fresh post must win despite fewer likes.
	dCfg := DiscoverConfig()
	dFresh := score(dCfg, user, fresh, testNow)
	dOlder := score(dCfg, user, older, testNow)
	if dFresh.Score <= dOlder.Score {
		t.Fatalf("discover should favor the fresh post: %v <= %v", dFresh.Score, dOlder.Score)
	}
}
