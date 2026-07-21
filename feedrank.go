package feedrank

import (
	"hash/fnv"
	"math"
	"math/rand/v2"
	"sort"
	"time"
)

// Source tags where a candidate came from.
type Source string

const (
	SourceInNetwork   Source = "in_network"
	SourceSocialProof Source = "social_proof"
	SourceTopic       Source = "topic"
	SourceTrending    Source = "trending"
	SourceExplore     Source = "explore"
)

var sourcePriority = map[Source]int{
	SourceInNetwork:   0,
	SourceSocialProof: 1,
	SourceTopic:       2,
	SourceTrending:    3,
	SourceExplore:     4,
}

// Post is the ranking view of a post.
type Post struct {
	ID        string
	AuthorID  string
	CreatedAt time.Time

	Likes     int64
	Reposts   int64
	Bookmarks int64
	Replies   int64
	Views     int64

	// Topics are canonical taxonomy tags; see PostTopics.
	Topics []string
	// Labels are moderation labels matched against Config.LabelPenalties.
	Labels []string
	// Language is the post's lowercase primary language tag ("ja", "en");
	// "" = unknown (never penalized).
	Language string

	IsReply         bool
	ReplyToAuthorID string
}

// Candidate is a post plus sourcing metadata.
type Candidate struct {
	Post
	Source Source
	// EngagedFollows counts followed accounts that engaged with the post.
	EngagedFollows int
	// AuthorPostsLast24h is the author's posting rate; 0 = unknown.
	AuthorPostsLast24h int
	MentionsViewer     bool
}

// UserContext holds the viewer's signals; zero values mean "no signal".
type UserContext struct {
	UserID               string
	Interests            []string
	Follows              map[string]bool
	AuthorAffinity       map[string]float64 // 0..1 per author
	Seen                 map[string]bool
	NotInterestedTopics  map[string]bool
	NotInterestedAuthors map[string]bool
	MoreInterestedTopics map[string]bool
	// Languages is the set of languages the viewer reads; empty disables
	// language matching.
	Languages map[string]bool
	// SearchInterests are topics derived from the viewer's own recent
	// searches (see MapQuery); they join Interests in the topic term.
	SearchInterests []string
	// TrendingTopics is the network-wide search heat per topic in [0,1],
	// aggregated by the caller from all users' search queries.
	TrendingTopics map[string]float64
}

// Breakdown decomposes a score: Score = Freshness × Core × Penalty.
// Penalty is the combined modifier and can exceed 1 when boosts apply.
type Breakdown struct {
	Engagement  float64 `json:"engagement"`
	Affinity    float64 `json:"affinity"`
	Topic       float64 `json:"topic"`
	SocialProof float64 `json:"social_proof"`
	Trending    float64 `json:"trending"`
	Core        float64 `json:"core"`
	Freshness   float64 `json:"freshness"`
	Penalty     float64 `json:"penalty"`
	Score       float64 `json:"score"`
}

// Ranked is a scored candidate in its final page position.
type Ranked struct {
	Candidate
	Score     float64
	Reasons   []string
	Breakdown Breakdown
}

// Rank runs the full pipeline and returns at most pageSize posts in display
// order. Identical inputs (including now) yield identical output.
func Rank(cfg Config, user UserContext, candidates []Candidate, now time.Time, pageSize int) []Ranked {
	if pageSize <= 0 || len(candidates) == 0 {
		return nil
	}

	maxAge := time.Duration(cfg.MaxAgeHours * float64(time.Hour))
	scored := make([]Ranked, 0, len(candidates))
	for _, c := range dedupe(candidates) {
		switch {
		case c.ID == "" || c.AuthorID == "":
		case c.AuthorID == user.UserID:
		case user.NotInterestedAuthors[c.AuthorID]:
		case now.Sub(c.CreatedAt) > maxAge:
		default:
			scored = append(scored, score(cfg, user, c, now))
		}
	}
	if len(scored) == 0 {
		return nil
	}

	orderRanked(scored)
	page := selectPage(cfg, user, scored, pageSize)
	if cfg.TopicRunCap >= 2 {
		breakTopicRuns(page, cfg.TopicRunCap)
	}
	if cfg.ExploreSlotEvery >= 2 {
		injectExplore(cfg, user, page, scored, now)
	}
	return page
}

