# Fix for xbapps/xbvr#1896 — SLR scenes with emoticons in titles fail to scrape

Origin issue: https://github.com/xbapps/xbvr/issues/1896

## What was wrong

Scene 51115's title ends with 💦, and the slug a browser shows (and the user
pastes) carries it: `...pussy💦-51115`. The v3 API matches labels exactly, so
that label 404s while the canonical ASCII label and the bare numeric ID both
return 200 with identical bytes (verified live 2026-09-15: `.../51115` → 200,
emoji label → 404 "Resource not found"). The scraper had no recovery: label
404 → project=0 retry (same bad label) → legacy endpoint → give up.

## What this branch changes

- `pkg/scrape/slrstudios.go`: after the existing fallbacks, when the lookup
  still isn't 200, retry once with the numeric scene ID via the new
  `slrLabelFallback` helper (fires only when the ID is all digits and differs
  from the label, so plain and already-numeric labels behave as before — the
  numeric lookup returns the same scene either way).
- New maintained test `pkg/scrape/slrstudios_test.go` covering emoticon,
  plain, already-numeric, non-numeric and empty cases.

## How to test

1. `go test -vet=off ./pkg/scrape/ -run TestSlrLabelFallback` — passes.
   (`-vet=off`: pre-existing vet failures in `javlibrary.go`/`vrporn.go`.)
2. `go build ./pkg/scrape/` — passes.
3. Live: single-scene scrape the reported URL form (emoji slug + `-51115`) →
   scene created (previously 404'd out).
