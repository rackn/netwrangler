#!/usr/bin/env bash
set -e
[[ $GOPATH ]] || export GOPATH="$HOME/go"
fgrep -q "$GOPATH/bin" <<< "$PATH" || export PATH="$PATH:$GOPATH/bin"
if ! which glide &>/dev/null; then
    go get -v github.com/Masterminds/glide
fi
glide i
(cd cmd; go build -o "$GOPATH/bin/netwrangler")
