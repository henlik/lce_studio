.PHONY: install db dev build test check

install:
	go mod download
	npm --prefix web ci

db:
	docker compose up -d --wait postgres

dev:
	npm run dev

build:
	npm --prefix web run build
	go build -trimpath -o bin/studio ./cmd/studio

test:
	go test -race -count=1 ./...

check:
	go vet ./...
	npm --prefix web run typecheck
