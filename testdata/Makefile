.PHONY: init test

init:
	go install github.com/bufbuild/buf/cmd/buf@latest
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	buf dep update
	buf dep prune


test:
	go run ./cmd/entproto
	buf generate
	go run ./cmd/entconv
	go run ./cmd/entcrud
	go run ./cmd/test
