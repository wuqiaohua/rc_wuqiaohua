.PHONY: test run docker-up docker-down

test:
	go test ./...

run:
	go run ./cmd/server

docker-up:
	docker compose up --build

docker-down:
	docker compose down -v