// dedupe collapses same-ID candidates, keeping the highest-priority source
// and the maximum EngagedFollows.
func dedupe(candidates []Candidate) []Candidate {
	byID := make(map[string]int, len(candidates))
	out := make([]Candidate, 0, len(candidates))
	for _, c := range candidates {
		i, ok := byID[c.ID]
		if !ok {
			byID[c.ID] = len(out)
			out = append(out, c)
			continue
		}
		if sourcePriority[c.Source] < sourcePriority[out[i].Source] {
			out[i].Source = c.Source
		}
		if c.EngagedFollows > out[i].EngagedFollows {
			out[i].EngagedFollows = c.EngagedFollows
		}
	}
	return out
}

func score(cfg Config, user UserContext, c Candidate, now time.Time) Ranked {
	eng := engagementScore(cfg, c.Post)
	aff := affinityScore(cfg, user, c.AuthorID)
	top, matchedTopic := topicScore(mergeInterests(user.Interests, user.SearchInterests), c.Topics)
	proof := socialProofScore(cfg, c.EngagedFollows)
	trend := trendingScore(c.Topics, user.TrendingTopics)

	core := cfg.Weights.Engagement*eng +
		cfg.Weights.Affinity*aff +
		cfg.Weights.Topic*top +
		cfg.Weights.SocialProof*proof +
		cfg.Weights.Trending*trend

	fresh := freshness(cfg, now.Sub(c.CreatedAt))

	penalty := 1.0
	if user.Seen[c.ID] {
		penalty *= cfg.Penalties.Seen
	}
	if anyTopicIn(c.Topics, user.NotInterestedTopics) {
		penalty *= cfg.Penalties.NotInterestedTopic
	}
	if c.IsReply && c.ReplyToAuthorID != "" && !user.Follows[c.ReplyToAuthorID] {
		penalty *= cfg.Penalties.ReplyToNonFollowed
	}
	for _, l := range c.Labels {
		if m, ok := cfg.LabelPenalties[l]; ok {
			penalty *= m
		}
	}
	boosted := anyTopicIn(c.Topics, user.MoreInterestedTopics)
	if boosted {
		penalty *= cfg.MoreLikeThisBoost
	}
	if c.MentionsViewer {
		penalty *= cfg.MentionBoost
	}
	if cfg.ProlificDampThreshold > 0 && c.AuthorPostsLast24h > cfg.ProlificDampThreshold {
		penalty *= math.Sqrt(float64(cfg.ProlificDampThreshold) / float64(c.AuthorPostsLast24h))
	}
	if cfg.LanguageMismatchPenalty < 1 && c.Language != "" &&
		len(user.Languages) > 0 && !user.Languages[c.Language] {
		penalty *= cfg.LanguageMismatchPenalty
	}

	r := Ranked{
		Candidate: c,
		Score:     fresh * core * penalty,
		Breakdown: Breakdown{
			Engagement:  eng,
			Affinity:    aff,
			Topic:       top,
			SocialProof: proof,
			Trending:    trend,
			Core:        core,
			Freshness:   fresh,
			Penalty:     penalty,
		},
	}
	r.Breakdown.Score = r.Score
	r.Reasons = reasons(user, c, matchedTopic, boosted, trend > 0)
	return r
}

func mergeInterests(a, b []string) []string {
	if len(b) == 0 {
		return a
	}
	if len(a) == 0 {
		return b
	}
	out := make([]string, 0, len(a)+len(b))
	return append(append(out, a...), b...)
}

// trendingScore is the best network-wide search heat among the post's topics.
func trendingScore(topics []string, trending map[string]float64) float64 {
	best := 0.0
	for _, t := range topics {
		if v := trending[t]; v > best {
			best = v
		}
	}
	return math.Max(0, math.Min(1, best))
}

