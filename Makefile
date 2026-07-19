GOCACHE ?= /tmp/whyfi-go-cache

.PHONY: fmt test race vet web-format web-lint web-test web-build web-portfolio web-e2e build build-linux build-windows load-build run serve demo

fmt:
	gofmt -w cmd internal

test:
	GOCACHE=$(GOCACHE) go test ./...

race:
	GOCACHE=$(GOCACHE) go test -race ./...

vet:
	GOCACHE=$(GOCACHE) go vet ./...

web-format:
	cd web && npm run format

web-lint:
	cd web && npm run lint

web-test:
	cd web && npm test

web-build:
	cd web && npm run build

web-portfolio:
	cd web && npm run build:portfolio

web-e2e: web-build build
	cd web && npm run test:e2e

build:
	mkdir -p bin
	GOCACHE=$(GOCACHE) go build -o bin/routecause ./cmd/whyfi

build-linux:
	mkdir -p bin
	GOCACHE=$(GOCACHE) GOOS=linux GOARCH=amd64 go build -o bin/routecause-linux-amd64 ./cmd/whyfi

build-windows:
	mkdir -p bin
	GOCACHE=$(GOCACHE) GOOS=windows GOARCH=amd64 go build -o bin/routecause-windows-amd64.exe ./cmd/whyfi

load-build:
	mkdir -p bin
	GOCACHE=$(GOCACHE) go build -o bin/routecause-load ./cmd/routecause-load

run: build
	./bin/routecause diagnose --duration 15s --output session.json

serve: build
	./bin/routecause serve

demo: build
	./bin/routecause serve --demo
