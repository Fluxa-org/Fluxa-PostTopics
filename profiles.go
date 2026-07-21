package feedrank

// Built-in feed profiles: one engine, several published weight sets
// (algorithmic choice). Instances add their own via ConfigFromJSON.

// DiscoverConfig is the out-of-network exploration feed: engagement- and
// recency-forward, low in-network quota, aggressive exploration.
func DiscoverConfig() Config {
	c := DefaultConfig()
	c.Version = "v0.3.0.discover"
	c.Weights = Weights{Engagement: 0.40, Affinity: 0.05, Topic: 0.20, SocialProof: 0.20, Trending: 0.15}
	c.FreshnessTauHours = 4
	c.InNetworkTargetRatio = 0.3
	c.ExploreSlotEvery = 5
	return c
}

// QuietPostersConfig surfaces followed accounts that post rarely:
// affinity-forward, long freshness window, harsh prolific damp, no explore.
func QuietPostersConfig() Config {
	c := DefaultConfig()
	c.Version = "v0.3.0.quiet-posters"
	c.Weights = Weights{Engagement: 0.10, Affinity: 0.45, Topic: 0.20, SocialProof: 0.25}
	c.FreshnessTauHours = 24
	c.InNetworkTargetRatio = 1.0
	c.ExploreSlotEvery = 0
	c.ProlificDampThreshold = 2
	return c
}

// SmallCommunityConfig tunes the feed for young or low-volume instances
// (roughly < 20 posts/hour): a 30-day window, gentle 48 h freshness decay so
// the page isn't dominated by the last few hours, a stricter prolific damp
// (one hyperactive account cannot flood a small feed), and more exploration.
func SmallCommunityConfig() Config {
	c := DefaultConfig()
	c.Version = "v0.3.0.small-community"
	c.FreshnessTauHours = 48
	c.MaxAgeHours = 720
	c.ProlificDampThreshold = 5
	c.ExploreSlotEvery = 5
	return c
}

// BuiltinProfiles returns a fresh copy of the named profiles.
func BuiltinProfiles() map[string]Config {
	return map[string]Config{
		"for-you":         DefaultConfig(),
		"discover":        DiscoverConfig(),
		"quiet-posters":   QuietPostersConfig(),
		"small-community": SmallCommunityConfig(),
	}
}
