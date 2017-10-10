#!/usr/bin/env bash

cd assets/revealjs/ && grunt sass && cd ../..
go-bindata -pkg revealgo -o assets.go assets/revealjs/lib/... assets/revealjs/plugin/... assets/revealjs/css/... assets/revealjs/js/... assets/templates/...
cd cmd/revealgo/ && go install && cd ../..
