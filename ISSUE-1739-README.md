# Fix for xbapps/xbvr#1739 — Files with "&" never rematch after move+rescan

Origin issue: https://github.com/xbapps/xbvr/issues/1739

## What was wrong

`RescanVolumes` (`pkg/tasks/volume.go`) matched files to scenes with a single
HTML-escaped `LIKE` pattern. That breaks two ways:

1. **Legacy rows**: scenes whose `filenames_arr` was written with a raw `&`
   (older writes) can never match the escaped (`\u0026`) query.
2. **MySQL**: the backslash in `\u0026` is itself a `LIKE` escape, so the
   escaped-only query fails there even against escaped rows. SQLite was
   unaffected, which is why this only bites some installs.

A side bug in the same function: the alternate-source fallback was
`A AND b OR c OR d…`, so the `external_source like 'alternate scene %'`
filter applied to only the first pattern.

## What this branch changes

- New `pkg/tasks/filename_match.go`: `filenameMatchVariants()` returns both
  raw and escaped basename forms (plus sidecar extension swaps), and
  `escapeLike()` quotes LIKE metacharacters for use with `ESCAPE '\'`.
- `pkg/tasks/volume.go`: builds the `filenames_arr` and `external_data`
  queries from those variants (5–10 patterns), adds `ESCAPE '\'` so behavior
  is identical on SQLite and MySQL, and parentshesi​ses the alternate-source
  filter correctly.
- New `pkg/tasks/filename_match_test.go`: `TestMatchAmpersand` asserts both
  `A & B.mp4` and `A \u0026 B.mp4` are queried, plus sidecar-swap and
  LIKE-escaping tests.

## Why I think it's fixed

The mismatch class is eliminated by construction: every stored form the code
has ever written is now queried, and `ESCAPE '\'` removes the
SQLite/MySQL divergence. Unit tests pin both forms.

## How to test

1. `mkdir -p ui/dist && echo test > ui/dist/testkeep.txt` (embed placeholder,
   frontend not built in dev checkouts), then
   `go test ./pkg/tasks/ -run 'TestMatchAmpersand|TestFilenameMatchVariants|TestEscapeLike' -v`
   — all pass. Remove the placeholder after.
2. Manual: match a file named e.g. `Tom & Jerry.mp4` to a scene, move it to
   another volume, rescan — it rematches instead of landing in Unmatched.
3. MySQL installs: same manual flow; previously failed, now matches.

No pushes or PRs from this branch without explicit approval.
