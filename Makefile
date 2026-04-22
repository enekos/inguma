.PHONY: build test vet fmt crawler apid agentpop

GO := go
BIN := bin

build: crawler apid agentpop

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

agentpop:
	$(GO) build -o $(BIN)/agentpop ./cmd/agentpop
