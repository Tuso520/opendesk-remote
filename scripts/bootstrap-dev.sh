#!/usr/bin/env sh
set -eu
cd "$(dirname "$0")/../api"
cp -n .env.example .env || true
go mod tidy
go test ./...

