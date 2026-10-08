.PHONY: build test

build:
	go build -o bin/mia-plan ./cmd/mia-plan

test: build
	go test ./...
	scripts/loop.sh
