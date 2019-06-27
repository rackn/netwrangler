#!/usr/bin/env bash
set -e
# Work out the GO version we are working with:
GO_VERSION=$(go version | awk '{ print $3 }' | sed 's/go//')
WANTED_VER=(1 12)
if ! [[ "$GO_VERSION" =~ ([0-9]+)\.([0-9]+) ]]; then
    echo "Cannot figure out what version of Go is installed"
    exit 1
elif ! (( ${BASH_REMATCH[1]} > ${WANTED_VER[0]} || ${BASH_REMATCH[2]} >= ${WANTED_VER[1]} )); then
    echo "Go Version needs to be ${WANTED_VER[0]}.${WANTED_VER[1]} or higher: currently $GO_VERSION"
    exit -1
fi
[[ $GOPATH ]] || export GOPATH="$HOME/go"
fgrep -q "$GOPATH/bin" <<< "$PATH" || export PATH="$PATH:$GOPATH/bin"
if ! which glide &>/dev/null; then
    go get -v github.com/Masterminds/glide
fi
glide i
(cd cmd; go build -o netwrangler)
