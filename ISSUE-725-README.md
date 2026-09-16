# Fix for xbapps/xbvr#725 — wrong `<upnp:class>` breaks strict DLNA clients

Origin issue: https://github.com/xbapps/xbvr/issues/725

## What was wrong

`cdsObjectToUpnpavObject` (`pkg/dms/dlna/dms/cds.go`) built the class as
`"object.item." + mimeType.Type() + "Item"`, where `Type()` is the text
before `/`. Two MIME types slip through the `IsVideo()` gate above it while
not starting with `video/`:

- `.rmvb` → `application/vnd.rn-realmedia-vbr` (explicitly treated as video
  in `IsVideo()`) produced `object.item.applicationItem`
- anything unrecognized → `application/octet-stream` produced the same

`object.item.applicationItem` violates the `object.item.*` AV schema, so
strict clients (VLC, Skybox) mishandle or reject the items.

## What this branch changes

- `pkg/dms/dlna/dms/mimetype.go`: new `upnpClassForMimeType()` whitelist —
  video/audio/image map to their `*Item` class, everything else falls back
  to `videoItem` (callers only reach it for video files).
- `pkg/dms/dlna/dms/cds.go`: uses the helper instead of string concatenation.
- `pkg/dms/dlna/dms/mimetype_test.go`: asserts `.rmvb` and
  `application/octet-stream` yield `videoItem`, never anything containing
  `application`.

## Why I think it's fixed

The invalid string is unconstructible now: every path through the helper
returns a schema-valid class, pinned by unit test including the exact
`.rmvb` case from the report.

## How to test

1. `go test ./pkg/dms/dlna/dms/ -run TestUpnpClassForMimeType -v` — passes.
2. `go test ./pkg/dms/...` — whole DLNA tree still passes.
3. Live: browse the CDS (`/dlna/cds.xml` Browse) and confirm every video
   item carries `<upnp:class>object.item.videoItem</upnp:class>`; open in
   VLC/Skybox and confirm the listing plays.

No pushes or PRs from this branch without explicit approval.
