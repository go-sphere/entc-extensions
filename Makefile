MODULE := $(shell go list -m)

LINT_DIRS := entconv entcrud entproto testdata
VERIFY_DIRS := entproto entconv entcrud
TAG_MODULES := entconv entcrud entproto

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
		nilaway -exclude-errors-in-files internal/pkg/database/ent/enttest/enttest.go ./...; \
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
	cd testdata && $(MAKE) verify

.PHONY: regen
regen:
	cd testdata && $(MAKE) generate

.PHONY: test
test:
	$(MAKE) regen
	$(MAKE) verify

.PHONY: tag
tag:
	@test -n "$(TAG)" || (echo "TAG is required: make tag TAG=v0.0.1" && exit 1)
	git tag -s $(TAG) -m "$(TAG)"
	git push origin --tags

.PHONY: tag-all
tag-all:
	@test -n "$(TAG)" || (echo "TAG is required: make tag-all TAG=v0.0.1" && exit 1)
	@set -e; \
	for tag in $(TAG) $(addsuffix /$(TAG),$(TAG_MODULES)); do \
		git tag -d "$$tag" >/dev/null 2>&1 || true; \
		git push origin --delete "$$tag" >/dev/null 2>&1 || true; \
		git tag -s "$$tag" -m "$$tag"; \
	done
	git push origin --tags

.PHONY: tag-delete
tag-delete:
	@test -n "$(TAG)" || (echo "TAG is required: make tag-delete TAG=v0.0.1" && exit 1)
	@for tag in $(TAG) $(addsuffix /$(TAG),$(TAG_MODULES)); do \
		git tag -d "$$tag" || true; \
	done
	@for tag in $(TAG) $(addsuffix /$(TAG),$(TAG_MODULES)); do \
		git push origin --delete "$$tag" || true; \
	done
