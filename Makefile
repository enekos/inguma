.PHONY: build test vet fmt crawler apid

GO := go
BIN := bin

build: crawler apid

crawler:
	$(GO) build -o $(BIN)/crawler ./cmd/crawler

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

fmt:
	$(GO) fmt ./...

apid:
	$(GO) build -o $(BIN)/apid ./cmd/apid
