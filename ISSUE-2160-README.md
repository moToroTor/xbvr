# ISSUE-2160: FuckPassVR partial scrapes — Colly redirect dedup poisoned by /sfw/ geo-gate

Origin: https://github.com/xbapps/xbvr/issues/2160

## What

With rotating proxies, some `fuckpassvr-native` scene requests get a `302`
redirect to `/sfw/` (geo/age gate). Colly records every redirect target in
its shared visited set, so the first redirected scene marks `/sfw/` visited
and every later redirected scene fails with `"already visited"` and is
skipped (88/191 failed in the reporter's run). Only a restart-and-rerun
helped.

## Why (root cause)

- `pkg/scrape/scrape.go` `createCollector()` uses Colly's default visit
  dedup. Colly's redirect handler (`checkRedirectFunc`) marks each redirect
  target visited, so the transient `/sfw/` target poisons the set.
- `pkg/scrape/fuckpassvr.go` then hits `OnError` ("already visited") for
  subsequent scenes, and the `#sfwVideo` fallback never runs because the
  scene page is never parsed.

Verified against `colly v2.3.0` (`requestCheck` + `checkRedirectFunc` in
`colly.go`): both the originally-requested URL and every redirect target
are added to the visited store unless `AllowURLRevisit` is set.

## Fix

- `pkg/scrape/scrape.go`: new opt-in helper `allowURLRevisit(c)` that sets
  `AllowURLRevisit = true` on one collector. `createCollector()` defaults
  are unchanged, so the other ~50 scrapers are unaffected.
- `pkg/scrape/fuckpassvr.go`:
  - scene collector only: `allowURLRevisit(createCollector(...))`, so the
    shared `/sfw/` redirect target can no longer poison dedup;
  - `OnHTML(html)` guard: if the final URL path starts with `/sfw`, warn
    and return without emitting — the scene stays unknown and is retried
    on the next run (no immediate retry: same-proxy retry would likely hit
    the gate again);
  - manual `visitedScenes` map on canonical scene URLs replaces the lost
    Colly dedup for scenes within a run. The site (pagination) collector
    keeps default dedup.

## How to test (manual — needs live-proxy confirmation)

Full redirect reproduction likely needs the reporter's proxy setup
(rotating egress where some scene requests 302 to `/sfw/`); it cannot be
reproduced deterministically from a plain network.

1. Configure a proxy that triggers the gate (`Settings → Advanced →
   ScraperProxy`), or force `/sfw/` redirects for scene URLs.
2. Run `force-site-update` on the `fuckpassvr-native` scraper.
3. Check logs: zero `already visited` errors; gated scenes log
   `FuckPassVR: skipping /sfw/ geo-gate page ...` and no garbage scene is
   emitted for the gate page.
4. Run again consecutively without restarting the container: remaining
   scenes are processed (previously they kept failing until restart).

## Verification done here

- `curl` header check only (no media fetched): from this network even
  `https://www.fuckpassvr.com/destination` returns `302` with
  `location: https://www.fuckpassvr.com/sfw/`, confirming the geo-gate is
  real and `/sfw/` is the redirect target.
- `gofmt -l` on both touched files: clean.
- `go build ./pkg/scrape/`: OK.
- `go vet ./pkg/scrape/`: only pre-existing warnings in `javlibrary.go`
  / `vrporn.go`, unrelated to this change.
- `go test` was NOT run (broken repo-wide, pre-existing — per task rules).

## What needs live-proxy confirmation

- A multi-scene run where several scenes redirect to `/sfw/`: confirm no
  `already visited` cascade and that gated scenes are picked up on a later
  run once their egress IP passes the gate.
