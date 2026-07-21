package feedrank

import (
	"bytes"
	"encoding/json"
	"fmt"
)

type Weights struct {
	Engagement  float64 `json:"engagement"`
	Affinity    float64 `json:"affinity"`
	Topic       float64 `json:"topic"`
	SocialProof float64 `json:"social_proof"`
	// Trending weights the network-wide search-heat term (UserContext.TrendingTopics).
	Trending float64 `json:"trending"`
}

type EngagementWeights struct {
	Like     float64 `json:"like"`
	Repost   float64 `json:"repost"`
	Bookmark float64 `json:"bookmark"`
	Reply    float64 `json:"reply"`
	View     float64 `json:"view"`
}

// Penalties are multiplicative down-ranks in [0,1].
type Penalties struct {
	Seen               float64 `json:"seen"`
	NotInterestedTopic float64 `json:"not_interested_topic"`
	ReplyToNonFollowed float64 `json:"reply_to_non_followed"`
}

// Config holds every tunable of the algorithm. Load with ConfigFromJSON and
// log Version with each ranking pass.
type Config struct {
	Version string `json:"version"`

	Weights    Weights           `json:"weights"`
	Engagement EngagementWeights `json:"engagement_weights"`
	Penalties  Penalties         `json:"penalties"`

	EngagementCap     float64 `json:"engagement_cap"`
	FreshnessTauHours float64 `json:"freshness_tau_hours"`
	FreshnessFloor    float64 `json:"freshness_floor"`
	MaxAgeHours       float64 `json:"max_age_hours"`

	FollowBonus           float64 `json:"follow_bonus"`
	SocialProofSaturation int     `json:"social_proof_saturation"`

	AuthorCapPerPage     int     `json:"author_cap_per_page"`
	InNetworkTargetRatio float64 `json:"in_network_target_ratio"`
	TopicRunCap          int     `json:"topic_run_cap"`      // 0 disables
	ExploreSlotEvery     int     `json:"explore_slot_every"` // 0 disables

	MoreLikeThisBoost     float64            `json:"more_like_this_boost"` // >= 1
	MentionBoost          float64            `json:"mention_boost"`        // >= 1
	ProlificDampThreshold int                `json:"prolific_damp_threshold"`
	LabelPenalties        map[string]float64 `json:"label_penalties"`
	// LanguageMismatchPenalty multiplies posts in a language the viewer does
	// not read, in [0,1]; 1 disables language matching.
	LanguageMismatchPenalty float64 `json:"language_mismatch_penalty"`
	// RelaxMaxAgeWhenSparse readmits age-expired posts when fresh candidates
	// cannot fill a page, so small communities never see an empty feed.
	RelaxMaxAgeWhenSparse bool `json:"relax_max_age_when_sparse"`
}

func DefaultConfig() Config {
	return Config{
		Version: "v0.3.0.default",
		Weights: Weights{
			Engagement:  0.35,
			Affinity:    0.30,
			Topic:       0.20,
			SocialProof: 0.15,
			Trending:    0.10,
		},
		Engagement: EngagementWeights{
			Like:     1,
			Repost:   2,
			Bookmark: 2.5,
			Reply:    4,
			View:     0.01,
		},
		Penalties: Penalties{
			Seen:               0.1,
			NotInterestedTopic: 0.2,
			ReplyToNonFollowed: 0.3,
		},
		EngagementCap:           10_000,
		FreshnessTauHours:       8,
		FreshnessFloor:          0.05,
		MaxAgeHours:             168,
		FollowBonus:             0.3,
		SocialProofSaturation:   5,
		AuthorCapPerPage:        2,
		InNetworkTargetRatio:    0.5,
		TopicRunCap:             2,
		ExploreSlotEvery:        10,
		MoreLikeThisBoost:       1.5,
		MentionBoost:            2,
		ProlificDampThreshold:   10,
		LabelPenalties:          map[string]float64{},
		LanguageMismatchPenalty: 0.5,
		RelaxMaxAgeWhenSparse:   true,
	}
}

// ConfigFromJSON overlays a JSON document onto DefaultConfig, rejecting
// unknown fields so typos fail loudly.
func ConfigFromJSON(data []byte) (Config, error) {
	cfg := DefaultConfig()
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("feedrank: failed to parse config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	switch {
	case c.Weights.Engagement < 0 || c.Weights.Affinity < 0 || c.Weights.Topic < 0 ||
		c.Weights.SocialProof < 0 || c.Weights.Trending < 0:
		return fmt.Errorf("feedrank: term weights must be >= 0")
	case c.Weights.Engagement+c.Weights.Affinity+c.Weights.Topic+
		c.Weights.SocialProof+c.Weights.Trending <= 0:
		return fmt.Errorf("feedrank: at least one term weight must be > 0")
	case c.EngagementCap <= 0:
		return fmt.Errorf("feedrank: engagement_cap must be > 0")
	case c.FreshnessTauHours <= 0:
		return fmt.Errorf("feedrank: freshness_tau_hours must be > 0")
	case c.FreshnessFloor < 0 || c.FreshnessFloor > 1:
		return fmt.Errorf("feedrank: freshness_floor must be in [0,1]")
	case c.MaxAgeHours <= 0:
		return fmt.Errorf("feedrank: max_age_hours must be > 0")
	case c.FollowBonus < 0 || c.FollowBonus > 1:
		return fmt.Errorf("feedrank: follow_bonus must be in [0,1]")
	case c.SocialProofSaturation <= 0:
		return fmt.Errorf("feedrank: social_proof_saturation must be > 0")
	case c.AuthorCapPerPage < 1:
		return fmt.Errorf("feedrank: author_cap_per_page must be >= 1")
	case c.InNetworkTargetRatio < 0 || c.InNetworkTargetRatio > 1:
		return fmt.Errorf("feedrank: in_network_target_ratio must be in [0,1]")
	case c.TopicRunCap < 0 || c.TopicRunCap == 1:
		return fmt.Errorf("feedrank: topic_run_cap must be 0 (disabled) or >= 2")
	case c.ExploreSlotEvery < 0 || c.ExploreSlotEvery == 1:
		return fmt.Errorf("feedrank: explore_slot_every must be 0 (disabled) or >= 2")
	case c.MoreLikeThisBoost < 1:
		return fmt.Errorf("feedrank: more_like_this_boost must be >= 1")
	case c.MentionBoost < 1:
		return fmt.Errorf("feedrank: mention_boost must be >= 1")
	case c.ProlificDampThreshold < 0:
		return fmt.Errorf("feedrank: prolific_damp_threshold must be >= 0")
	case c.LanguageMismatchPenalty < 0 || c.LanguageMismatchPenalty > 1:
		return fmt.Errorf("feedrank: language_mismatch_penalty must be in [0,1]")
	}
	for label, m := range c.LabelPenalties {
		if m < 0 || m > 1 {
			return fmt.Errorf("feedrank: label penalty %q must be in [0,1]", label)
		}
	}
	for name, v := range map[string]float64{
		"seen":                  c.Penalties.Seen,
		"not_interested_topic":  c.Penalties.NotInterestedTopic,
		"reply_to_non_followed": c.Penalties.ReplyToNonFollowed,
	} {
		if v < 0 || v > 1 {
			return fmt.Errorf("feedrank: penalty %s must be in [0,1]", name)
		}
	}
	return nil
}
