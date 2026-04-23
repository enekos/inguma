#!/usr/bin/env bash
# End-to-end smoke for Inguma v2 Track A.
# Builds apid + inguma, seeds a fixture corpus with two versions of
# @foo/bar, and exercises install + --frozen + upgrade against a real apid.
set -euo pipefail

HERE="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
ROOT="$( cd "$HERE/.." && pwd )"
cd "$ROOT"

TMP=$(mktemp -d)
APID_PID=""
cleanup() {
    if [ -n "$APID_PID" ] && kill -0 "$APID_PID" 2>/dev/null; then
        kill "$APID_PID" 2>/dev/null || true
        wait "$APID_PID" 2>/dev/null || true
    fi
    chmod -R u+w "$TMP" 2>/dev/null || true
    rm -rf "$TMP" 2>/dev/null || true
}
trap cleanup EXIT

# Isolate state.DefaultPath() which uses $HOME/.inguma/state.json.
# Keep Go's module cache on the real filesystem so cleanup can delete $TMP.
export HOME="$TMP/home"
mkdir -p "$HOME"
# Satisfy harness "installed?" detection by creating a dummy Claude Code config.
mkdir -p "$HOME/.claude"
echo '{}' > "$HOME/.claude.json"

echo "== build =="
make build >/dev/null

echo "== seed fixture corpus =="
go run ./scripts/seed_fixture -corpus "$TMP/corpus" -artifacts "$TMP/artifacts"

echo "== start apid =="
./bin/apid \
    -addr :18091 \
    -corpus "$TMP/corpus" \
    -artifacts "$TMP/artifacts" \
    -sqlite "$TMP/db.sqlite" \
    -marrow http://127.0.0.1:1 \
    > "$TMP/apid.log" 2>&1 &
APID_PID=$!

for i in $(seq 1 50); do
    if curl -sf -o /dev/null "http://127.0.0.1:18091/api/tools/@foo/bar" 2>/dev/null; then
        break
    fi
    sleep 0.1
done
if ! curl -sf -o /dev/null "http://127.0.0.1:18091/api/tools/@foo/bar"; then
    echo "apid did not come up"
    cat "$TMP/apid.log"
    exit 1
fi

cd "$TMP"

echo "== install latest =="
"$ROOT/bin/inguma" install --api http://127.0.0.1:18091 @foo/bar -y \
    || { echo "install failed"; cat "$TMP/apid.log"; exit 1; }
grep -q '@foo/bar' inguma.lock
grep -q 'v1.1.0' inguma.lock
echo "  lockfile OK"

echo "== install --frozen succeeds =="
"$ROOT/bin/inguma" install --api http://127.0.0.1:18091 --frozen @foo/bar -y \
    || { echo "frozen install failed"; exit 1; }

echo "== upgrade is a no-op (already latest) =="
"$ROOT/bin/inguma" upgrade --api http://127.0.0.1:18091 @foo/bar 2>&1 | tee "$TMP/upgrade.log"
grep -q "up to date" "$TMP/upgrade.log"

echo ""
echo "TRACK-A-OK"
