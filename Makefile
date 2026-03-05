APP := ./cmd/bot
BIN_DIR := ./bin
BIN := $(BIN_DIR)/erpnext-bot

.PHONY: run stop build test tidy fmt

build:
	@mkdir -p $(BIN_DIR)
	@go build -o $(BIN) $(APP)

stop:
	@set -a; [ -f .env ] && . ./.env; set +a; \
	pids_bin=$$(pgrep -x -f "$(CURDIR)/bin/erpnext-bot" || true); \
	pids_go=$$(pgrep -x -f "go run ./cmd/bot" || true); \
	pids_token=""; \
	if [ -n "$$TELEGRAM_BOT_TOKEN" ]; then \
		for pid in $$(pgrep -u "$$(id -u)" || true); do \
			[ "$$pid" = "$$" ] && continue; \
			[ "$$pid" = "$$PPID" ] && continue; \
			env_file="/proc/$$pid/environ"; \
			[ -r "$$env_file" ] || continue; \
			if grep -zqx "TELEGRAM_BOT_TOKEN=$$TELEGRAM_BOT_TOKEN" "$$env_file" 2>/dev/null; then \
				pids_token="$$pids_token $$pid"; \
			fi; \
		done; \
	fi; \
	pids=$$(printf "%s\n%s\n%s\n" "$$pids_bin" "$$pids_go" "$$pids_token" | tr ' ' '\n' | awk 'NF' | sort -u | paste -sd' ' -); \
	if [ -n "$$pids" ]; then \
		echo "Stopping existing bot process(es): $$pids"; \
		kill $$pids 2>/dev/null || true; \
		sleep 1; \
		alive=$$(for pid in $$pids; do kill -0 $$pid 2>/dev/null && echo $$pid; done); \
		if [ -n "$$alive" ]; then \
			echo "Force killing process(es): $$alive"; \
			kill -9 $$alive 2>/dev/null || true; \
		fi; \
	else \
		echo "No existing local bot process found"; \
	fi

run: stop build
	@set -a; [ -f .env ] && . ./.env; set +a; \
	echo "Starting bot from $(BIN)"; \
	exec $(BIN)

test:
	@go test ./...

tidy:
	@go mod tidy

fmt:
	@gofmt -w $$(find . -name '*.go' -type f)
