# Fix for xbapps/xbvr#1696 — LethalHardcoreVR via the Gamma Algolia index

Origin issue: https://github.com/xbapps/xbvr/issues/1696

## What was wrong

The site moved to a client-rendered React app: page HTML carries no scene
data, so every selector in the old scraper matched nothing. The app reads a
shared Gamma Algolia index (`all_scenes`, keyed by `availableOnSite`) — the
same platform `upclosevr.go` already uses.

## What this branch changes

`pkg/scrape/lethalhardcorevr.go` rewritten (old selector code deleted):

- `LethalHardcoreSite` now takes a `siteHost`, reads the Algolia `apiKey` /
  `applicationID` off `/en/videos`, and queries `all_scenes` filtered by
  `availableOnSite:<scraperID>` — paginated listing plus single-scene lookup
  by `clip_id` (new canonical form `.../en/video/<site>/<slug>/<clip_id>`).
- Scene mapping (id, title, date, duration, synopsis w/ `movie_desc` fallback,
  gammacdn cover, cast w/ profiles, lowercased-deduped tags, trailer) mirrors
  the proven DarXV05/xbvr#20 implementation, adapted to this tree
  (`funk.ContainsString` known-scene check, both collector hosts allowed).
- `whorecraftvr` registration kept: the index returns no scenes for it, so it
  appears dead (noted in code).

## How to test

1. `go build ./pkg/scrape/` — passes.
2. Live single-scene run 2026-09-15 through the real function:
   `lethalhardcorevr-291210`, "Teen Lilibet Has Amazing Natural Tits",
   2026-10-02, 28 min, cast + 25 tags + cover — all present.
3. Index currently holds 345 lethalhardcorevr scenes (verified same day).

## Verification notes (no maintained test)

- The Algolia credentials are scraped live off the site and the index is a
  third-party service, so neither can be a repo fixture; a unit test would
  only re-test gjson paths. Verified instead by build plus the live
  end-to-end run above (probe removed after use).
