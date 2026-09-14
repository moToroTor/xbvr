# Issue #348 — Support wildcards in scene filenames matching

Origin: https://github.com/xbapps/xbvr/issues/348

## What
VirtualTaboo download filenames embed an unknowable numeric ID, e.g.
`daughter-bought-mini-monokini-files-28690-gear.mp4`, which the scraper
cannot reproduce, so automatic file→scene matching always failed for that
site. This change lets scraped `filenames` contain `*`/`?` wildcards and
teaches the rescan matcher to honor them.

## Why
Exact-match-only logic (`filenames_arr LIKE %"<name>"%` in
`pkg/tasks/volume.go`) can never match a file whose middle segment is a
per-download ID. Emitting a wildcard pattern at scrape time plus glob
matching at rescan time fixes the whole class (same shape as the NAVR
random-prefix problem).

## How to test
1. Re-scrape a VirtualTaboo scene — stored `filenames_arr` should now
   contain patterns like `base-files-*-gear*.mp4`
   (one per player: `smartphone`, `gear`, `psvr`, `oculus`, `oculus5k`,
   `oculus5k10`, `6k`, `7k`).
2. Place a real file `...-files-28690-gear.mp4` (or the long
   `...-files-28690-gear_180_LR.mp4` form) in a volume and rescan —
   it should auto-match the scene.
3. Confirm non-matching files still don't match: other-scene bases and
   other players must not match (verified via `filepath.Match`
   semantics: `*` matches the numeric ID and optional suffix, base and
   player segments still anchor the pattern).
4. `gofmt -l` clean on touched files; `go build ./pkg/scrape/
   ./pkg/tasks/` passes (repo needs a temporary `ui/dist/` placeholder
   for the `ui` embed directive; removed afterwards).
