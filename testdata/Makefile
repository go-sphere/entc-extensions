.PHONY: init test

init:
	buf dep update
	buf dep prune


test:
	go run ./cmd/entproto
	buf generate
	go run ./cmd/entconv
	go run ./cmd/entcrud
	go run ./cmd/test
