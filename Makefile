.PHONY: build test

build:
	go build -o bin/mia-plan ./cmd/mia-plan
	go build -o bin/mia-dev ./cmd/mia-dev

test: build
	go test ./...
	scripts/loop.sh
