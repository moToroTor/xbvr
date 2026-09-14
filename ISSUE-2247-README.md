# Issue #2247 — Bundle restore ignores JSON decode errors, reports success after partial import

Origin: https://github.com/xbapps/xbvr/issues/2247

## What
Restoring a v2.1 content bundle containing one malformed field (e.g. an
actor `birth_date: ""` that fails `time.Time` parsing) returned apparent
success and logged `Restore complete`, while only a sparse partial scene
row was written. Most of the bundle was silently dropped.

## Why
In `RestoreBundle` (`pkg/tasks/content.go`), the decode call was:

```go
var err error
...
json.UnmarshalFromString(request.UploadData, &bundleData)

if err == nil {
    ... all Restore* writes ...
    tlog.Infof("Restore complete")
}
```

The `UnmarshalFromString` return value was never assigned, so `err`
stayed `nil` forever: even when decoding failed (Go still populates a
partial `bundleData` — verified: unmarshalling `""` into `time.Time`
returns a parse error plus a zero value), the code proceeded through every
`Restore*` write and ended in `Restore complete`.

## Fix (2 lines)
- Assign the decode error: `err = json.UnmarshalFromString(...)`, so a
  malformed bundle skips all `Restore*` writes and never logs
  `Restore complete`.
- Log the failure with its cause at error level:
  `tlog.Errorf("Restore failed: %v", err)` (was a bare
  `tlog.Infof("Restore failed!")` that was previously unreachable).

Note: `restoreBundle` in `pkg/api/tasks.go` launches `RestoreBundle`
asynchronously (`go tasks.RestoreBundle(r)`), so the HTTP response is
sent before decoding happens — the failure surfaces in the task log,
not as an HTTP status. Making the handler synchronous would be a larger
behavioral change, deliberately left out of this minimal fix.

## How to test
1. `go build ./pkg/tasks/ ./pkg/api/` passes (repo needs a temporary
   `ui/dist/` placeholder for the `ui` embed directive; removed
   afterwards). `gofmt -l` clean. (`go test` not run: broken
   repo-wide, pre-existing.)
2. POST a bundle containing `"birth_date": ""` to
   `/api/task/bundle/restore`; confirm the task log shows
   `Restore failed: ...` and NOT `Restore complete`, and no partial
   scene rows are written.
3. Re-submit with `"birth_date": "0001-01-01T00:00:00Z"`; confirm all
   scenes restore (`sqlite3 main.db "select count(*) from scenes;"`).
4. UI: Options > Scene Data > Restore bundle with the bad file;
   confirm an error entry in the task log, not success.
