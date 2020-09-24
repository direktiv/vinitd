#!/bin/ash

/run_prep.sh
/run_tests.sh

# clean up for smaller export
echo "deleting files"
rm -Rf /usr/local/go
rm -Rf /go/pkg/mod/cache
rm -Rf /build/
rm -Rf /usr
rm -Rf /app
rm -Rf /tmp
echo "done"
