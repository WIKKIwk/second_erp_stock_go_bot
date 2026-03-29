# Next Agent Handoff

## Current Truth

- Work must target the remote ERP site `erp.accord.uz`.
- Do not use the laptop-local ERP for ACP import work.
- ACP import mandate lives in [ACP_IMPORT_MANDATE.md](/home/wikki/storage/local.git/erpnext_stock_telegram/ACP_IMPORT_MANDATE.md).
- Core rule: `1 item = 1 customer`.
- If an item name maps to multiple customers, do not guess. Exclude it or stop and ask.

## ACP Import Status

- Source file: [ACP-Products.csv](/home/wikki/storage/local.git/erpnext_stock_telegram/ACP-Products.csv)
- Clean import file: [ACP-Products-clean.csv](/home/wikki/storage/local.git/erpnext_stock_telegram/ACP-Products-clean.csv)
- Conflict list: [ACP-Products-conflicts.txt](/home/wikki/storage/local.git/erpnext_stock_telegram/ACP-Products-conflicts.txt)
- Conflicts excluded: `34`
- Imported to `erp.accord.uz` successfully.

### Verified Remote Counts

- `customers = 441`
- `items = 2769`
- `item_customer_rows = 2769`
- `item_barcodes = 487`

### Sample Verified Rows

- `CPP / 20 mikron / 500` -> customer `Group`, rate `2.8`
- `Imperator salyami` -> customer `Abdulatifaka Imperator`, rate `5.2`, barcode `4780052720304`
- `payushi 4,5sm trubichka 30mk` -> customer `Abdulaziz aka trubichka`, rate `2.6`

## ACP Importer Code

Implemented in:

- [main.go](/home/wikki/storage/local.git/erpnext_stock_telegram/mobile_server/cmd/import_acp_products/main.go)
- [importacp.go](/home/wikki/storage/local.git/erpnext_stock_telegram/mobile_server/internal/importacp/importacp.go)
- [importacp_test.go](/home/wikki/storage/local.git/erpnext_stock_telegram/mobile_server/internal/importacp/importacp_test.go)
- [client.go](/home/wikki/storage/local.git/erpnext_stock_telegram/mobile_server/internal/erpnext/client.go)
- [client_test.go](/home/wikki/storage/local.git/erpnext_stock_telegram/mobile_server/internal/erpnext/client_test.go)

### Important Behavior

- `Agent -> customer`
- `Nom -> item name / item code`
- `Price -> standard_rate`
- `Barcode -> Item Barcode`
- duplicate barcode on another item no longer aborts the whole import; importer skips that barcode and continues

## Relevant Commits Already Pushed

### Root repo

- `aa54600` - `Add ACP importer mandate note`

### mobile_server

- `c577fbb` - `Add CSV customer item import tool`
- `60d8904` - `Block CSV imports from reassigning linked items`
- `eedf871` - `Document exclusive CSV import rule`
- `f15b759` - `Add ACP products importer`

### accord_state_core

- `4b0b9b1` - `Add partial delivery UI status option`

## Remote Server Context

- host: `10.42.0.80`
- ssh user: `wikki`
- ERP bench root: `/home/wikki/erpnext`
- mobile server deploy root: `/home/wikki/deploy/mobile_server_deploy`

## Local Dirty Files To Avoid Touching Casually

These existed outside the ACP-specific code path and were not part of the ACP commit:

### root repo

- `.env.example`
- `ACP_IMPORT_MANDATE.md`
- deleted local docs/assets still unstaged
- `ACP-Products.csv`
- `ACP-Products-clean.csv`
- `ACP-Products-conflicts.txt`

### mobile_server repo

- `README.md`
- `cmd/core/main.go`
- `internal/mobileapi/auth.go`
- `internal/mobileapi/server_test.go`
- `.tmp/`
- `build/`
- `internal/core/session_manager_test.go`

### mobile_app repo

- `start_domain_core.sh`
- `android/app/google-services.json`
- `flutter_01.png`
- `flutter_02.png`

## If You Continue ACP Work

1. Stay on `erp.accord.uz`.
2. Stay on `ACP-Products.csv`.
3. Do not reintroduce `brnarsa.csv` side work.
4. Respect `1 item = 1 customer`.
5. If new ACP rows conflict, update the clean/conflict split explicitly and verify before import.
