#!/usr/bin/env bash
set -euo pipefail

BOT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BENCH_ROOT="${ERP_BENCH_ROOT:-/home/wikki/local.git/erpnext_n1/erp}"
ERP_LOCAL_URL="${ERP_LOCAL_URL:-http://localhost:8000}"
ERP_API_USER="${ERP_API_USER:-Administrator}"
ENV_FILE="${BOT_ENV_FILE:-$BOT_ROOT/.env}"

if [ ! -x "$BENCH_ROOT/env/bin/bench" ]; then
	echo "bench executable not found: $BENCH_ROOT/env/bin/bench" >&2
	exit 1
fi

if [ ! -x "$BENCH_ROOT/env/bin/python" ]; then
	echo "python executable not found: $BENCH_ROOT/env/bin/python" >&2
	exit 1
fi

raw_output="$(
	cd "$BENCH_ROOT"
	./env/bin/bench execute frappe.core.doctype.user.user.generate_keys --kwargs "{\"user\":\"${ERP_API_USER}\"}"
)"

parsed_output="$(
	"$BENCH_ROOT/env/bin/python" - "$raw_output" <<'PY'
import ast
import json
import sys

raw = sys.argv[1].strip()
data = None
for parser in (json.loads, ast.literal_eval):
    try:
        data = parser(raw)
        break
    except Exception:
        pass

if not isinstance(data, dict):
    raise SystemExit(f"Unable to parse generate_keys output: {raw}")

print(str(data["api_key"]).strip())
print(str(data["api_secret"]).strip())
PY
)"

api_key="$(printf '%s\n' "$parsed_output" | sed -n '1p')"
api_secret="$(printf '%s\n' "$parsed_output" | sed -n '2p')"

mkdir -p "$(dirname "$ENV_FILE")"
touch "$ENV_FILE"

upsert_env() {
	local key="$1"
	local value="$2"
	local tmp
	tmp="$(mktemp)"

	awk -v key="$key" -v value="$value" '
	BEGIN { updated = 0 }
	index($0, key "=") == 1 {
		print key "=" value
		updated = 1
		next
	}
	{ print }
	END {
		if (!updated) {
			print key "=" value
		}
	}
	' "$ENV_FILE" >"$tmp"

	mv "$tmp" "$ENV_FILE"
}

upsert_env "ERP_URL" "$ERP_LOCAL_URL"
upsert_env "ERP_API_KEY" "$api_key"
upsert_env "ERP_API_SECRET" "$api_secret"

echo "Updated $ENV_FILE for local ERPNext."
echo "ERP_URL=$ERP_LOCAL_URL"
echo "ERP_API_USER=$ERP_API_USER"
echo "API key and secret were refreshed from local bench."
