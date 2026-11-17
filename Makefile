GO ?= go
GOLANGCI_LINT ?= golangci-lint
NILAWAY ?= nilaway

GO_MOD_DIRS := . entconv entcrud entproto testdata
GO_PACKAGE_DIRS := entconv entcrud entproto testdata
VERIFY_DIRS := entproto entconv entcrud
TAG_MODULES := entconv entcrud entproto
DIRECT_DEPS_TEMPLATE := {{if and (not .Main) (not .Indirect) (not .Replace)}}{{.Path}}{{end}}

# Resolve go-sphere modules straight from GitHub, bypassing the module proxy
# and its cached "@latest", which lags behind freshly pushed tags.
DIRECT_ORIGIN := GOPRIVATE=github.com/go-sphere/*
FORCE ?= 0

.DEFAULT_GOAL := check

.PHONY: deps-update tidy tidy-check fmt build test lint lint-all check verify regen update-golden
.PHONY: tag tag-all tag-delete

deps-update:
	@set -eu; \
	for dir in $(GO_MOD_DIRS); do \
		echo "==> updating $$dir"; \
		( cd "$$dir"; \
		  GOWORK=off $(DIRECT_ORIGIN) $(GO) mod tidy; \
		  deps="$$(GOWORK=off $(DIRECT_ORIGIN) $(GO) list -m -f '$(DIRECT_DEPS_TEMPLATE)' all)"; \
		  if [ -n "$$deps" ]; then GOWORK=off $(DIRECT_ORIGIN) $(GO) get -u $$deps; fi; \
		  GOWORK=off $(DIRECT_ORIGIN) $(GO) mod tidy ); \
	done

tidy:
	@set -eu; \
	for dir in $(GO_MOD_DIRS); do \
		echo "==> tidying $$dir"; \
		( cd "$$dir" && GOWORK=off $(GO) mod tidy ); \
	done

# Non-mutating counterpart of tidy, for CI: fails if any module's go.mod/go.sum
# is not what a consumer would resolve. testdata's go.mod tracks the code the
# generators write there, so CI runs it after the regeneration step.
tidy-check:
	@set -eu; \
	for dir in $(GO_MOD_DIRS); do \
		echo "==> checking dependencies in $$dir"; \
		( cd "$$dir" && GOWORK=off $(GO) mod tidy -diff ); \
	done

fmt:
	@set -eu; \
	for dir in $(GO_PACKAGE_DIRS); do \
		echo "==> formatting $$dir"; \
		( cd "$$dir" && $(GO) fmt ./... && \
		  $(GOLANGCI_LINT) fmt --no-config --enable gofmt --enable goimports ); \
	done

# -o /dev/null: testdata holds main packages (the generators), and compiling
# them must not drop binaries into the tree.
build:
	@set -eu; \
	for dir in $(GO_PACKAGE_DIRS); do \
		echo "==> building $$dir"; \
		( cd "$$dir" && $(GO) build -o /dev/null ./... ); \
	done

test: regen verify

# nilaway exclusions are ent-generated files only: enttest.go, and user/where.go,
# whose GoType In/NotIn predicates index a variadic slice inside a loop bounded by
# its length, which nilaway cannot prove safe.
NILAWAY_EXCLUDE := internal/pkg/database/ent/enttest/enttest.go,internal/pkg/database/ent/user/where.go

lint:
	@set -eu; \
	for dir in $(GO_PACKAGE_DIRS); do \
		echo "==> linting $$dir"; \
		( cd "$$dir"; \
		  $(GOLANGCI_LINT) fmt --no-config --enable gofmt --enable goimports --diff; \
		  $(GO) vet ./...; \
		  $(GOLANGCI_LINT) run --no-config; \
		  $(NILAWAY) -exclude-errors-in-files $(NILAWAY_EXCLUDE) ./... ); \
	done

# Backward-compatible alias.
lint-all: lint

check: tidy-check
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

update-golden: regen
	$(MAKE) -C testdata update-golden

tag:
	@test -n "$(TAG)" || { echo "TAG is required: make tag TAG=v0.0.1"; exit 1; }
	@set -eu; \
	allow_force=0; \
	if [ "$(FORCE)" = "1" ]; then \
		case "$(TAG)" in \
			*-*) allow_force=1 ;; \
			*) echo "FORCE=1 is only allowed for pre-release tags containing '-' (got $(TAG))" >&2; exit 1 ;; \
		esac; \
	fi; \
	local_exists=0; \
	if git show-ref --verify --quiet "refs/tags/$(TAG)"; then local_exists=1; fi; \
	remote_ref="$$(git ls-remote --tags origin "refs/tags/$(TAG)")"; \
	if [ "$$local_exists" -eq 1 ] || [ -n "$$remote_ref" ]; then \
		if [ "$$allow_force" -ne 1 ]; then \
			echo "tag $(TAG) already exists; pass FORCE=1 only for pre-release tags containing '-'" >&2; \
			exit 1; \
		fi; \
		if [ "$$local_exists" -eq 1 ]; then git tag -d "$(TAG)"; fi; \
		if [ -n "$$remote_ref" ]; then git push origin --delete "refs/tags/$(TAG)"; fi; \
	fi; \
	git tag -s "$(TAG)" -m "$(TAG)"; \
	git push origin "refs/tags/$(TAG)"

tag-all:
	@test -n "$(TAG)" || { echo "TAG is required: make tag-all TAG=v0.0.1"; exit 1; }
	@set -eu; \
	allow_force=0; \
	if [ "$(FORCE)" = "1" ]; then \
		case "$(TAG)" in \
			*-*) allow_force=1 ;; \
			*) echo "FORCE=1 is only allowed for pre-release tags containing '-' (got $(TAG))" >&2; exit 1 ;; \
		esac; \
	fi; \
	tags="$(TAG) $(addsuffix /$(TAG),$(TAG_MODULES))"; \
	to_delete_local=""; \
	to_delete_remote=""; \
	for tag in $$tags; do \
		local_exists=0; \
		if git show-ref --verify --quiet "refs/tags/$$tag"; then local_exists=1; fi; \
		remote_ref="$$(git ls-remote --tags origin "refs/tags/$$tag")"; \
		if [ "$$local_exists" -eq 1 ] || [ -n "$$remote_ref" ]; then \
			if [ "$$allow_force" -ne 1 ]; then \
				echo "tag $$tag already exists; pass FORCE=1 only for pre-release tags containing '-'" >&2; \
				exit 1; \
			fi; \
			if [ "$$local_exists" -eq 1 ]; then to_delete_local="$$to_delete_local $$tag"; fi; \
			if [ -n "$$remote_ref" ]; then to_delete_remote="$$to_delete_remote refs/tags/$$tag"; fi; \
		fi; \
	done; \
	for tag in $$to_delete_local; do git tag -d "$$tag"; done; \
	if [ -n "$$to_delete_remote" ]; then git push origin --delete $$to_delete_remote; fi; \
	refs=""; \
	for tag in $$tags; do \
		git tag -s "$$tag" -m "$$tag"; \
		refs="$$refs refs/tags/$$tag"; \
	done; \
	git push origin $$refs

tag-delete:
	@test -n "$(TAG)" || { echo "TAG is required: make tag-delete TAG=v0.0.1"; exit 1; }
	@for tag in $(TAG) $(addsuffix /$(TAG),$(TAG_MODULES)); do git tag -d "$$tag" || true; done
	@for tag in $(TAG) $(addsuffix /$(TAG),$(TAG_MODULES)); do git push origin --delete "$$tag" || true; done
