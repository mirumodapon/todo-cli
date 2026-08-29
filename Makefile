# todo — a local task list
#
# Run "make" or "make help" to see the available targets.

BINARY := todo
CMD    := ./cmd/todo
BIN    := bin
GOBIN  ?= $(shell go env GOPATH)/bin

# Arguments forwarded by "make run", e.g. make run ARGS="ls -a"
ARGS ?=

.DEFAULT_GOAL := help

.PHONY: help build install run test cover fmt vet tidy check clean

help: ## Show this help
	@grep -hE '^[a-z-]+:.*## ' $(MAKEFILE_LIST) \
		| awk -F':.*## ' '{printf "  \033[1m%-8s\033[0m %s\n", $$1, $$2}'

build: ## Build the binary into bin/
	@mkdir -p $(BIN)
	go build -o $(BIN)/$(BINARY) $(CMD)

install: ## Install the binary into GOBIN
	go install $(CMD)

run: ## Run without installing, e.g. make run ARGS="ls -a"
	go run $(CMD) $(ARGS)

test: ## Run the test suite
	go test ./...

cover: ## Report test coverage and open it in a browser
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

fmt: ## Rewrite source files with gofmt
	gofmt -w .

vet: ## Run go vet
	go vet ./...

tidy: ## Tidy go.mod and go.sum
	go mod tidy

check: ## Verify formatting, vet, and tests — run this before committing
	@unformatted=$$(gofmt -l .); \
		if [ -n "$$unformatted" ]; then \
			echo "gofmt needed:"; echo "$$unformatted"; exit 1; \
		fi
	go vet ./...
	go test ./...

clean: ## Remove build output and coverage data
	rm -rf $(BIN) coverage.out
