# Fix for xbapps/xbvr#2283 — Files Match dialog is silently empty while index rebuilds

Origin issue: https://github.com/xbapps/xbvr/issues/2283

## What was wrong

`SceneMatch.vue` finds candidates through `/api/scene/search`, which needs
the bleve index. During a bundle import or reindex the `index` lock is held
and the query hangs until its 60s timeout (or returns empty) — the dialog
showed an empty table with no explanation, and the search promise rejection
was unhandled. No backend change: the dialog just never said what was wrong.

## What this branch changes

`ui/src/views/files/SceneMatch.vue` only:

- On open (and file change), it checks `GET /api/options/state/search`
  first. If a rebuild is in progress it shows a warning notice — "Can't
  search for matching scenes while the search index is rebuilding. Results
  will load automatically when the task finishes." — and polls the state
  every 5s instead of firing doomed searches.
- When the poll clears, it runs the pending search automatically with the
  current query string, so results appear on their own.
- A search failure mid-session (rebuild started after the dialog opened)
  re-checks state instead of dropping into an empty table; timers are
  cleared on close/destroy.

## Why I think it's fixed

The dialog can no longer sit empty without telling the user why, and the
user no longer has to guess when to retry — the retry is automatic. Normal
(non-rebuild) behavior is unchanged: same request, same table.

## How to test

1. Start a bundle import (or reindex), open Files > match on any file —
   the notice shows instead of an empty table; when indexing finishes,
   results load without further clicks.
2. Normal match flow with no rebuild running — unchanged.
3. Verification here: no UI test harness exists in the repo, so no
   maintained test was added; the extracted `<script>` passes
   `node --check`, template tags balance, and no Go files were touched
   (gofmt gate and `go build` unaffected by construction).

No pushes or PRs from this branch without explicit approval.
