# This file contains convenience targets for the OrchestAI project.
# It is not intended to be used as a build system.
# See the README for more information.

.PHONY: test
test:
	go test ./...

.PHONY: lint
lint: lint-deps
	golangci-lint run --color=always --sort-results ./...

.PHONY: lint-fix
lint-fix:
	golangci-lint run --fix ./...

.PHONY: lint-all
lint-all:
	golangci-lint run --color=always --sort-results ./...

.PHONY: lint-deps
lint-deps:
	@command -v golangci-lint >/dev/null 2>&1 || { \
		echo >&2 "golangci-lint not found. Installing..."; \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.61.0; \
	}

.PHONY: test-race
test-race:
	go test -race ./...

.PHONY: test-cover
test-cover:
	go test -cover ./...

.PHONY: clean
clean: clean-lint-cache

.PHONY: clean-lint-cache
clean-lint-cache:
	golangci-lint cache clean

.PHONY: build
build:
	go build -v ./...

.PHONY: build-examples
build-examples:
	for example in $(shell find ./cmd/examples -mindepth 1 -maxdepth 1 -type d); do \
		(cd $$example; echo Build $$example; go mod tidy; go build -o /dev/null) || exit 1; done

.PHONY: mod-tidy
mod-tidy:
	go mod tidy

.PHONY: add-go-work
add-go-work:
	go work init .
	go work use -r .

.PHONY: check
check: lint test build