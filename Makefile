.PHONY: build test vet fmt lint dev test-e2e crawl-local crawler apid agentpop

GO := go
BIN := bin

build: crawler apid agentpop

dev:
	@scripts/dev.sh

test-e2e:
	@scripts/test-e2e.sh

crawl-local:
	$(GO) build -o $(BIN)/crawler ./cmd/crawler
	$(BIN)/crawler -registry registry/tools.yaml -corpus corpus -local internal/crawl/testdata/repos -skip-marrow

crawler:
	$(GO) build -o $(BIN)/crawler ./cmd/crawler

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

fmt:
	$(GO) fmt ./...

lint:
	golangci-lint run ./...

apid:
	$(GO) build -o $(BIN)/apid ./cmd/apid

agentpop:
	$(GO) build -o $(BIN)/agentpop ./cmd/agentpop
