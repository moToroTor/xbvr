# Issue #1660 — DLNA generated filenames

Origin: https://github.com/xbapps/xbvr/issues/1660

## What
The built-in DLNA server named every scene
`"<cast> - <title> _180_180x180_3dh_LR.mp4"` regardless of the detected
video projection, so players picked the wrong projection for e.g.
`fisheye190` (`4080x2040 h264`) or `mkx200` (`5800x2900 hevc`) files.

## Why
`sceneToContainer` in `pkg/dms/dlna/dms/cds.go` hardcoded the
`_180_180x180_3dh_LR.mp4` suffix while the per-file detected projection
was already stored on `models.File.VideoProjection` (set during volume
scan from filename tokens / dimensions).

## How to test
1. Pick a scene whose Files tab shows `fisheye190` (or `mkx200`) and one
   that shows `180_sbs`:
   `sqlite3 main.db "select filename, video_projection from files where scene_id = <id>;"`
2. Browse the DLNA server (`BrowseDirectChildren` on `all`, e.g. with a
   UPnP inspector or DLNA client) and compare the container `Title`
   suffixes — the fisheye190 scene must now end with
   `_190_fisheye190_3dh_LR.mp4` (mkx200 → `_200_mkx200_3dh_LR.mp4`),
   not `_180_180x180_3dh_LR.mp4`.
3. Play via a DLNA client and confirm the correct projection is picked
   without manual override.
4. `go build ./pkg/dms/dlna/dms/` is clean.
