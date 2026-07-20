package feedrank

import (
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"
)

var testNow = time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC)

// cand builds a minimal candidate created `ageH` hours before testNow.
func cand(id, author string, src Source, ageH float64, mut ...func(*Candidate)) Candidate {
	c := Candidate{
		Post: Post{
			ID:        id,
			AuthorID:  author,
			CreatedAt: testNow.Add(-time.Duration(ageH * float64(time.Hour))),
		},
		Source: src,
	}
	for _, m := range mut {
		m(&c)
	}
	return c
}

func withLikes(n int64) func(*Candidate) { return func(c *Candidate) { c.Likes = n } }
func withTopics(t ...string) func(*Candidate) {
	return func(c *Candidate) { c.Topics = t }
}

func TestEngagementScore(t *testing.T) {
	cfg := DefaultConfig()
	tests := []struct {
		name string
		post Post
		want func(got float64) bool
	}{
		{"zero engagement is zero", Post{}, func(g float64) bool { return g == 0 }},
		{"saturates at cap", Post{Likes: 1_000_000}, func(g float64) bool { return g == 1 }},
		{"in range", Post{Likes: 100}, func(g float64) bool { return g > 0 && g < 1 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := engagementScore(cfg, tt.post); !tt.want(got) {
				t.Fatalf("engagementScore = %v", got)
			}
		})
	}

	// Monotonic in likes.
	prev := -1.0
	for _, likes := range []int64{0, 1, 10, 100, 1000} {
		got := engagementScore(cfg, Post{Likes: likes})
		if got < prev {
			t.Fatalf("engagement not monotonic at likes=%d: %v < %v", likes, got, prev)
		}
		prev = got
	}

	// Replies outweigh likes count-for-count.
	if engagementScore(cfg, Post{Replies: 10}) <= engagementScore(cfg, Post{Likes: 10}) {
		t.Fatal("10 replies should outscore 10 likes")
	}
}

func TestFreshness(t *testing.T) {
	cfg := DefaultConfig()
	tests := []struct {
		name string
		age  time.Duration
		want func(got float64) bool
	}{
		{"brand new", 0, func(g float64) bool { return g == 1 }},
		{"future post treated as new", -time.Hour, func(g float64) bool { return g == 1 }},
		{"one tau ~ e^-1", 8 * time.Hour, func(g float64) bool { return math.Abs(g-math.Exp(-1)) < 1e-9 }},
		{"floor holds", 6 * 24 * time.Hour, func(g float64) bool { return g == cfg.FreshnessFloor }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := freshness(cfg, tt.age); !tt.want(got) {
				t.Fatalf("freshness(%v) = %v", tt.age, got)
			}
		})
	}
}

func TestTopicScore(t *testing.T) {
	tests := []struct {
		name      string
		interests []string
		topics    []string
		want      float64
		wantMatch string
	}{
		{"no overlap", []string{"tech"}, []string{"food"}, 0, ""},
		{"full overlap", []string{"tech"}, []string{"tech"}, 1, "tech"},
		{"partial", []string{"tech", "music"}, []string{"tech", "food"}, 1.0 / 3.0, "tech"},
		{"empty interests", nil, []string{"tech"}, 0, ""},
		{"duplicate tags deduped", []string{"tech", "tech"}, []string{"tech"}, 1, "tech"},
		{"match is lexicographically first", []string{"music", "art"}, []string{"music", "art"}, 1, "art"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, match := topicScore(tt.interests, tt.topics)
			if math.Abs(got-tt.want) > 1e-9 || match != tt.wantMatch {
				t.Fatalf("topicScore = (%v, %q), want (%v, %q)", got, match, tt.want, tt.wantMatch)
			}
		})
	}
}

func TestRankHardFilters(t *testing.T) {
	cfg := DefaultConfig()
	user := UserContext{
		UserID:               "me",
		NotInterestedAuthors: map[string]bool{"blocked-author": true},
	}
	candidates := []Candidate{
		cand("own", "me", SourceInNetwork, 1, withLikes(100)),
		cand("not-interested", "blocked-author", SourceTrending, 1, withLikes(100)),
		cand("too-old", "a1", SourceTrending, 200, withLikes(100)),
		cand("no-author", "", SourceTrending, 1, withLikes(100)),
		cand("keeper", "a2", SourceTrending, 1, withLikes(100)),
	}
	got := Rank(cfg, user, candidates, testNow, 10)
	if len(got) != 1 || got[0].ID != "keeper" {
		t.Fatalf("hard filters failed, got %v", ids(got))
	}
}

