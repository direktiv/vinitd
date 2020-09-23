#!/bin/ash

echo "starting test"

export GOCACHE=/tmp

cp -Rf /build /app

echo "starting test1"
cd / && make statik basedir=/app
echo "starting test2"
cd /app && go test -v -coverprofile /c.out github.com/vorteil/vinitd/pkg/vorteil
echo "starting test3"
