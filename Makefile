MODULE := $(shell go list -m)

LINT_DIRS := entconv entcrud entproto testdata

.PHONY: lint-all
lint-all:
	@set -e; \
	for dir in $(LINT_DIRS); do \
		cd $$dir; \
		go fix ./...; \
		go fmt ./...; \
		go vet ./...; \
		go get ./...; \
		go test ./...; \
		go mod tidy; \
		golangci-lint fmt --no-config --enable gofmt,goimports; \
		golangci-lint run --no-config --fix; \
		nilaway ./...; \
		cd - >/dev/null; \
	done

.PHONY: test
test:
	cd testdata && make test