#!/bin/ash

echo "installing dependencies"

apk update
apk add ca-certificates
apk add make
apk add git
apk add gcc
apk add linux-headers
apk add libc-dev

export GOCACHE=/tmp

cd .. && make prep basedir=/app

# run test once to get all dependencies
cd /app && ls && go test -v -coverprofile c.out github.com/vorteil/vinitd/pkg/vorteil