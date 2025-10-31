#!/bin/sh

set -ex

export PATH=$PATH:/usr/local/go/bin
export GOPATH=$HOME/go
export PATH=$GOPATH/bin:$PATH
export GOBIN=$GOPATH/bin

go test -p 1 -tags=integration ./... -v

