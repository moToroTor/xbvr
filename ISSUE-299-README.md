# Issue #299 — Filter & Sort by file path

Origin: https://github.com/xbapps/xbvr/issues/299

## What
Users who organise files in per-studio folders can't filter/sort scenes by
file path, which makes manual matching hard (you can't tell which studio a
file came from to narrow down the scene).

## Why
Scene queries (`queryScenes` / `RequestSceneList`) supported volume, site,
cast, tag, etc., but had no filename/path filter or sort key over the
`files` join. The files list endpoint supported a filename filter only, with
no path filter/sort.

## Fix
- `pkg/models/model_scene.go`
  - `RequestSceneList`: new `filename` (`json:"filename"`) and `file_path`
    (`json:"file_path"`) filters.
  - `queryScenes`: `LIKE %…%` filters on `files.filename` / `files.path`,
    sharing a single `left join files` with the existing volume filter
    (`ensureFilesJoin` so the join is added once).
  - New sort keys `filename_asc/desc`, `file_path_asc/desc` ordering by
    `files.filename` / `files.path` (join ensured for the sort as well).
- `pkg/api/files.go`
  - `RequestFileList`: new `path` (`json:"path"`) filter
    (`path LIKE %…%`) plus `path_asc` / `path_desc` sorts. No change needed in
    `pkg/api/scenes.go: getScenes` — it deserialises `RequestSceneList`
    directly, so the new keys flow through automatically.
- UI scene list
  - `ui/src/store/sceneList.js`: new `filename`, `file_path` filter state
    (persisted in the `q` query param like the other filters).
  - `ui/src/views/scenes/Filters.vue`: Filename + File path inputs with clear
    buttons, and Filename / File path sort options.
- UI files list (used for manual matching)
  - `ui/src/store/files.js`: new `path` filter state.
  - `ui/src/views/files/Filters.vue`: Path input with clear button (>3 chars
    triggers reload, same as filename).
- Path display: already present, no change — files list shows
  `filename` + `path` (`ui/src/views/files/List.vue`), the match dialog shows
  both (`ui/src/views/files/SceneMatch.vue`), and scene details show both
  (`ui/src/views/scenes/Details.vue`).

## How to test
1. `go build ./pkg/models/` passes; `go build ./pkg/api/` passes (with a
   temporary `ui/dist/testkeep.txt` placeholder for the pre-existing
   `ui/fs.go` embed pattern; placeholder removed afterwards).
2. `POST /api/scene/list` with `{"filename": "<part>", "file_path": "<studio>", "sort": "file_path_asc"}` returns only matching scenes, ordered by folder.
3. `POST /api/files/list` with `{"path": "<studio>"}` returns only files under that path.
4. UI: Scenes → Filters → Filename / File path inputs filter the list; Sort by
   Filename / File path orders it. Files view → Path input filters unmatched
   files for manual matching.
