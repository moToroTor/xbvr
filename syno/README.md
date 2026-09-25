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
- **DSM Permissions:** the package runs as `sc-xbvr`; postinst grants
  it Read on the wizard video folders automatically (the exact File
  Station ACE). If a folder stays unreadable, grant it manually in
  Control Panel → Shared Folder.

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

## Troubleshooting (field notes)

All commands as root on the NAS unless noted. Package paths: live dir
`/var/packages/xbvr/var/` is a symlink to `/volume2/@appdata/xbvr` —
`chown`/`chmod` the real path (bare `-R` on the symlink goes nowhere).

- **Install fails, error 268 (dependent packages):** historical (a hard
  ffmpeg dep, since removed). No hard deps are declared anymore.
- **Install fails at postinstall (error 276):** the wizard defaults
  MariaDB on with an empty root password. `ensure_db` tries TCP
  (`host:port` + root password) then the local mysqld socket
  (passwordless `unix_socket` root). If both refuse, install fails
  loudly — supply the real root password via the GUI wizard.
- **Package shows `broken`:** `synopkg uninstall xbvr` (keeps `var/`
  unless data removal is ticked), then reinstall.
- **Start fails ("Failed to run the package service"):** postinst runs
  as root, the daemon as `sc-xbvr`. If `.env`/`bin/` are root-owned,
  prestart dies sourcing `.env`. postinst hands them to the var-dir
  owner on every path; manual repair:
  `chown -R sc-xbvr:synocommunity /volume2/@appdata/xbvr`.
- **Every log line twice:** fixed (daemon stdout to `/dev/null`,
  stderr to `var/xbvr.err.log` for Go panics). Pre-fix logs just look
  noisy; panics were always single (stderr-only).
- **Adding a folder panics (pre-fix) / 400s:** the daemon user needs
  read+traverse on the video path. postinst grants it automatically;
  verify with
  `su -s /bin/sh sc-xbvr -c 'ls -ld /volume2/<share>/...'`, else grant
  `sc-xbvr` Read on the share (Control Panel → Shared Folder).
- **`database is locked`:** check for two daemons
  (`ps aux | grep -i xbvr`) — one is correct. A crashed run can also
  leave a stale `lock-*` row (fatal exits skip defers), after which
  every job silently no-ops:
  `sqlite3 /var/packages/xbvr/var/main.db "PRAGMA busy_timeout=5000; select * from kvs where key like 'lock%';"`
  Stop the package before deleting stale rows.
- **Change settings after install:** reinstall the same version over
  the top — the upgrade wizard pre-fills every value from `.env`.
  (DSM-native config dialogs like MariaDB10's are first-party only;
  third-party `.spk`s cannot ship one.)
- **sqlite vs MariaDB:** sqlite is the default and fine into the GBs
  for this workload. MariaDB helps only with real write contention
  (rising `database is locked` fatals), never with job scheduling —
  scrape/rescan/index run under single-holder KV locks on any engine.
  Watch the ports: DSM's system MariaDB is 3306, the MariaDB10 package
  defaults to 3307; point the wizard at whichever answers.

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
