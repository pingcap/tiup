#!/usr/bin/env bash

echo "generate code by proto..."
#GOGO_ROOT=${GOPATH}/src/github.com/gogo/protobuf
GOGO_ROOT=${GOPATH}/pkg/mod/github.com/gogo/protobuf@v1.3.2
protoc -I.:${GOGO_ROOT}:${GOGO_ROOT}/protobuf --gofast_out=./ report.proto
sed -i.bak -E 's/import _ \"gogoproto\"//g' *.pb.go
sed -i.bak -E 's/import fmt \"fmt\"//g' *.pb.go
rm -f *.bak
goimports -w *.pb.go
