# Fluxa-PostTopics (FluxRank)

[![ci](https://github.com/Fluxa-org/Fluxa-PostTopics/actions/workflows/ci.yml/badge.svg)](https://github.com/Fluxa-org/Fluxa-PostTopics/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/Fluxa-org/Fluxa-PostTopics.svg)](https://pkg.go.dev/github.com/Fluxa-org/Fluxa-PostTopics)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

**FluxRank** is the open-source feed-ranking algorithm behind
[Fluxa](https://github.com/Fluxa-org)'s "For You" timeline — a transparent,
deterministic alternative to black-box recommendation systems.

日本語: Fluxaの「おすすめ」フィードを並べているアルゴリズム本体です。機械学習でも
ブラックボックスでもなく、**誰でも読める1つの数式と公開された重み**で動きます。同じ
入力からは常に同じフィードが再現され、全ての投稿に「なぜ表示されたか」の内訳が付きます。

## Why open

- **Transparent** — one scrutable formula; every weight lives in a versioned
  config, not in code.
- **Deterministic** — pure functions, no I/O, no clock, seeded randomness
  only. Identical inputs always produce identical feeds, so the algorithm is
  auditable and golden-testable.
- **Explainable** — every ranked post carries `Reasons`
  (`followed_author`, `topic:music`, `mentions_you`, …) and a full per-term
  score `Breakdown`, enough to render a "why am I seeing this?" panel.
- **Privacy-respecting** — no dwell-time tracking, no DM signals, no
  follower-count worship. Earned reach comes only through log-damped
  engagement.

See [ALGORITHM.md](ALGORITHM.md) for the full specification.

## Install

```sh
go get github.com/Fluxa-org/Fluxa-PostTopics
```

Pure Go, standard library only, `go >= 1.22`.

## Quick start

```go
import feedrank "github.com/Fluxa-org/Fluxa-PostTopics"

// Map hashtags (EN/JA aliases included) onto your topic taxonomy.
topics := feedrank.PostTopics("新曲できた！ #音楽", nil, canonical, feedrank.DefaultAliases(), 3)

user := feedrank.UserContext{
    UserID:    "viewer",
    Interests: []string{"music"},
    Follows:   map[string]bool{"alice": true},
}
candidates := []feedrank.Candidate{{
    Post: feedrank.Post{
        ID: "post-1", AuthorID: "alice", CreatedAt: createdAt,
        Likes: 12, Replies: 3, Topics: topics,
    },
    Source: feedrank.SourceInNetwork,
}}

page := feedrank.Rank(feedrank.DefaultConfig(), user, candidates, time.Now(), 20)
for _, r := range page {
    fmt.Println(r.ID, r.Score, r.Reasons) // e.g. post-1 0.32 [followed_author topic:music]
}
```

The caller owns all I/O: gather candidates from your stores, map them into
`Candidate`s, and render the returned page. `Rank` never touches a database.

## The formula

```
score = freshness × (0.35·engagement + 0.30·affinity + 0.20·topic + 0.15·social_proof + 0.10·search_trend) × modifiers
```

| Term | Meaning |
|---|---|
| engagement | `log10(1 + likes + 2·reposts + 2.5·bookmarks + 4·replies + views/100)`, saturating — conversation beats passive likes, virality is damped |
| affinity | your history with the author (+ follow bonus) |
| topic | Jaccard overlap between your interests and the post's topics |
| social_proof | how many accounts you follow engaged with it |
| search_trend | what the whole network is searching right now (aggregate topic heat via `TrendingTopics`); your own recent searches join the topic term via `SearchInterests` |
| freshness | `exp(-age/8h)` decay |

Modifiers: seen ×0.1 · not-interested topic ×0.2 · stranger-reply ×0.3 ·
moderation labels (stackable) · prolific-author damp `sqrt(threshold/count)` ·
"show more like this" ×1.5 · mentions-you ×2 · language mismatch ×0.5.

Page rules: max 2 posts per author, ~50 % in-network quota, no 3-in-a-row
same topic, and every 10th slot reserved for exploration (seeded, stable
within the hour) so new authors get discovered.

## Feed profiles

One engine, several published weight sets — algorithmic choice in the spirit
of Bluesky's feed marketplace:

| Profile | Character |
|---|---|
| `for-you` | balanced default |
| `discover` | engagement/recency-forward, aggressive exploration |
| `quiet-posters` | surfaces followed accounts that post rarely |

```go
cfg := feedrank.BuiltinProfiles()["discover"]
cfg, err := feedrank.ConfigFromJSON(raw) // or define your own (unknown fields rejected)
```

## Topics, hashtags, mentions, and search — i18n included

Ranking never consumes raw hashtags. `PostTopics` maps them onto a canonical
taxonomy — ~350 built-in aliases across ten language groups (en, ja, ko, zh,
es, hi, vi, fr, de, pt: `#音楽`→`music`, `#맛집`→`food`, `#fútbol`→`sports`)
— and unmapped tags contribute nothing, so tag spam cannot game the feed.
Extraction is Unicode-aware, so hashtags in any language extract.

- `ExtractMentions` pulls `@handles` (Unicode) so callers can set the
  mentions-you boost.
- `MapQuery` turns free-text search queries into topics: feed a user's own
  searches back as `SearchInterests`, and aggregate everyone's searches into
  `TrendingTopics` so the feed reflects what the network is looking for
  (aggregate counts only — individual queries never reach the ranker).
- Post `Language` + viewer `Languages` down-rank posts the viewer can't read;
  unknown languages are never penalized.

## Testing

~35 table-driven tests, randomized invariant tests, fuzzing, and benchmarks;
98 % coverage, race-clean. Ranking 600 candidates into a 50-post page takes
**~0.4 ms** on an M1 Pro — your database is the bottleneck, not this.

```sh
go test -race -cover ./...
go test -run='^$' -fuzz=FuzzRank -fuzztime=30s .
go test -run='^$' -bench=. -benchmem .
```

## What is deliberately absent

Dwell time, profile-visit tracking, DM signals, follower counts, and ML
prediction. The pipeline is shaped so a learned scorer could replace the
formula later without touching sourcing, filters, or page rules — but v1
stays fully auditable.

## License

[MIT](LICENSE)
