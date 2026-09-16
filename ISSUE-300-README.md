# ISSUE-300: RealityLovers scraper POV vs Voyeur

Origin: https://github.com/xbapps/xbvr/issues/300

## What

RealityLovers hosts distinct POV and Voyeur versions of the same scene
(example: Blowjob Anniversary w/ Claudia Macc). Both versions share one
`contentId`, but each has its own download URL with a unique `sceneId` and
`perspective`:

- `https://realitylovers.com/video/download?contentId=160944479&sceneId=160944512&...&perspective=POV&...`
- `https://realitylovers.com/video/download?contentId=160944479&sceneId=160944552&...&perspective=VOYEUR&...`

The scraper keyed `SceneID` only on the shared `contentId`
(`realitylovers-160944479`), collapsing both versions into a single XBVR
entry, so the second version could never be matched/downloaded.

## Why (root cause)

`pkg/scrape/realitylovers.go` built `SceneID` as
`slugify(Site) + "-" + contentId` with no perspective component, emitting
exactly one scene per scene page. Additionally, the scraper sent only the
`agreedToDisclaimer=true` cookie, but the live site now gates
listing/scene pages behind `isAgeVerified=true` too (without it both
return a ~9 KB age-verification wall with no scene data).

## Fix (`pkg/scrape/realitylovers.go` only)

1. Scene handler detects the on-page POV/VOYEUR picker
   (`input#POV` + `input#VOYEUR`, rendered only when both versions exist).
   Dual-version pages now emit two scenes:
   - `realitylovers-<contentId>-pov`, title suffixed ` [POV]`
   - `realitylovers-<contentId>-voyeur`, title suffixed ` [Voyeur]`
2. Single-version pages keep the historic `realitylovers-<contentId>`
   SceneID (no DB churn for existing matches).
3. Both collectors now send `Cookie: agreedToDisclaimer=true;
   isAgeVerified=true` so listing/scene pages render again.

Conservative notes: `HomepageURL`, covers, gallery, cast, tags and dates
are identical for both variants (per-version download `sceneId`s and file
names are member-only and were intentionally not invented). Applies to
both sites sharing the handler (`realitylovers`, `tsvirtuallovers`).

## How to test (incl. live URLs checked 2026-09-14, desktop UA, HTML only)

Selectors verified against live markup with goquery (same engine colly
uses) before/after the change:

- Listing `https://realitylovers.com/videos/page1`
  (`Cookie: agreedToDisclaimer=true; isAgeVerified=true` -> HTTP 200,
  ~239 KB): `div#gridView` (1x), `div.video-grid-view` (24x),
  `p.card-title` (24x), `a.page-link[aria-label="Next"]` (1x, not
  disabled) all present; scene links now `/vd/<id>/<slug>/`.
  Without `isAgeVerified=true` the same URL returns only the age wall
  (~9 KB, no `gridView`).
- Dual-version scene `https://realitylovers.com/vd/160944479/Blowjob-Anniversary`
  (HTTP 200, ~108 KB): `input#POV` (1x), `input#VOYEUR` (1x),
  `table.video-description-list` (cast/tags/date rows intact),
  `div.accordion-body` (1x), `div.owl-carousel div.item` present.
  Simulating the new logic yields `realitylovers-160944479-pov` /
  `realitylovers-160944479-voyeur`.
- Single-version scene `https://realitylovers.com/vd/184589813/The-Enchanted-Fae/`
  (HTTP 200): `input#POV`/`input#VOYEUR` (0x) -> keeps legacy
  `realitylovers-184589813`.
- Without the cookie, the scene URL above returns only the age wall
  (~9.6 KB, no `video-description-list`).

Build/verify:

- `gofmt -l pkg/scrape/realitylovers.go` (clean)
- `go build ./pkg/scrape/`
- `go test` intentionally NOT run (broken repo-wide, pre-existing).
- After scraping the Blowjob Anniversary page, XBVR should hold two
  scenes (`realitylovers-160944479-pov`, `realitylovers-160944479-voyeur`).

## Offline proof (2026-09-14)
Real scene page saved by reporter (`rl-160944479.html`, Blowjob Anniversary):
branch's exact selectors (`input#POV`, `input#VOYEUR`) yield both perspectives
via goquery (colly's engine) → dual emit fires; single-version pages keep the
legacy ID. Fixture not committed (third-party HTML); check above is reproducible
with any saved scene page.

## Gate status (2026-09-14, final)
Reporter-confirmed: the age wall keys off IP reputation and stands against any
dumb HTTP client from flagged egress (all VPN/datacenter exits tried, cookies
and browser headers included) — only real browsers pass. The cookie lines in
this branch match the site's own gate JS and are harmless, but treat them as
best-effort: the provable fix here is the dual emit, verified above.
