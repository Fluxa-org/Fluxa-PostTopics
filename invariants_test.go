package feedrank

import (
	"fmt"
	"math"
	"math/rand/v2"
	"reflect"
	"testing"
	"time"
)

func genScenario(rng *rand.Rand, nCand int) (UserContext, []Candidate) {
	user := UserContext{
		UserID:               "me",
		Interests:            []string{"tech", "music", "food"},
		Follows:              map[string]bool{},
		AuthorAffinity:       map[string]float64{},
		Seen:                 map[string]bool{},
		NotInterestedTopics:  map[string]bool{"crypto": true},
		NotInterestedAuthors: map[string]bool{},
		MoreInterestedTopics: map[string]bool{"music": true},
	}
	topics := []string{"tech", "music", "food", "crypto", "pets", ""}
	sources := []Source{SourceInNetwork, SourceTopic, SourceTrending, SourceSocialProof, SourceExplore}
	nAuthors := nCand/2 + 1

	var cands []Candidate
	for i := 0; i < nCand; i++ {
		author := fmt.Sprintf("a%d", rng.IntN(nAuthors))
		if rng.IntN(4) == 0 {
			user.Follows[author] = true
		}
		if rng.IntN(10) == 0 {
			user.NotInterestedAuthors[fmt.Sprintf("a%d", rng.IntN(nAuthors))] = true
		}
		if rng.IntN(5) == 0 {
			user.AuthorAffinity[author] = rng.Float64() * 2 // may exceed 1; must clamp
		}
		c := Candidate{
			Post: Post{
				ID:        fmt.Sprintf("p%d", i),
				AuthorID:  author,
				CreatedAt: testNow.Add(-time.Duration(rng.IntN(200)) * time.Hour),
				Likes:     int64(rng.IntN(2000)) - 20,
				Reposts:   int64(rng.IntN(200)),
				Bookmarks: int64(rng.IntN(100)),
				Replies:   int64(rng.IntN(300)),
				Views:     int64(rng.IntN(100000)),
				Topics:    []string{topics[rng.IntN(len(topics))]},
			},
			Source:             sources[rng.IntN(len(sources))],
			EngagedFollows:     rng.IntN(8),
			AuthorPostsLast24h: rng.IntN(50),
			MentionsViewer:     rng.IntN(20) == 0,
		}
		if rng.IntN(6) == 0 {
			c.IsReply = true
			c.ReplyToAuthorID = fmt.Sprintf("a%d", rng.IntN(nAuthors))
		}
		if rng.IntN(15) == 0 {
			c.Labels = []string{"sensitive"}
		}
		if rng.IntN(10) == 0 {
			user.Seen[c.ID] = true
		}
		cands = append(cands, c)
	}
	for i := 0; i < nCand/10; i++ {
		dup := cands[rng.IntN(len(cands))]
		dup.Source = sources[rng.IntN(len(sources))]
		cands = append(cands, dup)
	}
	return user, cands
}

func checkInvariants(t *testing.T, cfg Config, user UserContext, page []Ranked, pageSize int) {
	t.Helper()
	if len(page) > pageSize {
		t.Fatalf("page has %d posts, limit %d", len(page), pageSize)
	}
	maxAge := time.Duration(cfg.MaxAgeHours * float64(time.Hour))
	seen := make(map[string]bool, len(page))
	for _, r := range page {
		if seen[r.ID] {
			t.Fatalf("duplicate post %s in page", r.ID)
		}
		seen[r.ID] = true
		if r.AuthorID == user.UserID {
			t.Fatalf("viewer's own post %s in page", r.ID)
		}
		if user.NotInterestedAuthors[r.AuthorID] {
			t.Fatalf("not-interested author %s in page", r.AuthorID)
		}
		if testNow.Sub(r.CreatedAt) > maxAge {
			t.Fatalf("expired post %s in page", r.ID)
		}
		if r.Score < 0 || math.IsNaN(r.Score) || math.IsInf(r.Score, 0) {
			t.Fatalf("invalid score %v for %s", r.Score, r.ID)
		}
		if len(r.Reasons) == 0 {
			t.Fatalf("post %s has no reasons", r.ID)
		}
		got := r.Breakdown.Freshness * r.Breakdown.Core * r.Breakdown.Penalty
		if math.Abs(got-r.Score) > 1e-9 {
			t.Fatalf("breakdown does not reproduce score for %s: %v vs %v", r.ID, got, r.Score)
		}
	}
}

