# Fix for xbapps/xbvr#398 — batch JAV code scraping

Origin issue: https://github.com/xbapps/xbvr/issues/398

## What was wrong

Reporters with hundreds of JAV codes had to enter them one at a time: the
only intake was single-query `POST /api/task/scrape-javr`, and rapid manual
repeats risk IP-based blocking.

## What this branch changes

- `pkg/tasks/content.go`: `ParseJavCodes` (exported pure helper: lines,
  commas or whitespace separated; drops empties and dupes, keeps order) plus
  `ScrapeJAVRBatch`, which scrapes one scene per code sequentially with a
  10 s delay between requests. Each `ScrapeJAVR` call takes/releases the
  scrape lock itself, so sequential calls queue naturally.
- `pkg/api/tasks.go`: new `POST /api/task/scrape-javr-batch`
  (`{s, codes}`) — parses, launches the batch in the background, responds
  `{response: "OK", queued: N}`. Empty lists are ignored.
- New maintained test `pkg/tasks/content_javbatch_test.go` (first test in
  `pkg/tasks`, pure function, no DB needed).
- No UI changes: the file-upload control from the issue is the natural
  follow-up; the endpoint is curl-usable today
  (`curl -X POST localhost:9999/api/task/scrape-javr -d ...` shape with
  `codes`).

## How to test

1. `go test -vet=off ./pkg/tasks/ -run TestParseJavCodes` — passes.
2. `go build ./pkg/tasks/ ./pkg/api/` — passes.
3. Live (verified 2026-09-15 against a scratch server): `POST
   /api/task/scrape-javr-batch` with 3 codes (1 dupe) → `{queued: 2}`;
   log shows `Scraping JavDB`, `JAV batch: waiting 10s before DEF-456
   (2/2)`, then the second scrape exactly 10 s later. Fake codes → 0
   scenes, as expected; dispatch, dedupe and spacing all confirmed.
