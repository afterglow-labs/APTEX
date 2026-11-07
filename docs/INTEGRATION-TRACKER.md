# Afterglow integration tracker

The product goal is one application with the useful capabilities from ScopeWeaver, Artex-on-ChatGPT, artex-deep-hunter, artex-browser-chat, and KSIR-ARTEX. This tracks the integrated code, not a release claim.

| Capability | Current state | Release gate or intentional boundary |
| --- | --- | --- |
| ScopeWeaver task graph, agent routing, six-language UI, MCP, notifications, local LLM controls | Base | The full root Go suite passed with a disposable PostgreSQL database; release QA remains. |
| ChatGPT OAuth, account storage, refresh/recheck/quota, Responses adapter, profile routing, model catalog, UI | Integrated | End-to-end login, refresh, replay, quota, and blocked-plan cases need a real account and full build. |
| Persistent browser sessions, operator control, screenshots and image compaction | Integrated | Browser tests need Playwright and an executable app. |
| Authenticated browser traffic extension | Integrated in `634e705` | Full browser capture test remains. |
| Workflow canvas, manual/interval/daily schedules and five agent workflow tools | Integrated | The saved-prompt runner reports unavailable agents instead of a false success. |
| Task-backed red, purple, and blue workflows | 29 authored procedures, persisted task phases and scope, 40 bundled playbooks | Live execution requires a configured model and applicable target/tool dependencies. BBOT, REA MCP, and Windows lab execution are external prerequisites. |
| Operator questions, encrypted test accounts, task outcomes, lessons, skill gaps | Integrated | End-to-end task tests need the full app. |
| Deep Hunter follow-up waves, cross-task hints, campaign branches, hunt board, business map, attack chains and verification, stall watchdog | Integrated | Database and lifecycle tests need PostgreSQL. Chain composition is operator-started and verification is a separate task. |
| Native race, identity, and inflight request replay | Integrated with scope, capture, and approval controls | Full worker and proxy integration tests need PostgreSQL and a running capture proxy. |
| Defensive rules, ATT&CK layer, indicators, host triage, bundled JSONL reference matcher | Integrated | Docker detection suite covers atomic and correlation positives and negatives; production SIEM field mappings need separate verification. |
| Proxy graph, artifact index, C2 handoff, cloud planning, lateral management-path inventory | Integrated with operator and scope controls | `lateral_scan` measures scoped TCP ports; credential, movement, escalation, EDR, and cleanup tools remain evidence/planning paths without a Windows lab executor. C2 is a handoff record. |
| Optional Telegram MCP helper | Integrated as stdio plugin | Operator must install Python dependencies, supply credentials and destination allowlist, and connect it manually. No HTTP wizard. |
| Additional source playbooks | 40 bundled; WAF and chaining techniques expanded into testable runbooks | Installed playbooks are bound once to Worker/Pentest; operator opt-outs persist. Source payloads and automatic rotation lists remain archived outside the runtime image. |
| KSIR mocked offensive success tools | Not enabled as execution | Their source implementations returned success without performing the claimed action. Handoff, plans, and local artifact indexing replace those claims. |

## ChatGPT account path

The plan profile stores only an account ID in PostgreSQL. The private OAuth store has a dedicated Docker volume. Plan inference uses the official public Responses endpoint and does not enter API-key failover. Opaque replay state stays in the transcript and is removed from debug request records. `SCOPEWEAVER_LOCAL_LLM_ONLY=true` blocks plan inference and the account model catalog.

## Verification limits

The root Go build, vet, and full test suite now pass with a disposable PostgreSQL
database. The Next.js static build, frontend tests, browser runtime test,
extension tests, and Docker detection suites also passed; see [merge
verification](AFTERGLOW-MERGE.md) for the exact checks. The local Compose
deployment and native APTEX launcher have also been verified with the existing
local model. APTEX and Odysseus now use separate clients and server lifecycles;
the earlier embedded-mode browser journey is retained as historical evidence.
A real ChatGPT account sign-in and inference journey remains unverified.
