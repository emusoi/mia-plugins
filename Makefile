.PHONY: build test

build:
	go build -o bin/mia-plan ./cmd/mia-plan
	go build -o bin/mia-agent ./cmd/mia-agent

test: build
	go test ./...
	scripts/loop.sh
