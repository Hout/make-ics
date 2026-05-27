#!/usr/bin/env bash
# Build cross-platform binaries and re-stage them for the commit.
set -euo pipefail

GOOS=darwin  GOARCH=arm64 go build -o import-checks-report-macos ./cmd/import-checks-report
GOOS=windows GOARCH=amd64 go build -o import-checks-report.exe   ./cmd/import-checks-report
GOOS=darwin  GOARCH=arm64 go build -o list-shifts-macos          ./cmd/list-shifts
GOOS=windows GOARCH=amd64 go build -o list-shifts.exe            ./cmd/list-shifts

git add import-checks-report-macos import-checks-report.exe list-shifts-macos list-shifts.exe
