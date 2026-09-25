# XBVR Synology package (DSM 7, `syno/` build dir in this checkout)

Hand-rolled `.spk` following the proven Wankarr patterns (dispatcher +
six per-action stubs, service-user privilege, dynamic wizard
generators, no `adminurl`, plain-tar outer).

## What the wizard asks

- **Server:** web UI port (default 9999, change it on collision) +
  video **share name** (e.g. `porn`) + optional subfolders. Resolved to
  `/volume*/<share>/...` at install and saved to `var/volumes.txt` —
  add each as a local Volume in XBVR under Settings → Storage, then
  delete the reminder. With no share set, absolute subfolder entries
  are used as-is.
- **Database:** MariaDB checkbox (default on) + host (127.0.0.1) +
  port (**3307**, DSM's default) + name/user/password + **root
  password**. Uncheck for sqlite. The installer **creates the database
  and user idempotently** (`IF NOT EXISTS`, utf8mb4) and fails loudly
  when MariaDB is unreachable. The root password is used once and
  **never stored**.
- **DSM Permissions:** the package runs as `sc-xbvr`; grant it Read on
  every video share in Control Panel → Shared Folder. A wizard cannot
  grant share ACLs itself — this manual step stays.

Upgrades pre-fill every box from `.env` (the MariaDB URL is split back
into parts) and `volumes.txt`. Reinstalls and wizard-less upgrades
never overwrite `.env`. Install scripts run as root but the daemon runs
as the service user, so postinst hands `.env`/`volumes.txt`/`bin/` to
the var-dir owner — an unreadable `.env` aborts service start.

## ffmpeg: none of our business

No ffmpeg dependency is declared and postinst does no ffmpeg handling:
XBVR self-downloads its static 4.2.1 `ffprobe`/`ffmpeg` pair into
`var/bin/` on first run. Measured on a 21-file probe-failure corpus,
the SynoCommunity ffmpeg6 build fails the exact same 8 genuinely broken
files — no demuxer advantage found, so the extra machinery (and the
hard dep that blocked installs) was removed.

## Building & distribution

1. Build the binary with old-glibc CGO (see Makefile `pack` notes).
2. `make pack BINARY=<path> VERSION=<ver> ARCH=avoton` (from `syno/`).
3. Tag `syno-v*` in this fork → Actions matrix builds both arches →
   `.spk` assets on the fork's **Release**, `.spk`/icons/catalog on the
   fork's gh-pages `dl/` + `api/package`. The Wankarr repo can then
   list it (direct Pages links only — DSM can't follow
   releases/download 302s).

`make` runs `go vet` + the packer suite: INFO shape (9999, no
adminurl, no hard deps), wizard generators + prefill (incl. checkbox
booleans), MariaDB create SQL + loud failure + root-secret hygiene,
share resolution, var ownership, installer failure propagation.
