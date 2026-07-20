# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses
[Semantic Versioning](https://semver.org/).

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
