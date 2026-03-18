CORE_APP := ./cmd/core
CORE_ADDR ?= http://127.0.0.1:8081
CORE_PID_FILE := .core.pid
CORE_LOG_FILE := .core.log
CORE_REV_FILE := .core.rev

.PHONY: run run-core stop test tidy fmt local-erp-env local-erp-check run-local-erp run-mobile-api core-up core-stop core-restart

stop: core-stop

core-up:
	@current_rev="$$(git rev-parse HEAD 2>/dev/null || echo unknown)"; \
	stored_rev=""; \
	if [ -f "$(CORE_REV_FILE)" ]; then \
		stored_rev="$$(cat "$(CORE_REV_FILE)" 2>/dev/null || true)"; \
	fi; \
	if curl -fsS "$(CORE_ADDR)/healthz" >/dev/null 2>&1 && [ "$$stored_rev" = "$$current_rev" ]; then \
		echo "Core already running at $(CORE_ADDR) on $$current_rev"; \
	else \
		if curl -fsS "$(CORE_ADDR)/healthz" >/dev/null 2>&1; then \
			echo "Core revision changed ($$stored_rev -> $$current_rev); restarting"; \
			$(MAKE) core-stop; \
		fi; \
		echo "Starting core on $(CORE_ADDR)"; \
		setsid go run $(CORE_APP) >"$(CORE_LOG_FILE)" 2>&1 < /dev/null & \
		echo $$! >"$(CORE_PID_FILE)"; \
		for _ in $$(seq 1 40); do \
			if curl -fsS "$(CORE_ADDR)/healthz" >/dev/null 2>&1; then \
				printf '%s\n' "$$current_rev" >"$(CORE_REV_FILE)"; \
				echo "Core ready at $(CORE_ADDR)"; \
				exit 0; \
			fi; \
			sleep 0.5; \
		done; \
		echo "Core failed to start; see $(CORE_LOG_FILE)" >&2; \
		exit 1; \
	fi

core-stop:
	@pids_file=""; \
	if [ -f "$(CORE_PID_FILE)" ]; then \
		pids_file="$$(cat "$(CORE_PID_FILE)" 2>/dev/null || true)"; \
	fi; \
	pids_go=$$(pgrep -x -f "go run ./cmd/mobileapi" || true); \
	pids_core_go=$$(pgrep -x -f "go run ./cmd/core" || true); \
	pids_port=$$(lsof -t -iTCP:8081 -sTCP:LISTEN -n -P 2>/dev/null || true); \
	pids=$$(printf "%s\n%s\n%s\n%s\n" "$$pids_file" "$$pids_go" "$$pids_core_go" "$$pids_port" | tr ' ' '\n' | awk 'NF' | sort -u | paste -sd' ' -); \
	if [ -n "$$pids" ]; then \
		echo "Stopping existing core process(es): $$pids"; \
		kill $$pids 2>/dev/null || true; \
		sleep 1; \
		alive=$$(for pid in $$pids; do kill -0 $$pid 2>/dev/null && echo $$pid; done); \
		if [ -n "$$alive" ]; then \
			echo "Force killing process(es): $$alive"; \
			kill -9 $$alive 2>/dev/null || true; \
		fi; \
	else \
		echo "No existing local core process found"; \
	fi; \
	rm -f "$(CORE_PID_FILE)" "$(CORE_REV_FILE)"

core-restart: core-stop core-up

run: run-core

run-core: core-stop
	@MOBILE_API_ADDR=":8081" go run $(CORE_APP)

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

run-mobile-api: run-core
