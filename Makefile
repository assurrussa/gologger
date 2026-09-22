.DEFAULT_GOAL := check
.PHONY: check fmt vet lint test test-race

check: vet lint test-race

fmt:
	gofmt -w .

vet:
	go vet ./...

lint:
	golangci-lint run --timeout=5m ./...

test:
	go test ./...

test-race:
	go test -race -count=5 ./...
