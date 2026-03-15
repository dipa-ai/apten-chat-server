.PHONY: run migrate-up migrate-down migrate-create sqlc-generate test docker-up docker-down

run:
	go run ./cmd/server

migrate-up:
	goose -dir migrations postgres "$$DATABASE_URL" up

migrate-down:
	goose -dir migrations postgres "$$DATABASE_URL" down

migrate-create:
	goose -dir migrations create $(NAME) sql

sqlc-generate:
	sqlc generate

test:
	go test ./...

docker-up:
	docker compose up -d

docker-down:
	docker compose down
