package feedrank_test

import (
	"fmt"
	"time"

	feedrank "github.com/Fluxa-org/Fluxa-PostTopics"
)

// Example shows the full adapter flow: map hashtags to topics, build
// candidates, rank, and read the explainable output.
func Example() {
	now := time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC)
	canonical := map[string]bool{"music": true, "food": true}

	topics := feedrank.PostTopics(
		"新曲できた！ #音楽", nil, canonical, feedrank.DefaultAliases(), 3)

	user := feedrank.UserContext{
		UserID:    "viewer",
		Interests: []string{"music"},
		Follows:   map[string]bool{"alice": true},
	}
	candidates := []feedrank.Candidate{
		{
			Post: feedrank.Post{
				ID: "post-song", AuthorID: "alice",
				CreatedAt: now.Add(-2 * time.Hour),
				Likes:     12, Replies: 3, Topics: topics,
			},
			Source: feedrank.SourceInNetwork,
		},
		{
			Post: feedrank.Post{
				ID: "post-viral", AuthorID: "stranger",
				CreatedAt: now.Add(-20 * time.Hour),
				Likes:     900, Reposts: 200,
			},
			Source: feedrank.SourceTrending,
		},
	}

	page := feedrank.Rank(feedrank.DefaultConfig(), user, candidates, now, 2)
	for _, r := range page {
		fmt.Printf("%s reasons=%v\n", r.ID, r.Reasons)
	}
	// Output:
	// post-song reasons=[followed_author topic:music]
	// post-viral reasons=[trending]
}
