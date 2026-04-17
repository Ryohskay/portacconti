.PHONY: run build test migrate-up migrate-down seed-key

MIGRATION_DIR=internal/db/migrations
DB_URL?=$(DATABASE_URL)

## Run the server (requires .env or exported env vars)
run:
	go run ./cmd/api/...

## Build binary
build:
	go build -o bin/portacconti ./cmd/api/...

## Run tests
test:
	go test ./...

## Apply all migrations (requires migrate CLI: https://github.com/golang-migrate/migrate)
migrate-up:
	migrate -path $(MIGRATION_DIR) -database "$(DB_URL)" up

## Roll back last migration
migrate-down:
	migrate -path $(MIGRATION_DIR) -database "$(DB_URL)" down 1

## Generate a secure 32-byte AES key (use as ENCRYPTION_KEY)
seed-key:
	@openssl rand -hex 32

## Generate a JWT secret
jwt-secret:
	@openssl rand -hex 32
