#!/usr/bin/env bash
# Build cross-platform binaries and re-stage them for the commit.
set -euo pipefail

GOOS=darwin GOARCH=arm64 go build -o make-ics-macos-arm64 ./cmd/make-ics
GOOS=darwin GOARCH=amd64 go build -o make-ics-macos-amd64 ./cmd/make-ics
GOOS=windows GOARCH=amd64 go build -o make-ics.exe       ./cmd/make-ics

git add make-ics-macos-arm64 make-ics-macos-amd64 make-ics.exe
