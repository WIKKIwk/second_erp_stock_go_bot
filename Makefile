APP := ./cmd/bot

.PHONY: run test tidy fmt

run:
	@set -a; [ -f .env ] && . ./.env; set +a; go run $(APP)

test:
	@go test ./...

tidy:
	@go mod tidy

fmt:
	@gofmt -w $$(find . -name '*.go' -type f)

