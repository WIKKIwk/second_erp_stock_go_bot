# Customer Partial Acceptance Plan

## Tracking
- Issue: https://github.com/WIKKIwk/erpnext-stock-telegram-core/issues/5

## Goal
Upgrade the current customer delivery workflow from a binary approve/reject model into a production-grade resolution flow that supports:
- full acceptance
- partial acceptance
- full rejection
- post-accept claim / defect return

## Why this is needed
The current system is too narrow for real operations:
- customer can only accept all or reject all
- partial receipt is impossible
- if customer accepts and later finds defects, there is no follow-up claim path
- ERP documents cannot fully represent real-world delivery outcomes

## Current behavior summary
### Customer
- UI only supports approve/reject.
- Server only accepts `approve: bool` and `reason`.
- Full reject creates a real return Delivery Note.
- Full accept closes the response path.

### Werka
- Pending / confirmed / returned counts are derived from delivery state.
- Runtime state reconciliation already exists and must remain correct.

### ERP
- Outbound shipment is represented by the original Delivery Note.
- Full reject creates a Delivery Note return.
- Partial and post-accept claim semantics do not exist yet.

## Desired business workflow
### 1. Initial customer response
Customer chooses one of:
- Accept all
- Accept partially
- Reject all

### 2. Partial acceptance
Customer provides:
- accepted quantity
- returned quantity
- reason
- optional comment

Rules:
- `accepted_qty >= 0`
- `returned_qty >= 0`
- `accepted_qty + returned_qty == sent_qty`
- if `returned_qty > 0`, reason is required

Outcome:
- original shipment remains the source document
- returned portion creates a real ERP return document
- customer-facing and werka-facing states show partial completion

### 3. Post-accept claim
After initial acceptance, customer can still open a follow-up issue:
- defect
- wrong item
- hidden shortage
- packaging damage

Claim provides:
- affected quantity
- reason
- optional comment
- optional photos later if we add attachments

Outcome:
- claim is linked to the original shipment
- ERP gets a real return / reverse document for the claimed quantity
- home/status/history remain consistent

## Implementation stages
### Stage 1. Data contract upgrade
Replace boolean-only customer response with a structured payload.

Proposed request shape:
```json
{
  "delivery_note_id": "MAT-DN-...",
  "mode": "accept_all | accept_partial | reject_all | claim_after_accept",
  "accepted_qty": 0,
  "returned_qty": 0,
  "reason": "",
  "comment": ""
}
```

Backward compatibility:
- keep old `approve` contract temporarily
- server maps old payloads into the new internal command shape

### Stage 2. Server state machine
Introduce explicit transition handling.

Allowed transitions:
- `pending -> accepted`
- `pending -> partial`
- `pending -> rejected`
- `accepted -> claim_open`
- `accepted -> partial` only via claim flow semantics

Disallowed:
- double response on the same pending record without claim mode
- qty totals that do not reconcile with original sent qty

### Stage 3. ERP document semantics
For every return-bearing flow:
- create real return Delivery Note documents
- keep linkage to original shipment
- preserve comments / reason trail

Cases:
- reject all:
  - return against full original quantity
- accept partial:
  - return document for returned portion only
- claim after accept:
  - return document for claim quantity only

### Stage 4. Mobile customer UX
Replace current binary prompt with a proper resolution UI:
- segmented choice or action cards:
  - Accept all
  - Accept partially
  - Reject all
- partial mode shows qty inputs
- accepted-after-claim flow exposed on confirmed shipment detail

### Stage 5. Werka UX
Ensure home/status/detail reflect:
- pending count
- confirmed count
- returned count
- partial/claim notes where relevant

No stale runtime mismatches are allowed.

### Stage 6. Runtime reconciliation
Keep runtime stores safe:
- server-reflected mutations must be reconciled out
- counts and detail lists must come from the same effective state
- add tests for:
  - pending -> confirmed
  - pending -> partial
  - accepted -> claim return

## Concrete code plan
### Mobile app
- `customer_delivery_detail_screen.dart`
  - replace binary approve/reject with structured resolution flow
- `mobile_api_customer.dart`
  - add structured response payload
- `customer_store.dart`
  - support partial / claim transitions
- `werka_store.dart`
  - ensure counts and pending lists remain aligned after customer actions

### Mobile server
- `internal/core/types.go`
  - add new customer response request / result types
- `internal/core/service.go`
  - implement new state machine and ERP return handling
- `internal/mobileapi/server.go`
  - accept new payload, preserve backward compatibility

### Tests
- add server tests for:
  - accept all
  - reject all
  - accept partial
  - claim after accept
- add mobile runtime tests for:
  - count reconciliation
  - partial transitions

## Rollout strategy
### Phase A
- server supports both old and new payloads
- mobile still sends old payload

### Phase B
- mobile upgraded to new customer resolution flow
- server still accepts old payload for safety

### Phase C
- remove old payload path only after we confirm no stale clients remain

## Risks
- ERP return document semantics for partial quantities must be exact
- runtime home counts can drift if mutations are not reconciled
- post-accept claim must not create duplicate returns for the same qty

## Definition of done
- customer can accept partially
- customer can reject fully
- customer can open a post-accept claim
- ERP creates real return documents for returned quantities
- werka home counts and detail lists stay consistent without manual refresh
- mobile/server tests cover all core transitions
