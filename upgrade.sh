#!/bin/sh
# XBVR Synology one-step updater.
#
# Usage (over SSH as an admin user):
#   curl -fsSL -o /tmp/xbvr-upgrade.sh https://motorotor.github.io/xbvr/upgrade.sh \
#     && sudo sh /tmp/xbvr-upgrade.sh
#
# What it does:
#   1. Detects your NAS arch (from the installed XBVR package, else uname).
#   2. Looks up the newest matching .spk in the Package Center catalog.
#   3. Skips if you're already on it (override with --force).
#   4. Downloads, verifies md5, and installs via synopkg (in-place upgrade,
#      same as Package Center manual install: config and DB are preserved).
#
# DSM 7 only. Must run as root (via sudo).
set -eu

PKG="xbvr"
CATALOG="https://motorotor.github.io/xbvr/api/package"
FORCE=0

if [ "${1:-}" = "--force" ]; then FORCE=1; fi

if [ "$(id -u)" -ne 0 ]; then
  echo "error: run as root (e.g. sudo sh /tmp/xbvr-upgrade.sh)" >&2
  exit 1
fi

SYNOPKG="$(command -v synopkg || echo /usr/syno/bin/synopkg)"
if [ ! -x "$SYNOPKG" ]; then
  echo "error: synopkg not found" >&2
  exit 1
fi

# 1. Arch: trust the installed package first, fall back to uname mapping.
#    (Only avoton=intel and aarch64=arm builds are published.)
INFO="/var/packages/$PKG/INFO"
if [ -f "$INFO" ]; then
  ARCH="$(grep -E '^arch=' "$INFO" | cut -d'"' -f2)"
else
  case "$(uname -m)" in
    x86_64)  ARCH="avoton" ;;
    aarch64) ARCH="aarch64" ;;
    *) echo "error: no XBVR installed and unknown arch '$(uname -m)'" >&2; exit 1 ;;
  esac
fi
echo "arch: $ARCH"

# 2. Catalog lookup for this arch (pretty-printed JSON, one field per line).
CATALOG_JSON="$(curl -fsSL "$CATALOG")"
BLOCK="$(printf '%s' "$CATALOG_JSON" | grep -A40 "\"arch\": \"$ARCH\"")"
LINK="$(printf '%s' "$BLOCK" | grep -m1 '"link"' | cut -d'"' -f4)"
MD5="$(printf '%s' "$BLOCK" | grep -m1 '"md5"' | cut -d'"' -f4)"
LATEST="$(printf '%s' "$BLOCK" | grep -m1 '"version"' | cut -d'"' -f4)"
if [ -z "$LINK" ] || [ -z "$MD5" ] || [ -z "$LATEST" ]; then
  echo "error: no $ARCH build found in catalog" >&2
  exit 1
fi
echo "latest: $LATEST"

# 3. Skip if already current.
if [ -f "$INFO" ]; then
  CURRENT="$(grep -E '^version=' "$INFO" | cut -d'"' -f2)"
  echo "installed: ${CURRENT:-unknown}"
  if [ "$FORCE" -eq 0 ] && [ "$CURRENT" = "$LATEST" ]; then
    echo "already up to date."
    exit 0
  fi
fi

# 4. Download + verify + install.
SPK="/tmp/$PKG-upgrade-$LATEST.spk"
echo "downloading $LINK"
curl -fsSL -o "$SPK" "$LINK"
echo "$MD5  $SPK" | md5sum -c - >/dev/null
echo "checksum ok, installing (this takes a few minutes)..."
"$SYNOPKG" install "$SPK"
rm -f "$SPK"

"$SYNOPKG" status "$PKG" || true
echo "done. check: tail -f /var/packages/$PKG/var/xbvr.log"
