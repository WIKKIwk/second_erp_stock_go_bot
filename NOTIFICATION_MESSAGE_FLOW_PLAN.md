# Notification And Message Flow Plan

This document defines the target plan for fixing notifications and message
flow without throwing away the current architecture.

The goal is:

- keep ERPNext as the source of truth
- keep the Go mobile server as the orchestration layer
- preserve the fast DB-reader path for performance
- stop using generic `history` as a fake notification feed
- support direct cross-role messaging notifications

## Current Problems

### 1. Notification feed is not canonical

Right now the app treats `history` as `notifications`.

For Werka this is especially wrong:

- `WerkaNotificationsScreen` reads `WerkaStore.historyItems`
- `WerkaRecentScreen` also reads `WerkaStore.historyItems`
- `WerkaHistory` mixes multiple event types:
  - purchase receipt history
  - supplier acknowledgment events
  - customer result events

Because of that:

- `recent` and `notifications` are semantically mixed together
- pending customer shipments created by Werka are not clearly represented
- the user sees empty or confusing notification screens

### 2. Werka runtime polling is architecturally wrong

The runtime notification poll for Werka currently:

- refreshes `home`
- then reads `historyItems`

This means the refresh source and the displayed feed are not the same thing.

### 3. Comment notifications are incomplete

Comments already exist in ERPNext and are available through
`NotificationDetail` / `AddNotificationComment`, but cross-role push logic is
only partially wired.

Examples:

- supplier acknowledgment special case already sends push to Werka
- generic comment messages do not consistently notify the opposite side
- Werka batch customer issue creation currently skips the single-create push path

### 4. Local unread/hidden state is not canonical

Unread and hidden state currently live only in app local storage.

This is acceptable for an incremental phase, but it means:

- read/unread does not sync across devices
- clear-all is local to one device

## Source Of Truth Principle

ERPNext remains the source of truth.

The mobile server must not invent a separate notification database in the first
phase.

Instead:

- server derives notification feed from ERPNext documents and ERPNext comments
- reader path uses direct DB reads for speed
- fallback path can continue using ERP API if needed

This keeps the architecture aligned with the current design.

## Target Model

We split three concepts that are currently mixed:

### 1. History

Purpose:

- audit-style timeline
- older operational events
- broad historical trace

Examples:

- purchase receipt completed
- customer accepted/rejected shipment
- supplier acknowledgment event

### 2. Notifications

Purpose:

- inbox for actionable and fresh events
- cross-role updates
- message/comment alerts

Examples:

- Werka sent a delivery note to customer
- customer responded to Werka shipment
- supplier responded to unannounced receipt
- comment added on a relevant thread

### 3. Recent / Repeat

Purpose:

- quick action replay
- prefill helper

For Werka this is optional and currently low value. It can be archived or
removed from dock without affecting the new notification model.

## Target Endpoints

Add role-specific canonical notification endpoints:

- `GET /v1/mobile/werka/notifications`
- `GET /v1/mobile/supplier/notifications`
- `GET /v1/mobile/customer/notifications`

These endpoints should return the same `DispatchRecord` shape in phase 1 so the
app can integrate with minimal churn.

That lets us:

- keep existing row rendering
- keep local unread/hidden logic for now
- avoid a large DTO migration

In a later phase we can introduce a dedicated notification DTO if needed.

## DB Reader Strategy

Notification feed should use the same fast-path philosophy as current Werka
home/history.

### Werka notifications should be derived from:

1. Customer delivery note events
   - new customer shipment sent by Werka
   - customer accepted / partial / rejected

2. Supplier acknowledgment events
   - supplier writes acknowledgment-style comments

3. Supplier unannounced response events
   - supplier approved or rejected the unannounced receipt

4. Comment events on relevant operational documents
   - comments on Werka-customer delivery notes
   - comments on Werka-supplier purchase receipts

### Supplier notifications should be derived from:

1. Supplier purchase receipt events relevant to that supplier
2. Werka unannounced receipt events
3. Werka comments on supplier-side threads
4. Werka confirm result events if relevant to supplier workflow

### Customer notifications should be derived from:

1. Delivery note events for that customer
2. Werka comments on that customer's delivery notes
3. customer-side state transitions already visible in delivery note state

## Message / Comment Notification Rules

The comment behavior should be explicit.

### Delivery Note thread

If target document is a Delivery Note:

- customer writes comment -> notify Werka
- Werka writes comment -> notify that customer

### Purchase Receipt thread

If target document is a Purchase Receipt:

- supplier writes comment -> notify Werka
- Werka writes comment -> notify that supplier

### Special cases

- supplier acknowledgment style message still remains a first-class event
- generic comments should also produce push and feed visibility

