# Fix for xbapps/xbvr#1309 — Scenes page times out on large libraries / Pi

Origin issue: https://github.com/xbapps/xbvr/issues/1309

## What was wrong

Every Scenes page load runs **5 filtered count queries** plus a
`GROUP BY scene_id / ORDER BY release_date` page query (`QueryScenes`,
`pkg/models/model_scene.go:704`), over columns with **no indexes**
(`scenes` had only `scene_id`, `deleted_at`, `scraper_id`). Cost grows with
library size — exactly the reported symptom — and Pi SD-card I/O amplifies
full-table sorts into timeouts.

Measured on a real 30k-scene library (fast SSD!):

| query | before | after |
|---|---|---|
| count (hidden filter + group) | 8.60s | 0.18s (48×) |
| count (available/accessible) | 0.19s | 0.03s |
| page (80 rows, default sort) | 1.85s | 0.38s (5×) |

## What this branch changes

- `pkg/migrations/migrations.go`: new migration
  `0088-add-scene-list-indexes` adding, via the same `AddIndex` pattern as
  0087 (which fixed the identical problem on the Scrapers page):
  - `idx_scenes_list (is_hidden, release_date)` — page query + default sort
  - `idx_scenes_available (is_hidden, is_available, is_accessible)` — counts
- Nothing else. No query changes, no UI changes; the planner picks the
  indexes up on both SQLite and MySQL.

Based on pristine `origin/master` (`adfe592`) — intentionally does NOT
include the `fix/flag-parse-breaks-go-test` baseline.

## Why I think it's fixed

The before/after above ran the actual page/count SQL shapes against a copy
of a production 30k-row database with only these two indexes added. The
slowest per-load query drops from 8.6s (already past browser timeout
budgets; far worse on Pi storage) to 0.18s on SSD.

## How to test

1. `go build ./...` — passes.
2. Back up `main.db`. Start XBVR, watch the log for
   `Running migration 0088-add-scene-list-indexes`, open Scenes.
3. `sqlite3 main.db ".indexes scenes"` shows both new indexes.
4. On a large library, time the page load before/after (browser devtools
   Network on `POST /api/scene/list`); expect single-digit seconds → <1s.
5. Reporter check: Pi 4 + ~100 scenes page loads without `TimeoutError`.

No pushes beyond this fork branch, no PRs, without explicit approval.
