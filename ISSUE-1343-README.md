# Fix for xbapps/xbvr#1343 — manual projection override per file

Origin issue: https://github.com/xbapps/xbvr/issues/1343

## What was wrong

Players dropped the projection override, so a wrongly derived projection
could not be corrected: rescan re-derives `VideoProjection` from
ffprobe/filename on every pass with no way to pin a value.

## What this branch changes

- `pkg/models/model_file.go`: new `ProjectionOverride` column (empty =
  automatic). Picked up by the existing `AutoMigrate(&models.File{})` at
  startup — no manual migration needed. Included in `xbvrbackup` tags.
- `pkg/tasks/volume.go`: after ffprobe/filename derivation, a non-empty
  override wins, so rescan preserves the user's choice (including after
  re-downloads that trigger reprocessing).
- `pkg/api/files.go`: new `PUT /api/files/file/{id}/projection`
  `{projection}` with allowlist validation (`180_sbs`, `180_mono`,
  `360_tb`, `360_mono`, `fisheye`, `fisheye190`, `mkx200`, `mkx220`,
  `rf52`, `vrca220`, `flat`, plus `""` to clear back to auto). Sets both
  `VideoProjection` (what heresphere/deovr/filters already read — no
  consumer changes needed) and the override.
- `ui/src/views/files/List.vue`: Projection column with per-video dropdown
  (`Auto (<current>)` + list), `PUT`s on change and reloads.

Based on pristine `origin/master` (`adfe592`) — intentionally does NOT
include the `fix/flag-parse-breaks-go-test` baseline.

## Why I think it's fixed

Single source of truth (`files.video_projection`) is now writable and
rescan-proof; every existing consumer reads that field unchanged, and the
allowlist matches exactly the values derivation and scene filters already
use. Go side compiles (`go build ./pkg/...`) and is gofmt-clean.

## How to test

1. `go build ./...` — passes.
2. `curl -X PUT localhost:9999/api/files/file/<id>/projection -d '{"projection":"180_mono"}'`
   then `sqlite3 main.db "select video_projection, projection_override from files where id=<id>;"` → both `180_mono`.
3. Rescan (touch size to force reprocessing) → values preserved.
4. `PUT ... '{"projection":""}'` → back to auto on next reprocess.
5. UI: Files list shows dropdown; Heresphere/DeoVR JSON for the scene
   reports the overridden projection. (UI added following existing
   buefy/ky patterns — needs a `yarn lint` + browser pass; no node
   toolchain was available here.)
6. `PUT ... '{"projection":"bogus"}'` → 400.

No pushes beyond this fork branch, no PRs, without explicit approval.
