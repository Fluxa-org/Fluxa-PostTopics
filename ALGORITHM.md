# FluxRank

A transparent, deterministic feed-ranking algorithm. No ML, no surveillance
signals, no black box: one scrutable formula, every weight in a versioned
config, and an explainable breakdown attached to every ranked post.

This package is pure Go (standard library only): no I/O, no clock (callers
pass `now`), and the only randomness is a seeded exploration slot. Identical
inputs always produce identical feeds, which is what makes the algorithm
auditable and golden-testable.

## Pipeline

```
candidates → dedupe → hard filters → score → order
           → page selection (author cap + in-network quota)
           → topic-run break → seeded exploration slots
```

**Candidate sources** (tagged by the caller): `in_network` (follows),
`topic` (interest match), `trending`, `social_proof` (engaged by follows),
`explore` (authors the viewer has never seen).

**Hard filters**: the viewer's own posts, "not interested" authors, posts
older than `max_age_hours`. Blocking/muting/visibility are the caller's job —
they are policy, not ranking.

**Sparse content**: when fresh candidates cannot fill a page,
`relax_max_age_when_sparse` (default on) readmits age-expired posts — they
score at the freshness floor, so fresh posts still lead, but a small or young
community never sees an empty feed. With very low volume the algorithm
degrades gracefully toward a recency feed: engagement terms go to zero and
freshness plus interests dominate, which is the correct behavior for a
network that is still growing.

## Scoring

```
score = freshness × (We·engagement + Wa·affinity + Wt·topic + Ws·social_proof + Wq·search_trend) × modifiers
```

| Term | Definition | Default weight |
|---|---|---|
| engagement | `log10(1 + likes + 2·reposts + 2.5·bookmarks + 4·replies + views/100)`, normalized to saturate at `engagement_cap` | 0.35 |
| affinity | viewer's decayed engagement history with the author (+`follow_bonus` if followed) | 0.30 |
| topic | Jaccard overlap between viewer interests ∪ `SearchInterests` (topics from the viewer's own searches) and post topics | 0.20 |
| social_proof | engaged followed accounts / `social_proof_saturation` | 0.15 |
| search_trend | best `TrendingTopics` heat among the post's topics — network-wide search interest, aggregated by the caller from all users' queries (see `MapQuery`) | 0.10 |
| freshness | `exp(-age / tau)`, floored at `freshness_floor` | multiplier |

Replies weigh most (conversation over passive likes — the same conclusion X
and Bluesky reached), bookmarks signal quality without performativity, and
log damping stops viral posts from drowning small accounts.

**Modifiers** (multiplicative):

| Modifier | Default | Trigger |
|---|---|---|
| seen | ×0.1 | viewer already had an impression |
| not-interested topic | ×0.2 | explicit "show less like this" |
| reply to non-followed | ×0.3 | keeps stranger conversations out |
| moderation labels | configured per label, stack | `label_penalties`, e.g. `{"sensitive":0.5,"spam":0}` |
| prolific-author damp | `sqrt(threshold/count)` above `prolific_damp_threshold`/24h | keeps quiet posters visible |
| more-like-this | ×1.5 | explicit "show more like this" |
| mentions the viewer | ×2 | `Candidate.MentionsViewer` (detect with `ExtractMentions`) |
| language mismatch | ×0.5 | post `Language` set, viewer `Languages` set, and no overlap; unknown language is never penalized |

## Page rules

- At most `author_cap_per_page` posts per author (relaxed only over a short page).
- In-network posts capped at `in_network_target_ratio` of the page while
  out-of-network supply lasts.
- No more than `topic_run_cap` consecutive posts sharing a dominant topic.
- Every `explore_slot_every`-th position is an exploration slot, chosen by an
  RNG seeded with `hash(userID, hour)` — stable within the hour, reproducible
  in tests, and the escape hatch from filter bubbles and author cold start.

## Topics, not hashtags

Ranking never consumes raw hashtags. `PostTopics` maps hashtags onto a
canonical taxonomy and falls back to the author's profile interests. Unmapped
tags contribute nothing, so tag spam cannot game the topic term.
`DefaultAliases` ships ~350 aliases across ten language groups (en, ja, ko,
zh, es, hi, vi, fr, de, pt); extraction itself is Unicode-aware so any
language's hashtags extract. `MapQuery` applies the same mapping to free-text
search queries, and `ExtractMentions` pulls `@handles` (Unicode) for the
mention boost.

## Search as a signal

Two search-derived inputs, both computed by the caller:

- **Personal**: map the viewer's own recent search queries with `MapQuery`
  and pass them as `SearchInterests` — they join profile interests in the
  topic term. What you search for shapes your feed.
- **Network-wide**: aggregate *all* users' queries (e.g. count per topic over
  a sliding window, normalize to [0,1], decay) into `TrendingTopics` — posts
  about what everyone is searching right now get the `search_trend` term and
  the `trending_search` reason. Only aggregate counts are needed; individual
  queries never reach the ranker.

## Feed profiles (algorithmic choice)

One engine, several published weight sets — the OSS analog of Bluesky's feed
marketplace. `BuiltinProfiles()` ships:

| Profile | Character |
|---|---|
| `for-you` | the default balance above |
| `discover` | engagement/recency-forward: tau 4 h, 30 % in-network, search-trend 0.15, explore every 5th slot |
| `quiet-posters` | affinity-forward: tau 24 h, prolific damp threshold 2, no explore |
| `small-community` | for low-volume instances (< ~20 posts/h): 30-day window, tau 48 h, prolific damp threshold 5, explore every 5th slot |

Instances define their own with `ConfigFromJSON` (overlays the defaults,
rejects unknown fields, validates ranges). Log `Config.Version` with every
ranking pass for auditability.

## Explainability

Every `Ranked` post carries `Reasons` (`followed_author`, `topic:music`,
`liked_by_follows`, `mentions_you`, `more_like_this`, `trending_search`,
`trending`, `explore`, `popular`) and a `Breakdown` with each term, the freshness factor, the
combined modifier, and the final score — enough to render a "why am I seeing
this?" panel without extra queries.

## What is deliberately absent

- **Dwell time, profile visits, DM signals** — privacy by construction.
- **Follower counts / author fame** — earned reach comes only through the
  log-damped engagement term.
- **ML prediction** — needs behavior data at scale and breaks auditability;
  the pipeline is shaped so a learned scorer could replace `score()` later
  without touching sourcing, filters, or page rules.
