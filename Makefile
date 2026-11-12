.PHONY: install dev build test check

install:
	go mod download
	npm --prefix web ci

dev:
	npm run dev

build:
	npm --prefix web run build
	go build -trimpath -o bin/studio ./cmd/studio

test:
	go test -race ./...

check:
	go vet ./...
	npm --prefix web run typecheck
