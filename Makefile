.PHONY: tidy build run docker-build docker-up docker-down smoke-test

tidy:
	go mod tidy

build:
	go build ./cmd/vff-remnawave-events
	go build ./cmd/send-test-webhook

run:
	LISTEN_ADDR=:8080 \
	WEBHOOK_SECRET_HEADER=change-me \
	SQLITE_PATH=./data/events.db \
	TELEGRAM_BOT_TOKEN=change-me \
	go run ./cmd/vff-remnawave-events

docker-build:
	docker build -t vff-remnawave-events:local .

docker-up:
	docker compose up -d --build

docker-down:
	docker compose down

smoke-test:
	go run ./cmd/send-test-webhook --secret change-me
