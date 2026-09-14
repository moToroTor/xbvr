# Fix for xbapps/xbvr#1459 — crash when a scanned file goes missing mid-scan

Origin issue: https://github.com/xbapps/xbvr/issues/1459

## What was wrong

`scanLocalVolume` (`pkg/tasks/volume.go`) walked the volume, then processed
each file later. A file moved/deleted in between hit two unchecked stats:

- video loop: `fStat, _ := os.Stat(path)` — nil `FileInfo` → `fStat.Size()`
  nil-pointer panic → process restarts, index left inconsistent.
- script loop: same pattern with `os.Stat` + `times.Stat`, same crash.

(`Hash`/`ffprobe` errors in the same function were already handled.)

## What this branch changes

- `pkg/tasks/volume.go`, video loop: on `os.Stat` error, log, delete the
  stale `files` row (path + filename + `video`), `continue`. On
  `times.Stat` error, log and `continue` (file is retried next rescan —
  nothing is deleted on a clock failure).
- Same nil-guards in the script loop (skip-only; scripts carry no
  re-derivable row worth pruning there).

Based on pristine `origin/master` (`adfe592`) — intentionally does NOT
include the `fix/flag-parse-breaks-go-test` baseline, so this stays
mergeable even if that PR is rejected.

## Why I think it's fixed

Every `FileInfo`/`Timespec` dereference in both loops is now preceded by an
error check; the only remaining failure modes log and continue. The stale-row
delete reuses the exact `Path`/`Filename`/`Type` key the scan itself uses.

## How to test

1. `go build ./...` — passes. (Full `go test` needs the test-runnable
   baseline from #2252 plus a `ui/dist` embed placeholder; see that issue.)
2. Start a rescan, delete/move a video mid-scan.
3. Confirm the process stays up, the log shows `Skipping <path>, stat
   failed`, and the stale `files` row is gone:
   `sqlite3 main.db "select * from files where filename='<name>';"` → empty.
4. Same drill with a `.funscript` file — skip is logged, no restart.

No pushes beyond this fork branch, no PRs, without explicit approval.
