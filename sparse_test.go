package feedrank

import (
	"fmt"
	"testing"
)

func TestSparseRelaxFillsFromExpired(t *testing.T) {
	user := UserContext{UserID: "me"}
	var cands []Candidate
	for i := 0; i < 2; i++ {
		cands = append(cands, cand(fmt.Sprintf("fresh%d", i), fmt.Sprintf("f%d", i), SourceTrending, 1, withLikes(10)))
	}
	for i := 0; i < 5; i++ {
		cands = append(cands, cand(fmt.Sprintf("old%d", i), fmt.Sprintf("o%d", i), SourceTrending, 200+float64(i)))
	}

	page := Rank(DefaultConfig(), user, cands, testNow, 6)
	if len(page) != 6 {
		t.Fatalf("sparse relax should fill the page: got %d, want 6", len(page))
	}
	if page[0].ID != "fresh0" && page[0].ID != "fresh1" {
		t.Fatalf("fresh posts must outrank readmitted expired ones: top=%s", page[0].ID)
	}
	checkInvariants(t, DefaultConfig(), user, page, 6)

	strict := DefaultConfig()
	strict.RelaxMaxAgeWhenSparse = false
	page = Rank(strict, user, cands, testNow, 6)
	if len(page) != 2 {
		t.Fatalf("with relax disabled only fresh posts remain: got %d, want 2", len(page))
	}
}

func TestSparseRelaxNotTriggeredWhenSupplyEnough(t *testing.T) {
	user := UserContext{UserID: "me"}
	var cands []Candidate
	for i := 0; i < 10; i++ {
		cands = append(cands, cand(fmt.Sprintf("fresh%d", i), fmt.Sprintf("f%d", i), SourceTrending, 1, withLikes(int64(10-i))))
	}
	for i := 0; i < 5; i++ {
		cands = append(cands, cand(fmt.Sprintf("old%d", i), fmt.Sprintf("o%d", i), SourceTrending, 300, withLikes(1000)))
	}

	page := Rank(DefaultConfig(), user, cands, testNow, 5)
	if len(page) != 5 {
		t.Fatalf("expected full page, got %d", len(page))
	}
	for _, r := range page {
		if testNow.Sub(r.CreatedAt).Hours() > DefaultConfig().MaxAgeHours {
			t.Fatalf("expired post %s admitted despite sufficient fresh supply", r.ID)
		}
	}
}

func TestSmallCommunityProfile(t *testing.T) {
	cfg := SmallCommunityConfig()
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	cfg.RelaxMaxAgeWhenSparse = false // prove the wider window itself admits old posts

	user := UserContext{UserID: "me"}
	twentyDaysOld := []Candidate{cand("p1", "a1", SourceTrending, 480, withLikes(5))}

	if page := Rank(cfg, user, twentyDaysOld, testNow, 5); len(page) != 1 {
		t.Fatalf("small-community 30-day window should admit a 20-day-old post: got %d", len(page))
	}

	strictDefault := DefaultConfig()
	strictDefault.RelaxMaxAgeWhenSparse = false
	if page := Rank(strictDefault, user, twentyDaysOld, testNow, 5); len(page) != 0 {
		t.Fatalf("default 7-day window should drop a 20-day-old post: got %d", len(page))
	}
}
