.PHONY: init deps-update clean generate verify test

init:
	go install github.com/bufbuild/buf/cmd/buf@v1.59.0
	go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11

deps-update:
	buf dep update
	buf dep prune

clean:
	rm -rf api/entpb
	rm -rf proto/entpb
	rm -rf internal/pkg/database/ent
	rm -rf internal/pkg/render/entbind
	rm -rf internal/pkg/render/entmap

generate: clean
	@echo "[generate] entproto"
	go run ./cmd/entproto
	@echo "[generate] buf/protoc-gen-go"
	buf generate
	@echo "[generate] entconv"
	go run ./cmd/entconv
	@echo "[generate] entcrud"
	go run ./cmd/entcrud
	@echo "[generate] tidy testdata module"
	go mod tidy

verify:
	@echo "[verify] run all tests"
	go test ./...
	@echo "[verify] run e2e assertions"
	go test ./tests

test: generate verify
