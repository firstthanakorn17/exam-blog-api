.PHONY: run build tidy db-up db-down db-logs

db-up:
	docker compose up -d

db-down:
	docker compose down

db-logs:
	docker compose logs -f

run:
	go run -ldflags="-linkmode=external" ./cmd/api

build:
	go build -ldflags="-linkmode=external" -o bin/exam-blog-api ./cmd/api

tidy:
	go mod tidy
