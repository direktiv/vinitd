#!/bin/ash

echo "starting test"

export GOCACHE=/tmp

cp -Rf /build /app

cd / && make statik basedir=/app

cd /app && go test -v -coverprofile /c.out github.com/vorteil/vinitd/pkg/vorteil
