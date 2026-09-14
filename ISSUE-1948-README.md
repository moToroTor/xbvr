# ISSUE-1948 README

Origin: https://github.com/xbapps/xbvr/issues/1948

## What
Running scrapers with SwallowBay enabled (or SwallowBay alone) crashed the
whole app; deselecting it let the run complete. Another user confirmed.

## Why
`SwallowBay` in `pkg/scrape/swallowbay.go` did
`regexpSceneID.FindStringSubmatch(e.Request.URL.Path)[1]` unchecked. Any URL
not matching `\-(\d+)\.html$` (redirect, listing page, www-vs-apex host page)
yields a nil/short slice, so indexing `[1]` panics — and since scrapers share
the process, one bad URL kills the app.

## Fix
`pkg/scrape/swallowbay.go` only (minimal diff, 4 lines):
- Capture the submatch slice and return early (skip + no emit) when
  `len(matches) < 2` instead of indexing `[1]` blindly.
- This composes with the existing `if sc.SiteID != ""` guard before `out <- sc`,
  so non-scene pages are skipped gracefully instead of crashing.

## How to test
1. `gofmt -l pkg/scrape/swallowbay.go` (expect no output).
2. `go build ./pkg/scrape/` (expect success).
3. UI: run the SwallowBay scraper alone, expect completion without crash
   (non-matching URLs skipped).
4. UI: run all scrapers with SwallowBay enabled, expect completion.