// engagementScore log-damps weighted engagement and saturates at
// cfg.EngagementCap, so viral posts cannot drown small accounts.
func engagementScore(cfg Config, p Post) float64 {
	raw := cfg.Engagement.Like*float64(p.Likes) +
		cfg.Engagement.Repost*float64(p.Reposts) +
		cfg.Engagement.Bookmark*float64(p.Bookmarks) +
		cfg.Engagement.Reply*float64(p.Replies) +
		cfg.Engagement.View*float64(p.Views)
	if raw <= 0 {
		return 0
	}
	return math.Min(1, math.Log10(1+raw)/math.Log10(1+cfg.EngagementCap))
}

func affinityScore(cfg Config, user UserContext, authorID string) float64 {
	aff := math.Max(0, math.Min(1, user.AuthorAffinity[authorID]))
	if user.Follows[authorID] {
		aff = math.Min(1, aff+cfg.FollowBonus)
	}
	return aff
}

// topicScore returns the Jaccard overlap plus the first matching topic in
// sorted order (for the reasons list).
func topicScore(interests, topics []string) (float64, string) {
	if len(interests) == 0 || len(topics) == 0 {
		return 0, ""
	}
	interestSet := make(map[string]bool, len(interests))
	for _, s := range interests {
		if s != "" {
			interestSet[s] = true
		}
	}
	topicSet := make(map[string]bool, len(topics))
	for _, s := range topics {
		if s != "" {
			topicSet[s] = true
		}
	}
	if len(interestSet) == 0 || len(topicSet) == 0 {
		return 0, ""
	}

	sortedTopics := make([]string, 0, len(topicSet))
	for t := range topicSet {
		sortedTopics = append(sortedTopics, t)
	}
	sort.Strings(sortedTopics)

	inter := 0
	matched := ""
	for _, t := range sortedTopics {
		if interestSet[t] {
			inter++
			if matched == "" {
				matched = t
			}
		}
	}
	if inter == 0 {
		return 0, ""
	}
	union := len(interestSet) + len(topicSet) - inter
	return float64(inter) / float64(union), matched
}

func socialProofScore(cfg Config, engagedFollows int) float64 {
	if engagedFollows <= 0 {
		return 0
	}
	return math.Min(1, float64(engagedFollows)/float64(cfg.SocialProofSaturation))
}

func freshness(cfg Config, age time.Duration) float64 {
	if age < 0 {
		age = 0
	}
	return math.Max(cfg.FreshnessFloor, math.Exp(-age.Hours()/cfg.FreshnessTauHours))
}

func reasons(user UserContext, c Candidate, matchedTopic string, boosted, trending bool) []string {
	var out []string
	if c.MentionsViewer {
		out = append(out, "mentions_you")
	}
	if user.Follows[c.AuthorID] {
		out = append(out, "followed_author")
	}
	if matchedTopic != "" {
		out = append(out, "topic:"+matchedTopic)
	}
	if boosted {
		out = append(out, "more_like_this")
	}
	if trending {
		out = append(out, "trending_search")
	}
	if c.EngagedFollows > 0 {
		out = append(out, "liked_by_follows")
	}
	switch c.Source {
	case SourceTrending:
		out = append(out, "trending")
	case SourceExplore:
		out = append(out, "explore")
	}
	if len(out) == 0 {
		out = append(out, "popular")
	}
	return out
}

// orderRanked sorts by score desc, recency desc, then ID asc so ordering is
// total and deterministic.
func orderRanked(rs []Ranked) {
	sort.SliceStable(rs, func(i, j int) bool {
		if rs[i].Score != rs[j].Score {
			return rs[i].Score > rs[j].Score
		}
		if !rs[i].CreatedAt.Equal(rs[j].CreatedAt) {
			return rs[i].CreatedAt.After(rs[j].CreatedAt)
		}
		return rs[i].ID < rs[j].ID
	})
}

