GO ?= go
GOLANGCI_LINT ?= golangci-lint
NILAWAY ?= nilaway

GO_MOD_DIRS := . entconv entcrud entproto testdata
GO_PACKAGE_DIRS := entconv entcrud entproto testdata
VERIFY_DIRS := entproto entconv entcrud
TAG_MODULES := entconv entcrud entproto
DIRECT_DEPS_TEMPLATE := {{if and (not .Main) (not .Indirect) (not .Replace)}}{{.Path}}{{end}}

.DEFAULT_GOAL := check

.PHONY: deps-update tidy fmt test lint lint-all check verify regen
.PHONY: tag tag-all tag-delete

deps-update:
	@set -eu; \
	for dir in $(GO_MOD_DIRS); do \
		echo "==> updating $$dir"; \
		( cd "$$dir"; \
		  deps="$$(GOWORK=off $(GO) list -m -f '$(DIRECT_DEPS_TEMPLATE)' all)"; \
		  if [ -n "$$deps" ]; then GOWORK=off $(GO) get -u $$deps; fi; \
		  GOWORK=off $(GO) mod tidy ); \
	done

tidy:
	@set -eu; \
	for dir in $(GO_MOD_DIRS); do \
		echo "==> tidying $$dir"; \
		( cd "$$dir" && GOWORK=off $(GO) mod tidy ); \
	done

fmt:
	@set -eu; \
	for dir in $(GO_PACKAGE_DIRS); do \
		echo "==> formatting $$dir"; \
		( cd "$$dir" && $(GO) fmt ./... && \
		  $(GOLANGCI_LINT) fmt --no-config --enable gofmt --enable goimports ); \
	done

test: regen verify

lint:
	@set -eu; \
	for dir in $(GO_PACKAGE_DIRS); do \
		echo "==> linting $$dir"; \
		( cd "$$dir"; \
		  $(GOLANGCI_LINT) fmt --no-config --enable gofmt --enable goimports --diff; \
		  $(GO) vet ./...; \
		  $(GOLANGCI_LINT) run --no-config; \
		  $(NILAWAY) -exclude-errors-in-files internal/pkg/database/ent/enttest/enttest.go ./... ); \
	done

# Backward-compatible alias.
lint-all: lint

check:
	@set -eu; \
	for dir in $(GO_MOD_DIRS); do \
		echo "==> checking dependencies in $$dir"; \
		( cd "$$dir" && GOWORK=off $(GO) mod tidy -diff ); \
	done
	$(MAKE) lint
	$(MAKE) test

verify:
	@set -eu; \
	for dir in $(VERIFY_DIRS); do \
		echo "==> testing $$dir"; \
		( cd "$$dir" && $(GO) test ./... ); \
	done
	$(MAKE) -C testdata verify

regen:
	$(MAKE) -C testdata generate

tag:
	@test -n "$(TAG)" || { echo "TAG is required: make tag TAG=v0.0.1"; exit 1; }
	git tag -s $(TAG) -m "$(TAG)"
	git push origin --tags

tag-all:
	@test -n "$(TAG)" || { echo "TAG is required: make tag-all TAG=v0.0.1"; exit 1; }
	@set -eu; \
	for tag in $(TAG) $(addsuffix /$(TAG),$(TAG_MODULES)); do \
		git tag -d "$$tag" >/dev/null 2>&1 || true; \
		git push origin --delete "$$tag" >/dev/null 2>&1 || true; \
		git tag -s "$$tag" -m "$$tag"; \
	done
	git push origin --tags

tag-delete:
	@test -n "$(TAG)" || { echo "TAG is required: make tag-delete TAG=v0.0.1"; exit 1; }
	@for tag in $(TAG) $(addsuffix /$(TAG),$(TAG_MODULES)); do git tag -d "$$tag" || true; done
	@for tag in $(TAG) $(addsuffix /$(TAG),$(TAG_MODULES)); do git push origin --delete "$$tag" || true; done
