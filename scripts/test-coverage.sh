#!/bin/bash
# test-coverage.sh — Runs tests with coverage and enforces minimum threshold

set -e

MIN_COVERAGE=${MIN_COVERAGE:-70}

echo "Running tests with coverage..."
go test -coverprofile=coverage.out -covermode=atomic ./...

COVERAGE=$(go tool cover -func=coverage.out | grep total | awk '{print $3}' | sed 's/%//')
echo "Total coverage: ${COVERAGE}%"

if (( $(echo "$COVERAGE < $MIN_COVERAGE" | bc -l) )); then
    echo "ERROR: Coverage ${COVERAGE}% is below minimum ${MIN_COVERAGE}%"
    exit 1
fi

echo "OK: Coverage ${COVERAGE}% meets minimum ${MIN_COVERAGE}%"
exit 0
