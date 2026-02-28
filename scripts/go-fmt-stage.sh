#!/bin/sh
# Run gofmt in-place and re-stage any files it modified, so the formatted
# version is included in the commit rather than requiring a follow-up commit.
gofmt -w "$@"
for f in "$@"; do
    if ! git diff --quiet "$f" 2>/dev/null; then
        git add "$f"
    fi
done
