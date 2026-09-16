# Fix for xbapps/xbvr#1414 — funscript update wipes human/AI script flags

Origin issue: https://github.com/xbapps/xbvr/issues/1414

## What was wrong

Worse than the report suspected. Two paths read two different phantom shapes:

- New-scene path read `scripts[].scriptAI` — correct.
- Funscript-update path (studio listing) read a `fleshlight[]` array with an
  `isAiScript` key.

Verified live 2026-09-15 against both v3 endpoints (single scene
`i-see-you-watching-me-37474`, which genuinely has one human + one AI script,
and a studio listing page): **neither `fleshlight` nor `isAiScript` exists
anywhere** — both endpoints return `scripts[]` keyed by `scriptAI`. So the
update path always computed `(false, false)` and, worse, the
`existingScene.HumanScript != human` check then fired and emitted an update
that **cleared both flags**. Every funscript update actively wiped the flags.

## What this branch changes

- New `slrScriptFlags` helper folds a v3 `scripts` array into `(human, ai)`;
  both paths use it, so they cannot disagree again.
- Update path now reads `scene.Get("scripts")` instead of the phantom
  `fleshlight` array.
- New-scene path refactored onto the helper (provably equivalent: flags start
  false; `HasScriptDownload` set iff at least one entry, same as before).
- New maintained test `pkg/scrape/slrstudios_test.go`: human-only, AI-only,
  both, empty array, key-missing entry.

## How to test

1. `go test -vet=off ./pkg/scrape/ -run TestSlrScriptFlags` — passes.
   (`-vet=off`: pre-existing vet failures in `javlibrary.go`/`vrporn.go`.)
2. `go build ./pkg/scrape/` — passes.
3. Live: scrape SLR scene 37474 → `human_script=1 AND ai_script=1`; re-run
   the funscript update → flags stay set (previously wiped).