func TestRankDeterminism(t *testing.T) {
	cfg := DefaultConfig()
	user := UserContext{
		UserID:    "me",
		Interests: []string{"tech", "music"},
		Follows:   map[string]bool{"friend": true},
	}
	var candidates []Candidate
	for i := 0; i < 40; i++ {
		candidates = append(candidates,
			cand(fmt.Sprintf("p%d", i), fmt.Sprintf("a%d", i), SourceTrending, float64(i%24), withLikes(int64(i*7%50))),
			cand(fmt.Sprintf("x%d", i), fmt.Sprintf("xa%d", i), SourceExplore, float64(i%48)))
	}
	a := Rank(cfg, user, candidates, testNow, 20)
	b := Rank(cfg, user, candidates, testNow, 20)
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("Rank is not deterministic:\n%v\n%v", ids(a), ids(b))
	}
	if len(a) != 20 {
		t.Fatalf("expected full page, got %d", len(a))
	}
}

func TestAuthorCap(t *testing.T) {
	cfg := DefaultConfig()
	user := UserContext{UserID: "me"}
	var candidates []Candidate
	for i := 0; i < 6; i++ {
		candidates = append(candidates, cand(fmt.Sprintf("spam%d", i), "spammer", SourceTrending, 1, withLikes(1000)))
	}
	for i := 0; i < 6; i++ {
		candidates = append(candidates, cand(fmt.Sprintf("ok%d", i), fmt.Sprintf("a%d", i), SourceTrending, 2, withLikes(10)))
	}
	got := Rank(cfg, user, candidates, testNow, 8)
	spam := 0
	for _, r := range got {
		if r.AuthorID == "spammer" {
			spam++
		}
	}
	if spam != cfg.AuthorCapPerPage {
		t.Fatalf("author cap violated: %d posts by one author in %v", spam, ids(got))
	}
	if len(got) != 8 {
		t.Fatalf("page should still fill: got %d", len(got))
	}
}

func TestAuthorCapRelaxedOnlyWhenSupplyShort(t *testing.T) {
	cfg := DefaultConfig()
	user := UserContext{UserID: "me"}
	var candidates []Candidate
	for i := 0; i < 5; i++ {
		candidates = append(candidates, cand(fmt.Sprintf("solo%d", i), "solo", SourceTrending, 1, withLikes(int64(100-i))))
	}
	got := Rank(cfg, user, candidates, testNow, 4)
	if len(got) != 4 {
		t.Fatalf("short supply should relax the cap to fill the page, got %d", len(got))
	}
}

func TestSeenPenaltyRanksBelowUnseenTwin(t *testing.T) {
	cfg := DefaultConfig()
	user := UserContext{
		UserID: "me",
		Seen:   map[string]bool{"seen": true},
	}
	candidates := []Candidate{
		cand("seen", "a1", SourceTrending, 1, withLikes(500)),
		cand("unseen", "a2", SourceTrending, 1, withLikes(500)),
	}
	got := Rank(cfg, user, candidates, testNow, 2)
	if len(got) != 2 || got[0].ID != "unseen" {
		t.Fatalf("seen post should rank below unseen twin: %v", ids(got))
	}
	if got[1].Breakdown.Penalty != cfg.Penalties.Seen {
		t.Fatalf("expected seen penalty %v, got %v", cfg.Penalties.Seen, got[1].Breakdown.Penalty)
	}
}

func TestReplyToNonFollowedPenalty(t *testing.T) {
	cfg := DefaultConfig()
	user := UserContext{UserID: "me", Follows: map[string]bool{"friend": true}}
	mkReply := func(id, replyTo string) Candidate {
		c := cand(id, "author", SourceTrending, 1, withLikes(100))
		c.IsReply = true
		c.ReplyToAuthorID = replyTo
		return c
	}
	toFriend := score(cfg, user, mkReply("r1", "friend"), testNow)
	toStranger := score(cfg, user, mkReply("r2", "stranger"), testNow)
	if toStranger.Score >= toFriend.Score {
		t.Fatalf("reply to stranger should be penalized: %v >= %v", toStranger.Score, toFriend.Score)
	}
	if toFriend.Breakdown.Penalty != 1 {
		t.Fatalf("reply to followed author should not be penalized: %v", toFriend.Breakdown.Penalty)
	}
}

func TestNotInterestedTopicDownranked(t *testing.T) {
	cfg := DefaultConfig()
	user := UserContext{UserID: "me", NotInterestedTopics: map[string]bool{"crypto": true}}
	plain := score(cfg, user, cand("p1", "a1", SourceTrending, 1, withLikes(100)), testNow)
	muted := score(cfg, user, cand("p2", "a1", SourceTrending, 1, withLikes(100), withTopics("crypto")), testNow)
	if muted.Score >= plain.Score {
		t.Fatalf("not-interested topic should be downranked: %v >= %v", muted.Score, plain.Score)
	}
}

