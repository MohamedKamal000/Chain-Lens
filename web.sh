#!/usr/bin/env bash
set -euo pipefail

###############################################################################
# web.sh — Bitcoin transaction web visualizer
#
# Starts the web visualizer server.
#
# Behavior:
#   - Reads PORT env var (default: 3000)
#   - Prints the URL (e.g., http://127.0.0.1:3000) to stdout
#   - Keeps running until terminated (CTRL+C / SIGTERM)
#   - Must serve GET /api/health -> 200 { "ok": true }
###############################################################################

PORT="${PORT:-3000}"


cd "$(dirname "$0")/analyzer"

# Build the analyzer if needed
if [ ! -f chainlens ]; then
  go build -o chainlens .
fi

echo "http://127.0.0.1:$PORT"
# Start Go server in background, suppress output
./chainlens -server -port "8080" > /dev/null 2>&1 &
# Start React dev server in background, suppress output
cd WebVisualizer
nohup npm run dev > /dev/null 2>&1 &
# Keep terminal alive until CTRL+C
while true; do sleep 1; done