# Issue #1867 — SexBabesVR scraper overwrites scenes instead of adding them

Origin: https://github.com/xbapps/xbvr/issues/1867
Branch: `fix/1867-sexbabesvr-ids` (from `origin/master`)

## What

Rescraping SexBabesVR reported hundreds of new scenes, yet only one scene row
existed afterwards — every single-scene scrape replaced the previous one.

## Why

`SiteID` was parsed from the `dl8-video` poster URL (`tmp[len(tmp)-2]` in
`pkg/scrape/sexbabesvr.go`). That segment is not a stable per-scene
identifier across poster-URL variants, and when the element/attribute is
absent `SiteID` stays empty — so every scene upserts under the same
`SceneID` (`sexbabesvr-<id>`) and overwrites a single row.

## Fix

- `SiteID` is now derived from the canonical scene URL slug
  (`https://sexbabesvr.com/video/<slug>/`) via the new pure helper
  `sexBabesVRSceneIDFromURL()`, which is always present and unique per scene.
- The poster-URL parse is kept only as a fallback for the (unexpected) case
  where the URL slug is empty.
- Added `pkg/scrape/sexbabesvr_test.go` with
  `TestSexBabesVRSceneIDFromURL`: slug/trailing-slash/query cases plus an
  assertion that two distinct scene URLs yield distinct `SceneID`s.

Note: `SceneID`s change from the old poster-derived form to the slug form,
so one stale duplicate row may remain in existing DBs — delete the old
`sexbabesvr-*` row once, then rescrape.

## How to test

1. `go build ./pkg/scrape/` (passes; `go test` not run — broken repo-wide,
   pre-existing).
2. Scrape 2 SexBabesVR single scenes, then
   `sqlite3 xbvr.db "select scene_id,homepage_url from scenes
   where scraper_id='sexbabesvr';"` → 2 distinct rows.

## Live verification (2026-09-14, curl, desktop User-Agent, markup only)

- Fetched 3 live scene pages; canonical URLs:
  `/video/wet-college-student-remastered/`,
  `/video/best-international-fuck-moments/`,
  `/video/hard-nipples-lilly-bella-helen-star/` → helper yields 3 distinct
  slugs/IDs.
- Poster `tmp[len-2]` happened to be distinct on these 3 pages
  (777/776/775), confirming the collision comes from poster-URL variants /
  missing elements rather than these samples — which is exactly why the
  always-present URL slug is the robust identifier. Fetched HTML was not
  committed.
