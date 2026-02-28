#!/usr/bin/env bash
# Build cross-platform binaries and re-stage them for the commit.
set -euo pipefail

GOOS=darwin  GOARCH=arm64 go build -o make-ics-macos ./cmd/make-ics
GOOS=windows GOARCH=amd64 go build -o make-ics.exe   ./cmd/make-ics

git add make-ics-macos make-ics.exe
