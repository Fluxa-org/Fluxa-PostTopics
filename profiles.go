package feedrank

// Built-in feed profiles: one engine, several published weight sets
// (algorithmic choice). Instances add their own via ConfigFromJSON.

// DiscoverConfig is the out-of-network exploration feed: engagement- and
// recency-forward, low in-network quota, aggressive exploration.
func DiscoverConfig() Config {
	c := DefaultConfig()
	c.Version = "2026-07-21.discover"
	c.Weights = Weights{Engagement: 0.45, Affinity: 0.05, Topic: 0.25, SocialProof: 0.25}
	c.FreshnessTauHours = 4
	c.InNetworkTargetRatio = 0.3
	c.ExploreSlotEvery = 5
	return c
}

// QuietPostersConfig surfaces followed accounts that post rarely:
// affinity-forward, long freshness window, harsh prolific damp, no explore.
func QuietPostersConfig() Config {
	c := DefaultConfig()
	c.Version = "2026-07-21.quiet-posters"
	c.Weights = Weights{Engagement: 0.10, Affinity: 0.45, Topic: 0.20, SocialProof: 0.25}
	c.FreshnessTauHours = 24
	c.InNetworkTargetRatio = 1.0
	c.ExploreSlotEvery = 0
	c.ProlificDampThreshold = 2
	return c
}

// BuiltinProfiles returns a fresh copy of the named profiles.
func BuiltinProfiles() map[string]Config {
	return map[string]Config{
		"for-you":       DefaultConfig(),
		"discover":      DiscoverConfig(),
		"quiet-posters": QuietPostersConfig(),
	}
}
