# Fix for xbapps/xbvr#1597 — WankItNowVR scenes lack a members URL

Origin issue: https://github.com/xbapps/xbvr/issues/1597

## What was wrong

`TwoWebMediaSite` (`pkg/scrape/zexywankitnow.go`) never set `sc.MembersUrl`,
so WankItNowVR scenes had no members link. Members links live at
`members.wankitnowvr.com/m/videos/...` vs public `wankitnowvr.com/videos/...`.

## What this branch changes

- New `wankitnowMembersURL` helper maps the public URL to the members URL;
  the handler calls it for `wankitnowvr` scenes only. `zexyvr` shares the
  handler but its members host is unverified, so it is deliberately left
  unset there rather than invented.
- Persistence needs no change: `PopulateSceneFieldsFromExternal`
  (`pkg/models/model_scene.go`) already copies `MembersUrl`.
- New maintained test `pkg/scrape/zexywankitnow_test.go`: public URL maps,
  an already-members URL passes through (guards against double-prefixing),
  other hosts pass through.

## How to test

1. `go test -vet=off ./pkg/scrape/ -run TestWankitnowMembersURL` — passes.
   (`-vet=off` because the package has pre-existing vet failures in
   `javlibrary.go`/`vrporn.go`, untouched by this branch.)
2. `go build ./pkg/scrape/` — passes.
3. Live: scrape one WankItNowVR scene; members URL points at the members
   subdomain. Members host responds 200 (verified 2026-09-15); no scene slug
   was available to check a full member page, so that half is unverified.
