.PHONY: init clean-generated generate-all verify-generated test

init:
	go install github.com/bufbuild/buf/cmd/buf@latest
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	buf dep update
	buf dep prune

clean-generated:
	rm -r internal/pkg/database/ent/*
	rm -r internal/pkg/render/*

generate-all: clean-generated
	go run ./cmd/entproto
	buf generate
	go run ./cmd/entconv
	go run ./cmd/entcrud

verify-generated:
	go test ./...
	go run ./cmd/test

test: generate-all verify-generated
