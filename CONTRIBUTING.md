# Contributing

Thanks for your interest in FluxRank. A few ground rules keep the algorithm
trustworthy:

## Principles (non-negotiable)

1. **Determinism** — no `time.Now()`, no unseeded randomness, no I/O inside
   the package. Callers pass `now`; the only RNG is the seeded exploration
   slot. Identical inputs must always produce identical output.
2. **Standard library only** — no third-party dependencies, ever.
3. **Weights live in config, not code** — behavior changes ship as a new
   versioned weight set, and `Validate` must reject nonsensical values.
4. **Explainable** — any new signal must surface in `Breakdown` and/or
   `Reasons`.
5. **No surveillance signals** — dwell time, profile-visit tracking, DM data,
   and follower counts are rejected by design, not by omission.
6. Keep comments minimal: document non-obvious constraints, put prose in
   `ALGORITHM.md`.

## Development

```sh
go test -race -count=1 -cover ./...          # full suite (98%+ expected)
go test -run='^$' -fuzz=FuzzRank -fuzztime=30s .
go test -run='^$' -bench=. -benchmem .
gofmt -l . && go vet ./...
```

CI runs all of the above on every push and pull request; it must stay green.

## Pull requests

- New behavior needs tests, including an invariant or property test when the
  change touches `Rank`'s pipeline.
- Weight/default changes need a short written rationale in the PR description
  (what user-visible outcome improves, and why the trade-off is right) and a
  `CHANGELOG.md` entry.
- Update `ALGORITHM.md` whenever the formula, modifiers, or page rules change.
