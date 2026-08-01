# kizashi — Makefile
#
# Targets principais:
#   make            build do binário em bin/kizashi
#   make install    build + instala kizashi no PATH
#   make test       testes unitários
#   make check      vet + testes + testes com race detector
#   make run        roda o TUI com os dados de exemplo (sample/demo.jsonl)

GO      ?= go
PKG     := ./cmd/kizashi
BIN_DIR := bin
BIN     := $(BIN_DIR)/kizashi
VERSION ?= dev

GOBIN ?= $(shell $(GO) env GOBIN)
ifeq ($(GOBIN),)
GOBIN := $(shell $(GO) env GOPATH)/bin
endif

.PHONY: all build install release test test-race check vet fmt run clean help

all: build

build:
	$(GO) build -o $(BIN) $(PKG)

install: build
	$(GO) install $(PKG)

# build com versão embutida (ldflags):
#   make release VERSION=1.0.0
release:
	$(GO) build -ldflags "-X kizashi/internal/cli.version=$(VERSION)" -o $(BIN) $(PKG)

test:
	$(GO) test ./...

test-race:
	$(GO) test -race ./...

# validação completa (equivalente ao comando do README)
check: vet test test-race

vet:
	$(GO) vet ./...

fmt:
	$(GO) fmt ./...

run: build
	cat sample/demo.jsonl | $(BIN) stream

clean:
	rm -rf $(BIN_DIR)

help:
	@echo "kizashi — targets disponíveis:"
	@echo "  make            build do binário em $(BIN)"
	@echo "  make install    build + instala 'kizashi' no PATH ($(GOBIN))"
	@echo "  make release    build com versão embutida (VERSION=<x>)"
	@echo "  make test       testes unitários"
	@echo "  make test-race  testes com race detector"
	@echo "  make check      vet + test + test-race"
	@echo "  make vet        go vet ./..."
	@echo "  make fmt        gofmt em todos os pacotes"
	@echo "  make run        roda o TUI com sample/demo.jsonl"
	@echo "  make clean      remove $(BIN_DIR)"
