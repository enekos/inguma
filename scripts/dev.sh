#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

# Build apid if needed
if [ ! -f bin/apid ] || [ cmd/apid/main.go -nt bin/apid ]; then
	make apid
fi

# Start apid with testdata corpus in the background
echo "Starting apid on :8091 with testdata corpus..."
bin/apid -addr :8091 -corpus internal/api/testdata/corpus &
APID_PID=$!

# Kill apid on exit
CLEANED_UP=0
cleanup() {
	if [ "$CLEANED_UP" -eq 0 ]; then
		CLEANED_UP=1
		echo "Shutting down apid..."
		kill "$APID_PID" 2>/dev/null || true
		wait "$APID_PID" 2>/dev/null || true
	fi
}
trap cleanup EXIT INT TERM

# Give apid a moment to bind
sleep 0.5

# Start frontend dev server
echo "Starting frontend dev server..."
cd web
npm run dev
