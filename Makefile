MODULE := $(shell go list -m)

LINT_DIRS := entconv entcrud entproto testdata
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
	for adapter in $(TAG_ADAPTERS); do \
		git tag -s $$adapter/$(TAG) -m "$$adapter/$(TAG)"; \
	done
	git push origin --tags

.PHONY: tag-delete
tag-delete:
	@test -n "$(TAG)" || (echo "TAG is required: make tag-delete TAG=v0.0.1" && exit 1)
	-git tag -d $(TAG)
	@for adapter in $(TAG_ADAPTERS); do \
		git tag -d $$adapter/$(TAG) || true; \
	done
	-git push origin --delete $(TAG)
	@for adapter in $(TAG_ADAPTERS); do \
		git push origin --delete $$adapter/$(TAG) || true; \
	done
