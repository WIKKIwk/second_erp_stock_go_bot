APP := ./cmd/bot
BIN_DIR := ./bin
BIN := $(BIN_DIR)/erpnext-bot

.PHONY: run stop build test tidy fmt local-erp-env local-erp-check run-local-erp run-mobile-api

token_from_env = $$( [ -f .env ] && sed -n 's/^TELEGRAM_BOT_TOKEN=//p' .env | head -n1 )

build:
	@mkdir -p $(BIN_DIR)
	@go build -o $(BIN) $(APP)

stop:
	@token="$(token_from_env)"; \
	token="$${token#\"}"; \
	token="$${token%\"}"; \
	token="$${token#\'}"; \
	token="$${token%\'}"; \
	pids_bin=$$(pgrep -x -f "$(CURDIR)/bin/erpnext-bot" || true); \
	pids_go=$$(pgrep -x -f "go run ./cmd/bot" || true); \
	pids_token=""; \
	if [ -n "$$token" ]; then \
		for pid in $$(pgrep -u "$$(id -u)" || true); do \
			[ "$$pid" = "$$" ] && continue; \
			[ "$$pid" = "$$PPID" ] && continue; \
			env_file="/proc/$$pid/environ"; \
			[ -r "$$env_file" ] || continue; \
			if grep -zqx "TELEGRAM_BOT_TOKEN=$$token" "$$env_file" 2>/dev/null; then \
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
	@token="$(token_from_env)"; \
	token="$${token#\"}"; \
	token="$${token%\"}"; \
	token="$${token#\'}"; \
	token="$${token%\'}"; \
	if [ -n "$$token" ]; then export TELEGRAM_BOT_TOKEN="$$token"; fi; \
	echo "Starting bot from $(BIN)"; \
	exec $(BIN)

local-erp-env:
	@./scripts/setup_local_erp_env.sh

local-erp-check:
	@url=$$( [ -f .env ] && sed -n 's/^ERP_URL=//p' .env | head -n1 ); \
	key=$$( [ -f .env ] && sed -n 's/^ERP_API_KEY=//p' .env | head -n1 ); \
	secret=$$( [ -f .env ] && sed -n 's/^ERP_API_SECRET=//p' .env | head -n1 ); \
	if [ -z "$$url" ] || [ -z "$$key" ] || [ -z "$$secret" ]; then \
		echo "ERP_URL / ERP_API_KEY / ERP_API_SECRET topilmadi. Avval make local-erp-env ni ishga tushiring."; \
		exit 1; \
	fi; \
	echo "Checking $$url"; \
	curl -fsS -H "Authorization: token $$key:$$secret" "$$url/api/method/frappe.auth.get_logged_user"

run-local-erp: local-erp-env run

test:
	@go test ./...

tidy:
	@go mod tidy

fmt:
	@gofmt -w $$(find . -name '*.go' -type f)

run-mobile-api:
	@go run ./cmd/mobileapi
