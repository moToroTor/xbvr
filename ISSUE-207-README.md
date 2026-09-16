# Issue #207 — Bitrate not detected for VP9 files

Origin: https://github.com/xbapps/xbvr/issues/207

## What

Files with VP9 encoding (typically WebM) end up with `video_bitrate = 0`
(and often `video_duration = 0`) in the database after a rescan.

## Why

`scanLocalVolume` in `pkg/tasks/volume.go` only read the **stream-level**
`bit_rate` from ffprobe. ffprobe omits stream-level `bit_rate` (and often
`duration`) for VP9/WebM streams, so the values stayed 0. Format-level
values were ignored for bitrate, and a stream `duration` of `"0"` shadowed
the format duration.

## Fix (`pkg/tasks/volume.go`)

- Bitrate fallback chain: stream `bit_rate` → format `bit_rate` →
  derived `size * 8 / duration` (last resort, needs duration + size).
- Duration: ignore zero/parseable-but-empty stream durations so the
  format-level `DurationSeconds` fallback actually applies; nil-guard
  `ffdata.Format`.
- Rescan now re-probes files with `VideoBitRate == 0`, so existing VP9
  rows are backfilled on the next rescan (previously only
  `VideoDuration == 0` triggered a re-probe).

## How to test

1. `go build ./pkg/tasks/` passes (add an empty `ui/dist/testkeep.txt`
   placeholder if the `ui` embed error appears; delete it after).
2. Place a VP9/WebM sample in a test volume, run Rescan.
3. `sqlite3 xbvr.db "select video_bitrate, video_codec_name, video_duration from files where filename like '%.webm';"`
   shows nonzero bitrate/duration.
4. Re-run Rescan; values stay stable (no flip back to 0).
