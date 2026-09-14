# Fix: colly AllowedDomains rejects www. URLs — single-scene scrapes die silently

Related: xbapps/xbvr#300 (found while verifying it).

## What was wrong

`createCollector()` (`pkg/scrape/scrape.go`) passes each scraper's bare domain
to `colly.AllowedDomains`, which is an EXACT host match
(`slices.Contains`, colly v2.3.0). A single-scene URL with a `www.` prefix —
the form browsers show and users paste — fails with `ErrForbiddenDomain`.
`sceneCollector.Request()` returns the error (it does not go through the
`OnError` handler) and every call site ignores it, so the scrape starts,
finishes instantly with 0 scenes, and logs nothing.

Proven live: `https://www.realitylovers.com/vd/160944479/Blowjob-Anniversary`
→ 0 scenes silent; same URL on the naked domain → 2 scenes.

## What this branch changes

`pkg/scrape/scrape.go` `createCollector()`: also allow the `www.` variant of
each registered domain (skipped when already prefixed). One place, all ~50
scrapers fixed. No behavior change for already-working URLs.

## How to test

1. `go build ./pkg/scrape/` — passes.
2. Fresh-DB single-scene scrape with a `www.` URL (any site) → scene indexes
   instead of a silent 0-scene run.
3. Regression: naked-domain single scrapes and full site scrapes unchanged.
