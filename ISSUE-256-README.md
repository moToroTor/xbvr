# Issue #256 — DeoVR JSON breaks on unescaped characters

Origin: https://github.com/xbapps/xbvr/issues/256

## What

The DeoVR JSON payload (`/deovr/{scene-id}` etc.) breaks for scenes whose
stored text contains backslashes (and potentially other stray characters).
Reported against the R18 scene `KMVR-387` via Discord.

## Why

In `getDeoScene` (`pkg/api/deovr.go`) the stored `ChromaKey` string was
round-tripped through `gjson` + `encoding/json` with ignored errors:

- `gjson.Valid` is lenient and accepts input such as doubly-escaped JSON
  (`"{\"enabled\":true}"`, i.e. containing literal backslashes) that
  `json.Unmarshal` into a map rejects.
- On that failure `ckdata` stayed `nil`, and the next line
  `ckdata["hasAlpha"] = "false"` panicked with
  `assignment to entry in nil map`, killing the handler and producing a
  broken/empty DeoVR response for the scene.
- Errors were swallowed with `fmt.Println`, and scene text was passed to
  the encoder unsanitized, so invalid UTF-8 bytes could likewise fail
  `json.Marshal`.

## Fix (`pkg/api/deovr.go`)

- chromaKey round-trip is now defensive: if `json.Unmarshal` fails (or yields
  a nil map) it logs via `log.Errorf` and falls back to `{}`; the re-marshal
  error is handled the same way. No more nil-map panic, no more swallowed
  errors, and the `fmt.Println` / `_ = chromaKey` warts are gone.
- Added `sanitizeDeoString` (`strings.ToValidUTF8`) applied to scene
  Title/Description in `getDeoScene`/`getDeoFile` and to list titles in
  `scenesToDeoList`/`filesToDeoList`, so all DeoVR string output passes
  through `json.Marshal` cleanly. Valid text is unchanged.

## How to test

1. `go build ./pkg/api/` passes (add an empty `ui/dist/testkeep.txt`
   placeholder if the `ui` embed error appears; delete it after).
2. Seed a scene with backslashes/quotes/newlines in title/synopsis (e.g.
   `KMVR-387` data) and with doubly-escaped `ChromaKey`
   (`"{\"enabled\":true}"`).
3. `curl localhost:9999/deovr/<scene-id> | python3 -m json.tool` parses
   without error (previously the request died on the chromaKey panic).
4. Same check for `/deovr` (library) and `/deovr/file/<id>` endpoints.