func TestInNetworkQuota(t *testing.T) {
	cfg := DefaultConfig()
	follows := map[string]bool{}
	var candidates []Candidate
	// 10 in-network posts, all scoring higher than the out-of-network ones.
	for i := 0; i < 10; i++ {
		a := fmt.Sprintf("friend%d", i)
		follows[a] = true
		candidates = append(candidates, cand(fmt.Sprintf("in%d", i), a, SourceInNetwork, 1, withLikes(1000)))
	}
	for i := 0; i < 10; i++ {
		candidates = append(candidates, cand(fmt.Sprintf("out%d", i), fmt.Sprintf("o%d", i), SourceTrending, 1, withLikes(10)))
	}
	user := UserContext{UserID: "me", Follows: follows}

	got := Rank(cfg, user, candidates, testNow, 10)
	inNet := 0
	for _, r := range got {
		if strings.HasPrefix(r.ID, "in") {
			inNet++
		}
	}
	want := int(math.Ceil(cfg.InNetworkTargetRatio * 10))
	if inNet != want {
		t.Fatalf("in-network quota: got %d in-network of %v, want %d", inNet, ids(got), want)
	}

	// When there is no out-of-network supply the page must still fill.
	onlyIn := candidates[:10]
	got = Rank(cfg, user, onlyIn, testNow, 8)
	if len(got) != 8 {
		t.Fatalf("quota must not starve the page: got %d", len(got))
	}
}

func TestExploreSlotInjection(t *testing.T) {
	cfg := DefaultConfig()
	user := UserContext{UserID: "me"}
	var candidates []Candidate
	for i := 0; i < 15; i++ {
		candidates = append(candidates, cand(fmt.Sprintf("top%d", i), fmt.Sprintf("a%d", i), SourceTrending, 1, withLikes(int64(1000-i))))
	}
	for i := 0; i < 3; i++ {
		candidates = append(candidates, cand(fmt.Sprintf("exp%d", i), fmt.Sprintf("e%d", i), SourceExplore, 40))
	}
	got := Rank(cfg, user, candidates, testNow, 10)
	if len(got) != 10 {
		t.Fatalf("expected full page, got %d", len(got))
	}
	slot := cfg.ExploreSlotEvery - 1
	if got[slot].Source != SourceExplore {
		t.Fatalf("position %d should be an explore slot, got %v (%v)", slot, got[slot].Source, ids(got))
	}
	if !contains(got[slot].Reasons, "explore") {
		t.Fatalf("explore slot should carry the explore reason: %v", got[slot].Reasons)
	}
}

func TestTopicRunBroken(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ExploreSlotEvery = 0 // isolate the adjacency rule
	user := UserContext{UserID: "me", Interests: []string{"tech"}}
	var candidates []Candidate
	// 4 consecutive tech posts, then one food post with lower score.
	for i := 0; i < 4; i++ {
		candidates = append(candidates, cand(fmt.Sprintf("tech%d", i), fmt.Sprintf("a%d", i), SourceTopic, 1, withLikes(int64(100-i)), withTopics("tech")))
	}
	candidates = append(candidates, cand("food0", "b0", SourceTrending, 1, withLikes(1), withTopics("food")))

	got := Rank(cfg, user, candidates, testNow, 5)
	if len(got) != 5 {
		t.Fatalf("expected 5 posts, got %d", len(got))
	}
	run := 0
	maxRun := 0
	last := ""
	for _, r := range got {
		d := dominantTopic(r.Post)
		if d != "" && d == last {
			run++
		} else {
			run = 1
		}
		if run > maxRun {
			maxRun = run
		}
		last = d
	}
	if maxRun > cfg.TopicRunCap {
		t.Fatalf("topic run of %d exceeds cap %d: %v", maxRun, cfg.TopicRunCap, ids(got))
	}
}

func TestDedupeMergesSources(t *testing.T) {
	dup := []Candidate{
		func() Candidate {
			c := cand("p1", "a1", SourceTrending, 1)
			c.EngagedFollows = 1
			return c
		}(),
		func() Candidate {
			c := cand("p1", "a1", SourceInNetwork, 1)
			c.EngagedFollows = 3
			return c
		}(),
	}
	got := dedupe(dup)
	if len(got) != 1 {
		t.Fatalf("expected 1 candidate after dedupe, got %d", len(got))
	}
	if got[0].Source != SourceInNetwork || got[0].EngagedFollows != 3 {
		t.Fatalf("dedupe should keep best source and max engaged follows: %+v", got[0])
	}
}

