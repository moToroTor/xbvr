# Issue #1601 — Crash on VRHush scraper

Origin: https://github.com/xbapps/xbvr/issues/1601

## What
VRHush scrape panics with `interface conversion: interface {} is nil, not string`,
killing the app. Stack trace points at `pkg/scrape/vrhush.go:99`
(`content["videos_duration"].(string)`).

## Why
Commit 5e08a4d (#1573) added key-existence (`ok`) guards, but a key that exists
with a JSON-null value still passes the `ok` check, and the unconditional
`.(string)` / `. غالب([]interface{})` / `.(map[string]interface{})` assertions
then panic. The same pattern exists for the `props/pageProps/content` chain,
`scene_code`, `title`, `trailer_screencap`, `description`, `tags`, `models`
(`name`/`slug`), `publish_date`, `videos_duration`, and `videos` (`file`/`url`).

## Fix
`pkg/scrape/vrhush.go` only:
- Added `vrhushString` / `vrhushMap` / `vrhushSlice` helpers (nil + type checks).
- Guarded the `props → pageProps → content` chain; return early (skip scene) if
  any level is missing/wrong type instead of panicking.
- `scene_code`: safe string extract, skip scene with a warning if missing/empty.
- `title`, `trailer_screencap`, `description`, `publish_date`: safe strings
  (empty values skipped).
- `tags`: safe slice, non-string entries skipped.
- `models`: safe slice from `pageProps` (fallback `content`), safe
  `gender`/`name`/`slug` strings; non-Female or malformed entries skipped.
- `videos_duration`: type switch handles `string` (existing behaviour),
  `float64`/`float32`/`int`/`int64`; nil/wrong types ignored (fixes line :99).
- `videos`/`file`/`url`: safe map + safe strings; malformed entries skipped.

Behaviour on valid payloads is unchanged.

## How to test
1. `go vet ./pkg/scrape/` (only pre-existing warnings in other files) and
   `go build ./pkg/scrape/` — both pass.
2. Run a single-site scrape for vrhush; confirm no panic in logs.
3. `curl -s localhost:9999/api/scrape | head -c 300`.
4. Confirm VRHush scenes present in DB.
