---
name: ssrf-to-rce
description: "Trace server-side URL fetches through redirect, scheme, DNS, metadata, and internal-service boundaries"
---

# SSRF impact chain assessment

Adapted from the KSIR-ARTEX SSRF playbook (`240ae7c`). A successful server-side fetch, access to an internal service, cloud metadata access, credential exposure, and code execution are distinct claims. Each transition requires its own scoped evidence.

## Find and confirm the fetch

1. Identify a URL-fetching feature such as a preview, webhook test, image import, document converter, or PDF renderer. Capture an ordinary request and its returned state.
2. Supply a unique operator-owned canary URL. Record the canary request ID, source IP, method, redirect path, DNS query if visible, and the application's response. A DNS lookup alone does not prove an HTTP fetch.
3. Check validation of scheme, host, IP literal, userinfo, port, redirect destination, and DNS changes using canary-controlled endpoints. Test one parser difference at a time and keep all destinations in task scope. Record a negative control rejected by the application.
4. Compare the requested URL with the final destination observed by the canary or proxy. Determine whether the application validates before or after redirect and DNS resolution.

## Advance only from evidence

- **Internal reachability:** use a specific operator-owned internal canary or supplied service evidence. A timeout or generic 403 does not identify the remote service.
- **Metadata:** `query_imds` provides a scoped assessment plan and does not fetch a token. If the operator supplies workload configuration or sanitized metadata evidence, record provider, endpoint protection, hop limit, and assigned identity without printing bearer tokens.
- **Credential exposure:** establish whether a response actually contains sensitive material, then retain only a redacted reference and follow the task's incident handling process.
- **Execution:** establish the exact vulnerable internal service, operation, and separately authorized harmless execution proof in an isolated environment. SSRF alone is not RCE.

Use `attack_chain` to link confirmed transitions and the chain verification task for end-to-end impact. Record blocked prerequisites and egress allowlist, redirect, scheme, and metadata-hardening recommendations. Never write credentials or raw tokens into task output.
