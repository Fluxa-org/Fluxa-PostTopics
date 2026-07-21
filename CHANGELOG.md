# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses
[Semantic Versioning](https://semver.org/).

## [0.3.0] - 2026-07-21

### Added
- Sparse-content handling: `relax_max_age_when_sparse` (default on) readmits
  age-expired posts when fresh candidates cannot fill a page — small or young
  communities never see an empty feed; fresh posts still rank first.
- `small-community` builtin profile for low-volume instances: 30-day window,
  48 h freshness decay, prolific damp threshold 5, explore every 5th slot.

[0.3.0]: https://github.com/Fluxa-org/Fluxa-PostTopics/releases/tag/v0.3.0

## [0.2.0] - 2026-07-21

### Added
- Search-trend term: `UserContext.TrendingTopics` (network-wide search heat
  per topic, weight 0.10 default / 0.15 in `discover`) with the
  `trending_search` reason, plus `UserContext.SearchInterests` — topics from
  the viewer's own searches join the topic term.
- `MapQuery`: maps free-text search queries onto canonical topics.
- `ExtractMentions`: Unicode `@handle` extraction for the mention boost.
- Language matching: `Post.Language` + `UserContext.Languages` +
  `language_mismatch_penalty` (default ×0.5; unknown language never penalized).
- Multilingual hashtag aliases: ko, zh, es, hi, vi, fr, de, pt added to the
  existing en/ja table (~350 aliases total), restructured per language.

### Changed
- Config versions now follow the release (`v0.2.0.default`, …).
- `Breakdown` gains a `trending` field; `Weights` gains `trending`.

[0.2.0]: https://github.com/Fluxa-org/Fluxa-PostTopics/releases/tag/v0.2.0

## [0.1.0] - 2026-07-21

Initial public release, extracted from the Fluxa monorepo.

### Added
- `Rank`: the full ranking pipeline — dedupe, hard filters, scoring, page
  selection (author cap + in-network quota), topic-run breaking, seeded
  exploration slots.
- Scoring formula: log-damped engagement, exponential freshness, author
  affinity, topic Jaccard, social proof; multiplicative modifiers (seen,
  not-interested, stranger-reply, moderation labels, prolific-author damp,
  more-like-this ×1.5, mentions-you ×2).
- `Config` with JSON overlay (`ConfigFromJSON`, unknown fields rejected) and
  full validation; versioned weight sets.
- Feed profiles: `for-you`, `discover`, `quiet-posters` via `BuiltinProfiles`.
- Topic mapping: `ExtractHashtags` / `MapTopics` / `PostTopics` with ~180
  built-in English/Japanese aliases (`DefaultAliases`).
- Explainability: per-post `Reasons` and full score `Breakdown`.
- Test suite: table-driven + randomized invariants + fuzzing + benchmarks
  (98 % coverage, race-clean; ~0.4 ms to rank 600 candidates on M1 Pro).

[0.1.0]: https://github.com/Fluxa-org/Fluxa-PostTopics/releases/tag/v0.1.0
