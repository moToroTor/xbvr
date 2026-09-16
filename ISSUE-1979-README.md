# Fix for xbapps/xbvr#1979 — `naughtyamericavr.com` URLs: "No scrapers exist for domain"

Origin issue: https://github.com/xbapps/xbvr/issues/1979

## What was wrong

Two gaps, one per layer:

1. **UI dispatch** (`ui/src/views/options/sections/OptionsSceneCreate.vue`):
   the single-scene dialog maps a pasted URL to a scraper by substring-matching
   each registered domain. The scraper registers `naughtyamerica.com`, which is
   not a substring of a `naughtyamericavr.com` URL (`…america` + `vr.com` ≠
   `…america` + `.com`), so no site matched and the dialog toasted
   "No scrapers exist for this domain".
2. **Collector allowlist** (`pkg/scrape/navr.go`): both collectors allowed only
   `www.naughtyamerica.com`. Even dispatched, a vr-host URL would have died with
   a silent `ErrForbiddenDomain` (colly exact host match).

## What this branch changes

- `OptionsSceneCreate.vue`: explicit `naughtyamericavr.com` → `naughtyamericavr`
  special case, alongside the existing per-site overrides. (`RescrapeButton.vue`
  needs no change: its loop matches scraper *id*, and `naughtyamericavr` is a
  substring of the vr host.)
- `navr.go`: both collectors also allow `naughtyamericavr.com` and
  `www.naughtyamericavr.com`. No host normalisation: both hosts serve
  byte-identical content (verified live 2026-09-15: `/vr-porn` → 302 + 29,810
  bytes on each), so either pasted form works as-is.

## How to test

1. `go build ./pkg/scrape/` — passes.
2. Paste a `naughtyamericavr.com` scene URL in Options → Scene Create: the
   "No scrapers exist" toast is gone and the `naughtyamericavr` scraper runs.
3. Regression: `naughtyamerica.com` pastes behave exactly as before.

## Verification notes (no maintained test)

- The repo has no UI test harness (`ui/tests` absent, no `test` script), so the
  one-line mapping cannot carry a maintained test.
- The Go change is declarative registration; verified by build plus the live
  byte-identical probe above (plus the colly exact-match analysis in #2254).
- The plan's secondary suspicion (drifted cast/cover selectors causing blank
  scenes) is explicitly out of scope and unverified — no live scene URL was
  available to check them against.
