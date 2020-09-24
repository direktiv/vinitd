#!/bin/ash

echo "starting test"

export GOCACHE=/tmp

cp -Rf /build /app

cd / && make statik basedir=/app

cd /app && go test -v -coverprofile /c.out github.com/vorteil/vinitd/pkg/vorteil

rm -Rf /usr/local/go
rm -Rf /go/pkg/mod/cache
rm -Rf build/
