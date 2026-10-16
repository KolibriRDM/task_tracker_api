export PATH := $(shell go env GOPATH)/bin:$(PATH)
.PHONY: run client generate test build
run:
	go run ./cmd/server
client:
	go run ./cmd/client
generate:
	buf generate --path proto/task
test:
	go test -race ./...
build:
	go build ./...
