---
name: race-condition
description: "Test single-use, ordering, and transaction invariants with bounded concurrent requests and committed-state proof"
---

# Race-condition assessment

Adapted from the KSIR-ARTEX race playbook (`240ae7c`). The source covered HTTP/2 synchronized release, HTTP/1.1 last-byte synchronization, coupon reuse, cart/payment limits, registration, file-upload checks, and session races. Its example scripts hardcoded a public host, disabled TLS verification, and treated multiple 200 responses as proof. Use the native `race_probe` tool for a bounded, captured test instead; it makes real concurrent requests and can compare committed state before and after. It does not claim to perform a single-packet attack.

## Choose the invariant

| Candidate | Expected invariant | Required independent state read |
| --- | --- | --- |
| Single-use coupon or invite | One redemption per owned test code | Coupon or account redemption count |
| Inventory reservation | No more units reserved than a disposable stock record allows | Reservation or stock ledger |
| Balance transfer | One transfer changes two owned test balances exactly once | Both balances and transaction IDs |
| Registration or enrollment | One identity or slot is created per unique test input | Created record count and ownership |
| File validation then use | A file is not served or executed before validation completes | Storage state and retrieval behavior |
| Session refresh or logout | A revoked session cannot complete a protected action | Session state plus action audit event |

Do not use real customer balances, shared promotion codes, or production payment records. Specify the exact URL, method, test identity, number of requests, expected invariant, and allowed duration before starting.

## Execute and verify

1. Capture a normal request and a read-only state endpoint. Save the baseline response, state value, and request ID. Ensure both URLs are in operator-entered task scope and traffic capture is active.
2. Start with a small `race_probe` burst and increase only within the agreed window; the tool caps a burst at 16 requests and accepts at most eight body variants. Use one disposable record per run.
3. Set `state_read_url` so the tool reads committed state before and after the burst. Record per-request status and response digests, but decide the result from state changes and transaction identifiers.
4. Repeat a promising result once with fresh state. Compare against a sequential control to separate a real concurrent violation from ordinary retries, idempotency, or stale reads.
5. If the result depends on tighter synchronization than `race_probe` provides, label the case unverified. In an isolated lab, a separately reviewed HTTP/2 or last-byte harness can test that specific timing hypothesis; do not paste the old source's hardcoded host or insecure TLS settings into a live task.
6. Save a finding only if the committed state breaks the invariant. Include exact request count, identities, input record, timing, before/after state, negative control, and the transaction or locking fix.

A duplicate 2xx or 302 may be a front-end response with no committed action. A 409 or 429 may come from rate limiting rather than race protection. The proof is the application state and its audit trail, not the status histogram alone.
