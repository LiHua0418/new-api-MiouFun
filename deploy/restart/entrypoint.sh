#!/bin/sh
# Stable entrypoint: resolve one complete release, then replace the shell with Go.
set -eu
runtime=/data/new-api-runtime
release=$(readlink -f "$runtime/current")
case "$release" in
  "$runtime"/releases/*) ;;
  *) printf '%s\n' 'Invalid New API release path' >&2; exit 1 ;;
esac
[ -x "$release/new-api" ] || { printf '%s\n' 'New API release is missing' >&2; exit 1; }
expected=$(cat "$release/SHA256")
actual=$(sha256sum "$release/new-api")
actual=${actual%% *}
[ "$actual" = "$expected" ] || { printf '%s\n' 'New API release checksum mismatch' >&2; exit 1; }
# This flag is implemented and tested in the approved CPU-fix binary.
export SKIP_DATABASE_MIGRATION=true
exec "$release/new-api" "$@"
