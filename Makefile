MODULE := $(shell go list -m)

LINT_DIRS := entconv entcrud entproto testdata
NILAWAY_DIRS := entconv entcrud entproto
VERIFY_DIRS := entproto entconv entcrud

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
		if echo " $(NILAWAY_DIRS) " | grep -q " $$dir "; then \
			nilaway ./...; \
		fi; \
		cd - >/dev/null; \
	done

.PHONY: verify
verify:
	@set -e; \
	for dir in $(VERIFY_DIRS); do \
		cd $$dir; \
		go test ./...; \
		cd - >/dev/null; \
	done; \
	cd testdata && $(MAKE) verify-generated

.PHONY: regen
regen:
	cd testdata && $(MAKE) generate-all

.PHONY: test
test:
	$(MAKE) regen
	$(MAKE) verify
