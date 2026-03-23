# Project Memory

## Workspace

- Root workspace: `erpnext_stock_telegram`
- Mobile app repo: `/home/wikki/local.git/erpnext_stock_telegram/mobile_app`
- Mobile backend repo: `/home/wikki/local.git/erpnext_stock_telegram/mobile_server`
- ERPNext custom app repo: `/home/wikki/local.git/erpnext_n1/erp/apps/accord_state_core`
- ERP bench used by `localhost:8000`: `/home/wikki/local.git/erpnext_n1/erp`

## Core Architecture

- Supplier mobile flow writes `Purchase Receipt`
- Werka -> Customer Issue flow writes `Delivery Note`
- Customer mobile flow responds to existing `Delivery Note`
- `accord_state_core` is a backend-only ERP app that creates/maintains custom fields on `Delivery Note`
- `mobile_server` also has fallback logic to ensure those custom fields exist

## ERP Custom Fields on Delivery Note

Maintained fields:

- `accord_flow_state`
- `accord_customer_state`
- `accord_customer_reason`
- `accord_delivery_actor`
- `accord_status_section`
- `accord_ui_status`

UI intent:

- internal state fields are hidden
- `accord_ui_status` is visible and read-only
- `accord_ui_status` is placed under `Details`, after `posting_time`, inside `Accord Status`

Expected values for `accord_ui_status`:

- `pending`
- `confirm`
- `rejected`

## Critical Business Rule

Important:

- `customer confirm` must NOT create a return document
- `customer reject` MUST create a real ERPNext return `Delivery Note`
- Reject is not just a label/status problem
- Reject must create real return flow so stock does not remain in limbo

## Delivery Note Response Logic

Current intended behavior:

1. Werka creates customer issue
   - opens and submits original `Delivery Note`
   - original DN gets customer workflow state fields
2. Customer confirms
   - updates custom state on original DN
   - no return DN is created
3. Customer rejects
   - creates and submits a real return `Delivery Note`
   - return DN has `is_return = 1`
   - return DN points to original with `return_against`
   - original DN becomes `Return Issued`

## Verified Facts

- Backend can create `Delivery Note` correctly
- Verified manually by creating:
  - `MAT-DN-2026-00087`
  - `MAT-DN-2026-00088`
- ERPNext standard return flow works through:
  - `erpnext.stock.doctype.delivery_note.delivery_note.make_sales_return`
- Standard behavior in ERPNext list is:
  - original DN remains visible
  - return DN appears as a separate row
  - this is normal

## Mobile App Facts

- Mobile app repo did NOT receive `Delivery Note` flow changes in this session
- Mobile app change done in this session:
  - `adce203` `Prevent duplicate supplier dispatch submits`
- Because of that, any remaining `Delivery Note` creation issue from the phone may still be a mobile flow problem, not backend creation capability

## Mobile Server Commits Added In This Session

- `690673a` `Write custom delivery note UI status`
- `f02ec64` `Create delivery UI status section field`
- `255bf42` `Use live delivery state codes`
- `7e7662c` `Update delivery state tests for live codes`
- `f70791c` `Normalize config phone values for Werka auth`
- `218df24` `Log delivery note create requests`
- `948b96c` `Match Werka phone by local-number suffix`
- `2d1cc32` `Do not block Werka on phone mismatch`
- `477db06` `Create real delivery return on customer reject`
- `6688173` `Only create delivery return on reject`

## ERP Custom App Commits Added In This Session

- `f4d3643` `Show delivery note state fields in UI`
- `d73d1e6` `Add missing Frappe module package`
- `4e6ee4e` `Add delivery note UI status field`
- `19d7d7e` `Preserve delivery UI status during backfill`
- `38fb585` `Place delivery UI status in details tab`
- `4dee08d` `Align delivery status mapping with live data`

## Important Bugs Found And Fixed

### Duplicate supplier submit

- Cause:
  - supplier confirm screen allowed double tap
  - same request could be sent twice
- Fix:
  - submit button locked while request is in progress

### Werka auth blocked by phone format

- Cause:
  - phone matching logic was too strict and caused delivery note flow to fail depending on phone format
- Fix:
  - backend no longer blocks Werka operations because of phone formatting drift

### Customer confirm incorrectly created return

- Cause:
  - return creation call was placed outside the `approve/reject` branch
- Fix:
  - return is created only when `approve == false`

## Current Cleanup State

A destructive cleanup was executed on ERP transaction data.

Deleted:

- all `Delivery Note`
- all `Purchase Receipt`
- all `Stock Entry`
- related `Comment`
- related `GL Entry`
- related `Stock Ledger Entry`

Kept:

- `Item`
- `Customer`
- `Supplier`
- Werka identity/config

## Current Counts After Cleanup

- `Delivery Note`: `0`
- `Purchase Receipt`: `0`
- `Stock Entry`: `0`
- `Item`: `9`
- `Customer`: `9`
- `Supplier`: `7`

## Useful URLs / Runtime

- Local backend health: `http://127.0.0.1:8081/healthz`
- Public domain: `https://core.wspace.sbs`
- ERP URL from backend env: `http://localhost:8000`

## Current Main Risk

Main unresolved risk:

- phone testing from the actual device may still be exercising a stale or wrong mobile flow
- backend delivery note creation itself is verified to work
- if phone still does not create DN, next debugging target should be `mobile_app` flow and actual endpoint hit path

## Rule For Future Work

- commit every meaningful change
- avoid touching unrelated areas while debugging critical flows
- for customer reject flow, preserve real ERP return semantics
