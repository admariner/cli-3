#!/bin/bash
set -euo pipefail

# Syncs the vendored API client (internal/planetscale) into a checkout of
# planetscale/planetscale-go. The CLI copy is the source of truth; the
# planetscale-go repo is a read-only mirror for external users. See
# doc/api-client.md.

usage="usage: sync-planetscale-go.sh <cli checkout> <planetscale-go checkout>"
CLI_DIR=${1:?$usage}
DEST_DIR=${2:?$usage}

SRC="$CLI_DIR/internal/planetscale"
DEST="$DEST_DIR/planetscale"

if [ ! -d "$SRC" ] || [ ! -d "$DEST" ]; then
  echo "error: expected $SRC and $DEST to exist" >&2
  exit 1
fi

rm -f "$DEST"/*.go
cp "$SRC"/*.go "$DEST"/

# CLI-only files: the package doc describing the vendoring, the test that
# guards against the CLI depending on the planetscale-go module, and
# internal (non-v1) API endpoints, which stay out of the public module.
# See doc/api-client.md.
rm -f "$DEST/doc.go" "$DEST/dependency_test.go" "$DEST"/*_internal.go "$DEST"/*_internal_test.go

echo "Synced $(ls "$DEST"/*.go | wc -l | tr -d ' ') files into $DEST"
