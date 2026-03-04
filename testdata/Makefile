.PHONY: init clean-generated generate-entproto generate-entconv generate-entcrud generate-all verify-entproto verify-entconv verify-entcrud verify-e2e verify-generated test

init:
	go install github.com/bufbuild/buf/cmd/buf@v1.59.0
	go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11

.PHONY: deps-update
deps-update:
	buf dep update
	buf dep prune

clean-generated:
	rm -rf api/entpb
	rm -rf proto/entpb
	rm -rf internal/pkg/database/ent
	rm -rf internal/pkg/render/entbind
	rm -rf internal/pkg/render/entmap

generate-entproto:
	@echo "[entproto] generate ent + proto definitions"
	go run ./cmd/entproto
	@echo "[entproto] generate pb.go from proto"
	buf generate

generate-entconv:
	@echo "[entconv] generate ent <-> proto converters"
	go run ./cmd/entconv

generate-entcrud:
	@echo "[entcrud] generate bind helpers"
	go run ./cmd/entcrud

generate-all: clean-generated generate-entproto generate-entconv generate-entcrud

verify-entproto:
	@echo "[verify][entproto] compile generated ent/proto artifacts"
	go test ./api/entpb ./internal/pkg/database/schema ./internal/pkg/database/ent/...

verify-entconv:
	@echo "[verify][entconv] compile generated converter package"
	go test ./internal/pkg/render/entmap

verify-entcrud:
	@echo "[verify][entcrud] compile generated bind package"
	go test ./internal/pkg/render/entbind

verify-e2e:
	@echo "[verify][e2e] run integration assertions"
	go test ./tests

verify-generated:
	$(MAKE) verify-entproto
	$(MAKE) verify-entconv
	$(MAKE) verify-entcrud
	$(MAKE) verify-e2e
	go test ./...

test: generate-all verify-generated