func TestRankInvariantsRandomized(t *testing.T) {
	cfg := DefaultConfig()
	for iter := 0; iter < 150; iter++ {
		rng := rand.New(rand.NewPCG(42, uint64(iter)))
		user, cands := genScenario(rng, 20+rng.IntN(180))
		pageSize := 1 + rng.IntN(30)

		page := Rank(cfg, user, cands, testNow, pageSize)
		checkInvariants(t, cfg, user, page, pageSize)

		again := Rank(cfg, user, cands, testNow, pageSize)
		if !reflect.DeepEqual(page, again) {
			t.Fatalf("iter %d: Rank is not deterministic", iter)
		}
	}
}

func TestRankInvariantsAcrossProfiles(t *testing.T) {
	for name, cfg := range BuiltinProfiles() {
		t.Run(name, func(t *testing.T) {
			rng := rand.New(rand.NewPCG(7, hashString(name)))
			user, cands := genScenario(rng, 120)
			page := Rank(cfg, user, cands, testNow, 20)
			checkInvariants(t, cfg, user, page, 20)
		})
	}
}

func TestRankExtremeValues(t *testing.T) {
	cfg := DefaultConfig()
	user := UserContext{UserID: "me"}
	cands := []Candidate{
		{Post: Post{ID: "max", AuthorID: "a1", CreatedAt: testNow,
			Likes: math.MaxInt64, Views: math.MaxInt64}, Source: SourceTrending},
		{Post: Post{ID: "neg", AuthorID: "a2", CreatedAt: testNow,
			Likes: -100, Views: -5}, Source: SourceTrending},
		{Post: Post{ID: "future", AuthorID: "a3",
			CreatedAt: testNow.Add(48 * time.Hour)}, Source: SourceTrending},
	}
	page := Rank(cfg, user, cands, testNow, 10)
	if len(page) != 3 {
		t.Fatalf("expected 3 posts, got %d", len(page))
	}
	checkInvariants(t, cfg, user, page, 10)
	for _, r := range page {
		if r.ID == "max" && r.Breakdown.Engagement != 1 {
			t.Fatalf("max engagement should clamp to 1, got %v", r.Breakdown.Engagement)
		}
		if r.ID == "neg" && r.Breakdown.Engagement != 0 {
			t.Fatalf("negative engagement should clamp to 0, got %v", r.Breakdown.Engagement)
		}
	}
}

func TestRankNilMapsSafe(t *testing.T) {
	page := Rank(DefaultConfig(), UserContext{UserID: "me"},
		[]Candidate{cand("p1", "a1", SourceTrending, 1, withLikes(10))}, testNow, 5)
	if len(page) != 1 {
		t.Fatalf("nil user maps must not break ranking: %d", len(page))
	}
}

func FuzzRank(f *testing.F) {
	f.Add(uint64(1), 10, 5)
	f.Add(uint64(99), 0, 1)
	f.Add(uint64(7), 200, 30)
	f.Fuzz(func(t *testing.T, seed uint64, nCand, pageSize int) {
		if nCand < 0 || nCand > 300 || pageSize < -5 || pageSize > 100 {
			t.Skip()
		}
		rng := rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))
		user, cands := genScenario(rng, nCand)
		cfg := DefaultConfig()
		page := Rank(cfg, user, cands, testNow, pageSize)
		if pageSize <= 0 {
			if page != nil {
				t.Fatalf("pageSize %d must yield nil", pageSize)
			}
			return
		}
		checkInvariants(t, cfg, user, page, pageSize)
	})
}

func BenchmarkRank600(b *testing.B) {
	rng := rand.New(rand.NewPCG(1, 2))
	user, cands := genScenario(rng, 600)
	cfg := DefaultConfig()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Rank(cfg, user, cands, testNow, 50)
	}
}

func BenchmarkPostTopics(b *testing.B) {
	aliases := DefaultAliases()
	canonical := make(map[string]bool, 30)
	for _, v := range aliases {
		canonical[v] = true
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		PostTopics("今日は#ラーメン と #音楽 と #プログラミング", []string{"tech"}, canonical, aliases, 3)
	}
}
