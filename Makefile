.PHONY: run build tidy

run:
	go run -ldflags="-linkmode=external" ./cmd/api

build:
	go build -ldflags="-linkmode=external" -o bin/exam-blog-api ./cmd/api

tidy:
	go mod tidy
