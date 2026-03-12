# Hard Test

Current phase: `5. Hard test`

Date: `2026-03-11`

## Current status

Safe runtime checks completed:

- `core` health check passes on `http://127.0.0.1:8081/healthz`
- admin login works on local `core`
- werka login works on local `core`
- `go test ./...` passes
- desktop/API hard-test flow was executed successfully on `2026-03-11`

Current blocker:

- Android real-device push is not yet verified to completion
- earlier ERP-local and partial-return issues were fixed during hard test
- Android release APK is ready at `/home/wikki/local.git/erpnext_stock_telegram/mobile_app/build/app/outputs/flutter-apk/accord.apk`
- current domain build target: `https://core.wspace.sbs`

## Executed results

Completed live checks on `2026-03-11`:

- admin read-only smoke: passed
- werka read-only smoke: passed
- supplier read-only smoke: passed
- supplier dispatch -> werka pending: passed
- werka full accept: passed
- werka partial return: passed
- supplier acknowledgment after partial return: passed
- werka synthetic acknowledgment event: passed
- full return path: passed
- cross-account detail guard: passed with `403 forbidden`

Representative live receipts created during test:

- full accept: `MAT-PRE-2026-00050`
- partial return + acknowledgment: `MAT-PRE-2026-00051`
- full return: `MAT-PRE-2026-00049`

Representative observed results:

- new supplier dispatches now appear in werka pending
- partial return now submits successfully with an alternate non-group rejected warehouse
- supplier acknowledgment no longer fails if remarks backfill is rejected by ERP
- supplier A receipt detail is blocked for supplier B

## Before running hard test

1. Make sure local ERP is up on `http://localhost:8000`.
2. Verify ERP auth works:

```bash
cd /home/wikki/local.git/erpnext_stock_telegram
make local-erp-check
```

3. Restart local core after ERP is confirmed:

```bash
cd /home/wikki/local.git/erpnext_stock_telegram
make core-restart
curl -sS http://127.0.0.1:8081/healthz
```

## Hard test order

Run in this order so failures are easy to isolate.

### 1. Admin read-only smoke

- login as admin
- open admin settings
- open supplier summary
- open supplier list
- open admin activity

Expected:

- no `500`
- supplier summary counts load
- supplier list renders
- admin activity returns existing ERP-backed history

### 2. Supplier read-only smoke

- login as an existing supplier
- open profile
- open supplier home
- open notifications
- open recent
- open item picker

Expected:

- supplier history loads
- notification detail opens only for that supplier
- recent screen shows repeatable history
- item list loads without lag spikes

### 3. Supplier -> Werka flow

- supplier creates a new dispatch
- verify it appears in supplier home/recent/notifications
- login as werka
- verify the same receipt appears in werka pending/home/notifications

Expected:

- one new draft purchase receipt in ERP
- same receipt id is visible on both sides
- no duplicate synthetic events

### 4. Werka accept flow

- from werka, fully accept a pending receipt
- verify supplier sees accepted status
- verify admin activity shows accepted event

Expected:

- receipt status becomes accepted
- accepted quantity matches sent quantity
- supplier notification detail is readable

### 5. Werka partial return flow

- create a new supplier dispatch
- from werka, accept partial quantity and return the rest
- include reason and optional comment
- verify supplier side note formatting
- verify admin activity shows partial result

Expected:

- note contains Accord partial-return lines
- supplier detail screen shows comment thread correctly
- status is partial, not accepted

### 6. Werka full return flow

- create a new supplier dispatch
- from werka, choose full return
- select reason
- finish confirmation

Expected:

- ERP submit path is skipped for full return
- remarks/comment note is written
- supplier app shows `rejected` or `cancelled`

### 7. Supplier acknowledgment flow

- open a returned receipt on supplier side
- press acknowledgment action
- verify werka notifications show supplier acknowledgment synthetic event
- verify admin activity also reflects it

Expected:

- acknowledgment comment is saved
- werka feed shows `supplier_ack` synthetic event
- duplicate acknowledgment is blocked on supplier detail

### 8. Cross-account guard

- login as supplier A and open a detail
- switch to supplier B
- verify stale detail is blocked

Expected:

- old detail does not leak across suppliers
- screen resets or blocks access cleanly

## Android push verification

This is required for full closure of the phase and cannot be treated as complete from desktop-only checks.

### Required device test

- install fresh domain APK: `/home/wikki/local.git/erpnext_stock_telegram/mobile_app/build/app/outputs/flutter-apk/accord.apk`
- login and allow notification permission
- verify token is stored in `data/mobile_push_tokens.json`
- with app foregrounded, backgrounded, and closed:
  - trigger supplier dispatch
  - trigger werka confirmation
  - trigger supplier acknowledgment

Expected:

- local notification appears on device
- push arrives in background and closed state
- affected screens refresh after reopening

## Completion rule

Mark `Hard test` complete only after:

- local ERP-backed flows pass end-to-end
- supplier -> werka -> supplier loop passes
- Android closed-app push is verified on real device

Current assessment:

- desktop/API hard-test portion: complete
- Android real-device push verification: still pending
