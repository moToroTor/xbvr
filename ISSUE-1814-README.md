# ISSUE-1814: VRSpy Not Scraping Correctly

Origin: https://github.com/xbapps/xbvr/issues/1814

## What

After a vrspy.com redesign, scene scrapes returned nav junk as the title
(`<title> Photos Related videos Discussion` concatenated) and missed
release date, duration, actors and tags (reported on v0.4.28, e.g. scene
"Absolute Taboo: Bicycle Ride").

## Why

The scene-page selectors in `pkg/scrape/vrspy.go` targeted the old DOM.
In the current DOM the section headings below the player (Photos,
Related videos, Discussion) are `h2.section-header-container`, sharing
the class with the scene `h1`, so any unscoped title selector regresses
into the reported junk title. Date/duration come from
`.video-details-info-item` rows that have shifted shape before
(`Release date: <span>19 July 2024</span>`, `Duration:
<span>00:59:05</span>`), with no secondary source when they miss.

## Fix (`pkg/scrape/vrspy.go` only)

1. Title primary selector scoped to the scene header:
   `div.video-title-container h1.section-header-container`, keeping
   `h1.section-header-container` as fallback. Verified: exactly 1 match
   with clean text on all scenes checked; the h2 section headings can no
   longer leak in.
2. JSON-LD `VideoObject` fallback for release date (`uploadDate`,
   RFC3339 -> `YYYY-MM-DD`) and duration (ISO 8601 `PT59M5S`/`PT1H10M9S`
   -> minutes, same truncation formula as the primary parser). Runs only
   when the primary detail-item extraction misses, so live behavior is
   unchanged but resilient to the next redesign.

Conservative notes: intervening fixes (#1883 titles, #2026, #2137
gallery/pagination) already re-mapped most selectors; everything else
was verified working against the live DOM (see below) and left
untouched, including the `age=true` cookie (still accepted, HTTP 200
with full content) and listing/pagination selectors.

## How to test (incl. live URLs checked 2026-09-14, desktop UA, HTML only)

Every selector verified against live markup with goquery (same engine
colly uses) before/after the change:

- Scene `https://www.vrspy.com/video/absolute-taboo-bicycle-ride`
  (HTTP 200, ~464 KB): title 1x `Absolute Taboo: Bicycle Ride`;
  `.show-more-text p` 7x (synopsis); `.video-categories` 1 block, 31
  tags; `.video-actor-item` 1x `Lucky Anne` (`a[href=/star/lucky-anne]`);
  6x `.video-details-info-item` incl. `Release date: 19 July 2024`,
  `Duration: 00:59:05`; SiteID `58` (first `cdn.vrspy.com/videos/<id>/`
  ref == `og:image` ID); cover/gallery-direct (41x)/trailer (7x) regexes
  match; JSON-LD fallback computes `2024-07-19` / 59 min (== primary).
- Scene `https://www.vrspy.com/video/playing-dirty` (HTTP 200):
  title `Playing Dirty`, cast `Luna Angel`, `Release date: 04 September
  2026`, SiteID `209`; fallback computes `2026-09-04` / 59 min.
- Scene `https://www.vrspy.com/video/gracey-snow-on-demand` (HTTP 200):
  title `Gracey Snow On Demand`, cast `Gracey Snow`,
  `Release date: 13 September 2026`, `Duration: 01:10:09`, SiteID `198`;
  fallback computes `2026-09-13` / 70 min.
- Listings `https://www.vrspy.com/`, `https://www.vrspy.com/videos`,
  `https://www.vrspy.com/videos?sort=new` (HTTP 200 each):
  `.item-wrapper .photo a` 28x/24x/24x clean `/video/<slug>` hrefs
  (no fragments); pagination path (`/videos?sort=new&page=N`)
  consistent with the `body`-handler logic.

Build/verify:

- `gofmt -l pkg/scrape/vrspy.go` (clean)
- `go build ./pkg/scrape/`
- `go test` intentionally NOT run (broken repo-wide, pre-existing).
- Single-scene scrape of the Bicycle Ride URL should yield title
  `Absolute Taboo: Bicycle Ride` (no `Photos`/`Discussion` suffix),
  released `2024-07-19`, duration `59`, cast `Lucky Anne`, 31 tags.
