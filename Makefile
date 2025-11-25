APP_NAME=pr-reviewer-service
BINARY=bin/pr-reviewer-service

.PHONY: build run tidy docker-build compose-up compose-down

build:
	go build -o $(BINARY) ./cmd/

run:
	go run ./cmd/

tidy:
	go mod tidy

docker-build:
	docker build -t $(APP_NAME) .

compose-up:
	docker-compose up --build

compose-down:
	docker-compose down -v