#!/bin/ash

echo "installing dependencies"

apk update
apk add ca-certificates
apk add make
apk add git
apk add gcc
apk add linux-headers
apk add libc-dev

export GOCACHE=/home

cd .. && make prep BASEDIR=/app

# run test once to get all dependencies
cd /app && go test -v -coverprofile c.out github.com/vorteil/vinitd/pkg/vorteil
