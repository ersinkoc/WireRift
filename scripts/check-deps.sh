#!/bin/bash
# check-deps.sh — Ensures zero external dependencies
# Exits with code 1 if go.sum has any entries

set -e

if [ -f "go.sum" ] && [ -s "go.sum" ]; then
    echo "ERROR: go.sum has entries. WireRift must have zero external dependencies."
    echo "Contents of go.sum:"
    cat go.sum
    exit 1
fi

echo "OK: No external dependencies detected."
exit 0
