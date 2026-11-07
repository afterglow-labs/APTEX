---
name: waf-bypass
description: "Compare WAF normalization, method, header, body, path, and rate-limit behavior on an authorized application"
---

# WAF policy-gap assessment

Adapted from the KSIR-ARTEX WAF playbook (`240ae7c`). Use an exact task scope, an operator-defined request budget, test identities, and the recording proxy. Once those are set, test the agreed variants within that budget; do not mistake a block page for a successful defense or a 200 response for a bypass.

## Baseline and objective

1. Name the protected route, expected application action, upstream origin, and test account. Save one ordinary request and response, WAF request ID, timestamp, and the corresponding application log or state read.
2. Define the security invariant. Examples: an unauthorized test account cannot access an object; a disallowed inert input is rejected; one account receives no more than the agreed requests. Use disposable data for write actions.
3. Record the permitted methods, request count, body size, and any edge challenge behavior. Keep all variants on the same task scope and interception path.

## Source technique inventory, adapted for controlled comparisons

Run one variable at a time and keep a normal control between groups. Use `curl` or the browser/request tools only when they route through the configured capture proxy; otherwise record the missing prerequisite.

| Surface | Controlled variants | What to compare |
| --- | --- | --- |
| Client IP trust | Baseline, then a single designated test-proxy value in `X-Forwarded-For`, `X-Real-IP`, or `True-Client-IP` | Whether the edge and origin agree on the client address and apply the same rule. A claimed private IP is not authorization. |
| Rewritten path | Baseline path, then `X-Original-URL` or `X-Rewrite-URL` containing that same approved route | Whether edge and origin route or authorize different paths. |
| HTTP method | GET versus HEAD or OPTIONS; POST versus PUT or PATCH only for a disposable operation that supports them | Whether a method reaches the application and whether the same policy is enforced. Do not substitute DELETE or TRACE for a write test. |
| Body parser | Valid JSON, form, text, XML, or multipart forms that the endpoint actually accepts | Whether the WAF and application parse equivalent fields differently. Set correct Content-Type and boundaries. |
| Path normalization | Case, encoded slash, double encoding, or trailing dot on the same approved resource | Whether edge and application normalize to the same route. Avoid requests that resolve outside scope. |
| Size boundary | A small benign marker before and after a documented inspection boundary | Whether an allowed body is interpreted differently by the application. Stay below the agreed body limit. |
| Rate limit | A small, fixed number of requests from one designated account, then a second designated account if allowed | Which identity key the control uses and whether the intended operation was committed. |

Cloudflare Ray IDs, Akamai reference IDs, ModSecurity rule IDs, and AWS WAF logs are useful correlation keys when available. Do not infer a vendor or a universal bypass from a status code or a single header; identify the actual edge response and configuration. The source fork's rotation pools and automatic escalation instructions are preserved in `extras/source-assets/waf-bypass` for provenance, but are not evidence of a working bypass.

## Confirmation and handoff

For every variant, save the exact request, status, response marker, application state, edge ID, and corresponding defender event. A positive result requires the application to perform the prohibited action or expose the prohibited data under the test identity, and a control showing the normal rule. If telemetry is missing, mark detection unverified. Report the specific normalization or trust mismatch, origin configuration, and replayable regression request. Keep a rejected hypothesis in the report rather than quietly retrying broader payloads.
