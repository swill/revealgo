#!/usr/bin/env bash

cd assets/revealjs/ && grunt sass && cd ../..
go-bindata -pkg revealgo -o assets.go assets/revealjs/lib/... assets/revealjs/plugin/... assets/revealjs/css/... assets/revealjs/js/... assets/templates/...
cd cmd/revealgo/ && gox -output="../../bin/{{.Dir}}_{{.OS}}_{{.Arch}}" -osarch="linux/amd64 darwin/amd64 windows/amd64" && cd ../..