## Push Rules

Push should be event-driven from the mobile server at write time.

This means:

- create shipment -> push immediately to target customer
- customer response -> push immediately to Werka
- supplier unannounced response -> push immediately to Werka
- comment create -> push immediately to opposite side

This is already partially present in `server.go`; phase 1 completes the missing
paths instead of replacing the push system.

## App Refresh Rules

Notification refresh should only refresh notification feed.

That means:

- notifications screen refresh calls notification endpoint only
- runtime poll uses notification endpoint only
- `home` refresh stays for home
- `history` refresh stays for history

No more:

- refresh home, then render notifications from history

## Recommended Incremental Rollout

### Phase 1. Minimal Correctness Fix

Goal:

- stop architectural mismatch with minimal risk

Steps:

1. Add canonical notification endpoints in mobile server
2. Build Werka notification reader path from ERPNext DB
3. Add app-side notification stores separate from history stores
4. Update notification runtime polling to hit notification endpoint only
5. Keep local unread/hidden behavior unchanged for now

Expected outcome:

- notification screen becomes correct without a large app rewrite

### Phase 2. Complete Push Coverage

Goal:

- every meaningful cross-role event sends push immediately

Steps:

1. Add generic comment push in `handleNotificationComment`
2. Add batch customer issue push coverage
3. Make sure all write paths emit `RefreshHub` plus push payload consistently

Expected outcome:

- users receive notification immediately after comments and responses

### Phase 3. Optional Recent Archive

Goal:

- remove low-value UX clutter

Options:

1. hide Werka recent tab from dock
2. keep route only for debugging
3. later delete route if the team never uses repeat flow

Expected outcome:

- Werka navigation gets simpler

### Phase 4. Optional Server-Side Read State

Goal:

- sync unread/read between devices

This is intentionally not phase 1.

Only do this after canonical feed is stable.

## Concrete Server Implementation Plan

### A. Add notification service methods

In `mobile_server/internal/core/service.go` add:

- `WerkaNotifications(ctx)`
- `SupplierNotifications(ctx, principal)`
- `CustomerNotifications(ctx, principal)`

Behavior:

- use `reader` fast path first
- fallback to ERP/API collector path second

### B. Add reader methods

In `mobile_server/internal/erpdb/reader.go` add:

- `WerkaNotifications(ctx)`
- `SupplierNotifications(ctx, supplierRef)`
- `CustomerNotifications(ctx, customerRef)`

Reader queries should merge:

- relevant documents
- relevant comment events
- relevant response events

Then:

- normalize into `DispatchRecord`
- sort descending by created label / timestamp
- cap with a reasonable limit, for example 120

### C. Add mobile API handlers

In `mobile_server/internal/mobileapi/server.go` add:

- `handleWerkaNotifications`
- `handleSupplierNotifications`
- `handleCustomerNotifications`

### D. Complete write-time push fanout

In `handleNotificationComment`:

- resolve target document type
- decide opposite role
- send push to opposite role/ref

In batch create:

- for each created customer shipment, send customer push

## Concrete App Implementation Plan

### A. Separate stores

Add:

- `WerkaNotificationStore`
- `SupplierNotificationStore`
- `CustomerNotificationStore`

Do not reuse `historyItems` for notifications anymore.

### B. Notification runtime

Update `NotificationRuntime`:

- supplier role -> refresh supplier notifications
- Werka role -> refresh Werka notifications
- customer role -> refresh customer notifications

No store should load `history` just to show notifications.

### C. Existing notification screens

Update:

- `WerkaNotificationsScreen`
- `SupplierNotificationsScreen`
- `CustomerNotificationsScreen`

to read their notification store instead of history store.

### D. Keep detail/comment screen

Do not redesign notification detail now.

Keep:

- `notification detail`
- `notification comments`

because they already map to ERPNext threads.

## Why This Plan Is Safe

This plan does not require:

- a new database
- a new event bus
- a full mobile app rewrite
- replacing ERPNext as source of truth

It only requires:

- new derived feed endpoints
- better DB-reader queries
- app separation of `history` vs `notifications`
- fuller push coverage

## Recommended Immediate First Slice

The first high-value slice should be:

1. `GET /v1/mobile/werka/notifications`
2. DB-reader implementation for Werka notifications
3. `WerkaNotificationStore`
4. update Werka notification screen + runtime poll to use it
5. generic comment push from `handleNotificationComment`

This slice gives the biggest architectural correction with the least churn.

## Final Decision Guidance

If the goal is:

- better performance
- simpler logic
- fewer empty notification screens
- real cross-role message alerts

then this plan is the correct direction.

It keeps the current system recognizable while fixing the part that is
currently conceptually wrong.
