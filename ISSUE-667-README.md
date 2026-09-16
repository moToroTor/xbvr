# Fix for xbapps/xbvr#667 — scheduled content-bundle URL polling

Origin issue: https://github.com/xbapps/xbvr/issues/667

## What was wrong

Manual bundle-URL import already existed (Options → Import/Export takes a URL
through `RestoreBundle`), but nothing polled bundle URLs automatically, so
community-generated scene data only landed via manual imports.

## What this branch changes

- `pkg/config/config.go`: new `cron.bundleSchedule` (same shape as the other
  schedules, default **disabled**, 12 h interval) plus
  `advanced.contentBundleUrls` (`[]string`).
- `pkg/tasks/content.go`: new `BundleURLScrape()` iterates the configured
  URLs (blank entries skipped) and imports each through the existing
  `RestoreBundle` path with unattended-safe defaults: scenes for all sites,
  no overwrite of local data.
- `pkg/server/cron.go`: wires `bundleSchedule` → `bundleCron()` following the
  existing per-schedule pattern (skips while a session is active).
- No UI changes: schedule/URL configuration is via the config file (and the
  existing manual URL import UI is untouched). A Schedules-view row and URL
  list editor would be the natural follow-up; the repo has no UI test harness
  to verify them with.

## How to test

1. `go build ./pkg/config/ ./pkg/tasks/ ./pkg/server/` — passes.
2. Set `advanced.contentBundleUrls: ["http://host/bundle.json"]` and enable
   `cron.bundleSchedule`; on the next tick scenes from the bundle appear
   (`sqlite3 main.db "select count(*) from scenes;"` grows).
3. Backward compatible: configs without the new keys get schedule disabled +
   empty URL list, so nothing polls unless configured.

## Verification notes (no maintained test)

- `pkg/tasks` has no test files and `RestoreBundle` needs a live DB, so a
  maintained unit test is disproportionate here; the new code is wiring over
  the already-exercised manual import path. Verified by build plus review of
  the cron/config patterns it mirrors.
