.PHONY: build test vet fmt crawler

GO := go
BIN := bin

build: crawler

crawler:
	$(GO) build -o $(BIN)/crawler ./cmd/crawler

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

fmt:
	$(GO) fmt ./...
