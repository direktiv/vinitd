#!/bin/ash

/run_prep.sh
/run_tests.sh

# clean up for smaller export
rm -Rf /usr/local/go
rm -Rf /go/pkg/mod/cache
rm -Rf /build/
rm -Rf /usr
rm -Rf /app
