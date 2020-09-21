#!/bin/ash

echo "starting test"

export GOCACHE=/tmp

go test -coverprofile c.out ./...
