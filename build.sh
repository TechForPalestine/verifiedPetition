#!/usr/bin/env bash

repo_root=$(git rev-parse --show-toplevel)
go mod vendor

set -x
docker volume create --name go-build-cache
docker run -it --rm \
  -v "$repo_root":/opt/src \
  -v go-build-cache:/root/.cache/go-build \
  -w /opt/src \
  --platform="linux/amd64" \
  -e CGO_ENABLED=1 \
  golang:1.22.0 \
  go build -v -o build/verifiedPetition cmd/verifiedPetition/main.go