// selectPage fills the page greedily under the author cap and the in-network
// quota. A short page is padded from deferred in-network posts first; the
// author cap is relaxed only as a last resort.
func selectPage(cfg Config, user UserContext, scored []Ranked, pageSize int) []Ranked {
	maxInNet := int(math.Ceil(cfg.InNetworkTargetRatio * float64(pageSize)))
	authorCount := make(map[string]int)
	inNet := 0

	page := make([]Ranked, 0, pageSize)
	var quotaDeferred, authorDeferred []Ranked

	for _, r := range scored {
		if len(page) == pageSize {
			break
		}
		if authorCount[r.AuthorID] >= cfg.AuthorCapPerPage {
			authorDeferred = append(authorDeferred, r)
			continue
		}
		if isInNetwork(user, r.Candidate) && inNet >= maxInNet {
			quotaDeferred = append(quotaDeferred, r)
			continue
		}
		page = append(page, r)
		authorCount[r.AuthorID]++
		if isInNetwork(user, r.Candidate) {
			inNet++
		}
	}

	for _, r := range quotaDeferred {
		if len(page) == pageSize {
			break
		}
		if authorCount[r.AuthorID] >= cfg.AuthorCapPerPage {
			continue
		}
		page = append(page, r)
		authorCount[r.AuthorID]++
	}
	for _, r := range authorDeferred {
		if len(page) == pageSize {
			break
		}
		page = append(page, r)
	}
	return page
}

func isInNetwork(user UserContext, c Candidate) bool {
	return c.Source == SourceInNetwork || user.Follows[c.AuthorID]
}

// dominantTopic is the lexicographically smallest tag, "" when untagged.
func dominantTopic(p Post) string {
	minTag := ""
	for _, t := range p.Topics {
		if t != "" && (minTag == "" || t < minTag) {
			minTag = t
		}
	}
	return minTag
}

// breakTopicRuns pulls the next different-topic post forward whenever more
// than runCap consecutive posts share a dominant topic.
func breakTopicRuns(page []Ranked, runCap int) {
	for i := runCap; i < len(page); i++ {
		d := dominantTopic(page[i].Post)
		if d == "" {
			continue
		}
		run := true
		for k := 1; k <= runCap; k++ {
			if dominantTopic(page[i-k].Post) != d {
				run = false
				break
			}
		}
		if !run {
			continue
		}
		for j := i + 1; j < len(page); j++ {
			if dominantTopic(page[j].Post) != d {
				moved := page[j]
				copy(page[i+1:j+1], page[i:j])
				page[i] = moved
				break
			}
		}
	}
}

// injectExplore fills every Nth position with an unused exploration
// candidate. The RNG is seeded from (userID, hour bucket): stable within the
// hour, reproducible in tests.
func injectExplore(cfg Config, user UserContext, page []Ranked, scored []Ranked, now time.Time) {
	inPage := make(map[string]bool, len(page))
	authorCount := make(map[string]int, len(page))
	for _, r := range page {
		inPage[r.ID] = true
		authorCount[r.AuthorID]++
	}

	var pool []Ranked
	for _, r := range scored {
		if r.Source == SourceExplore && !inPage[r.ID] {
			pool = append(pool, r)
		}
	}
	if len(pool) == 0 {
		return
	}

	rng := rand.New(rand.NewPCG(hashString(user.UserID), uint64(now.Truncate(time.Hour).Unix())))

	for idx := cfg.ExploreSlotEvery - 1; idx < len(page); idx += cfg.ExploreSlotEvery {
		if page[idx].Source == SourceExplore {
			continue
		}
		pick := -1
		start := rng.IntN(len(pool))
		for off := 0; off < len(pool); off++ {
			i := (start + off) % len(pool)
			allowed := cfg.AuthorCapPerPage
			if pool[i].AuthorID == page[idx].AuthorID {
				allowed++ // the replaced slot frees one of this author's spots
			}
			if authorCount[pool[i].AuthorID] < allowed {
				pick = i
				break
			}
		}
		if pick == -1 {
			return
		}
		authorCount[page[idx].AuthorID]--
		page[idx] = pool[pick]
		authorCount[pool[pick].AuthorID]++
		pool = append(pool[:pick], pool[pick+1:]...)
		if len(pool) == 0 {
			return
		}
	}
}

func hashString(s string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	return h.Sum64()
}

func anyTopicIn(topics []string, set map[string]bool) bool {
	if len(set) == 0 {
		return false
	}
	for _, t := range topics {
		if set[t] {
			return true
		}
	}
	return false
}
