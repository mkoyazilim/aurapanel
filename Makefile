# AuraPanel build/test hedefleri.
# CGO_ENABLED=0 zorunludur (ARCHITECTURE §2: CGO yasak).

BINARY    := aurapanel
VERSION   ?= dev
COMMIT    := $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
BUILDTIME := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -s -w \
	-X main.version=$(VERSION) \
	-X main.commit=$(COMMIT) \
	-X main.built=$(BUILDTIME)

.PHONY: build test vet run clean web

build:
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/aurapanel

test:
	CGO_ENABLED=0 go test ./...

vet:
	go vet ./...

run:
	go run ./cmd/aurapanel -config configs/aurapanel.example.yaml

# Web arayüzü: dist/ go:embed ile binary'ye gömülür — build'den ÖNCE çalıştırılmalı.
web:
	cd web && npm install --no-audit --no-fund && npm run build

clean:
	rm -rf bin