func TestReasons(t *testing.T) {
	cfg := DefaultConfig()
	user := UserContext{
		UserID:    "me",
		Interests: []string{"music"},
		Follows:   map[string]bool{"friend": true},
	}
	c := cand("p1", "friend", SourceInNetwork, 1, withTopics("music"))
	c.EngagedFollows = 2
	r := score(cfg, user, c, testNow)
	for _, want := range []string{"followed_author", "topic:music", "liked_by_follows"} {
		if !contains(r.Reasons, want) {
			t.Fatalf("missing reason %q in %v", want, r.Reasons)
		}
	}

	anon := score(cfg, UserContext{UserID: "me"}, cand("p2", "a1", SourceTopic, 1), testNow)
	if !reflect.DeepEqual(anon.Reasons, []string{"popular"}) {
		t.Fatalf("fallback reason should be popular, got %v", anon.Reasons)
	}
}

func TestScoreBreakdownConsistency(t *testing.T) {
	cfg := DefaultConfig()
	user := UserContext{
		UserID:         "me",
		Interests:      []string{"tech"},
		Follows:        map[string]bool{"friend": true},
		AuthorAffinity: map[string]float64{"friend": 0.5},
		Seen:           map[string]bool{"p1": true},
	}
	c := cand("p1", "friend", SourceInNetwork, 4, withLikes(50), withTopics("tech"))
	r := score(cfg, user, c, testNow)
	want := r.Breakdown.Freshness * r.Breakdown.Core * r.Breakdown.Penalty
	if math.Abs(r.Score-want) > 1e-12 || math.Abs(r.Breakdown.Score-r.Score) > 1e-12 {
		t.Fatalf("breakdown does not reproduce score: %v vs %v", r.Score, want)
	}
	if r.Breakdown.Affinity != math.Min(1, 0.5+cfg.FollowBonus) {
		t.Fatalf("affinity with follow bonus wrong: %v", r.Breakdown.Affinity)
	}
}

func TestRankOrderingPrefersPersonalRelevance(t *testing.T) {
	cfg := DefaultConfig()
	user := UserContext{
		UserID:         "me",
		Interests:      []string{"tech"},
		Follows:        map[string]bool{"friend": true},
		AuthorAffinity: map[string]float64{"friend": 0.6},
	}
	candidates := []Candidate{
		// Same engagement and age; only relationship/topic differ.
		cand("from-friend", "friend", SourceInNetwork, 2, withLikes(50)),
		cand("on-topic", "a1", SourceTopic, 2, withLikes(50), withTopics("tech")),
		cand("random", "a2", SourceTrending, 2, withLikes(50)),
	}
	got := Rank(cfg, user, candidates, testNow, 3)
	if len(got) != 3 || got[0].ID != "from-friend" || got[2].ID != "random" {
		t.Fatalf("expected [from-friend, on-topic, random], got %v", ids(got))
	}
}

func TestConfigFromJSON(t *testing.T) {
	t.Run("overlay keeps defaults", func(t *testing.T) {
		cfg, err := ConfigFromJSON([]byte(`{"version":"test.1","weights":{"engagement":0.5,"affinity":0.2,"topic":0.2,"social_proof":0.1}}`))
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Version != "test.1" || cfg.Weights.Engagement != 0.5 {
			t.Fatalf("overlay failed: %+v", cfg)
		}
		if cfg.AuthorCapPerPage != DefaultConfig().AuthorCapPerPage {
			t.Fatalf("unset fields should keep defaults: %+v", cfg)
		}
	})

	t.Run("unknown field rejected", func(t *testing.T) {
		if _, err := ConfigFromJSON([]byte(`{"weigths":{}}`)); err == nil {
			t.Fatal("typo'd field should be rejected")
		}
	})

	invalid := []string{
		`{"engagement_cap":0}`,
		`{"freshness_tau_hours":-1}`,
		`{"penalties":{"seen":2,"not_interested_topic":0.2,"reply_to_non_followed":0.3}}`,
		`{"author_cap_per_page":0}`,
		`{"in_network_target_ratio":1.5}`,
		`{"topic_run_cap":1}`,
		`{"explore_slot_every":1}`,
		`{"weights":{"engagement":0,"affinity":0,"topic":0,"social_proof":0}}`,
	}
	for _, raw := range invalid {
		if _, err := ConfigFromJSON([]byte(raw)); err == nil {
			t.Fatalf("expected validation error for %s", raw)
		}
	}

	if err := DefaultConfig().Validate(); err != nil {
		t.Fatalf("default config must validate: %v", err)
	}
}

func TestRankEmptyInputs(t *testing.T) {
	cfg := DefaultConfig()
	if got := Rank(cfg, UserContext{UserID: "me"}, nil, testNow, 10); got != nil {
		t.Fatalf("nil candidates should rank to nil, got %v", ids(got))
	}
	if got := Rank(cfg, UserContext{UserID: "me"}, []Candidate{cand("p1", "a1", SourceTrending, 1)}, testNow, 0); got != nil {
		t.Fatalf("pageSize 0 should rank to nil, got %v", ids(got))
	}
}

func ids(rs []Ranked) []string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = r.ID
	}
	return out
}

func contains(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}
