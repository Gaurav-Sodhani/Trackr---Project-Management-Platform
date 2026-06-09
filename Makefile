.PHONY: run build test swagger seed docker-up docker-down

run:
	go run ./cmd/server

build:
	go build -o bin/server ./cmd/server

test:
	go test ./... -v

swagger:
	swag init -g cmd/server/main.go -o docs

seed:
	go run ./seed

docker-up:
	docker-compose up --build -d

docker-down:
	docker-compose down -v

lint:
	golangci-lint run ./...
