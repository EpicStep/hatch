GOLANGCI_LINT_VERSION = 2.11.4

.PHONY: test lint lint-fix fmt

test:
	go test ./...

lint:
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v$(GOLANGCI_LINT_VERSION) run

lint-fix:
	golangci-lint run --fix ./...

fmt:
	golangci-lint fmt ./...
