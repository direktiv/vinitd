#!/bin/ash

echo "starting test prep"

export GOCACHE=/home

cp -Rf /build /app

cd / && make statik basedir=/app

echo "starting test"

cd /app && go test -v -coverprofile /c.out github.com/vorteil/vinitd/pkg/vorteil
