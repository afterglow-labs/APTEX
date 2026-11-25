# Afterglow merge decisions and verification

This record covers the original checkout and the expanded integration. ScopeWeaver is the base because it has the broadest maintained Go and Next.js application, task graph, six-language UI, MCP management, interception controls, and test coverage. The initial import is commit `634e705`; the verification below covers the combined application and later feature work.

| Input | Inspected commit | Integrated capability |
| --- | --- | --- |
| ScopeWeaver | `a89795a` | Base application, task and asset graphs, planner and workers, interception, traffic store, notifications, model profiles, agent triggers, MCP, and six UI languages. |
| Artex-on-ChatGPT | `afcd6ca` | ChatGPT OAuth/PKCE connection, encrypted private account store, refresh and quota checks, account model catalog, Responses adapter, plan profiles and UI, browser sessions, image messages, and provider-data compaction fixes. |
| artex-browser-chat | `13fabbb` | Authenticated traffic extension, workflow canvas and scheduling, operator questions, encrypted test accounts, task outcomes, lessons and skill gaps, optional Telegram stdio MCP helper, and imported playbooks. |
| artex-deep-hunter | `abbe8e4` | Defensive detections and triage, bounded autonomous follow-up waves, cross-task confirmed finding hints, business map, hypothesis campaigns, opt-in hunt-class coverage board, attack-chain composition and verification tasks, scoped native request replay, stall watchdog, and focused hunting skills. |
| KSIR-ARTEX | `240ae7c` | Operator-managed proxy graph, local artifact manifest, C2 handoff records, scoped cloud assessment and lateral movement planning tools, and additional playbooks. |

## Security decisions

- ChatGPT account tokens stay in a dedicated local store and Docker volume; PostgreSQL holds account references. OAuth uses state, nonce, PKCE, issuer/JWKS validation, and a fixed loopback callback. Plan inference fails closed when the account is unavailable and does not silently use an API key. The local-only model policy blocks plan inference and catalog requests. The auth key has its own durable volume and startup fails if it is invalid or cannot be persisted.
- Browser profiles live outside browsable task workspaces. The browser traffic extension sends its token only to HTTPS or loopback HTTP addresses, and the ingest endpoint is authenticated and size-limited.
- Workflow schedules and graph size are bounded and validate their agent and timezone. The source fork's hardcoded public connector was removed.
- Follow-up waves are opt-in, delayed, time-boxed, and capped at three per root task. Cross-task hints use confirmed findings from the same scoped surface. Proposed attack chains require confirmed finding references; a separate verification task records a verified or rejected result. The hunt-class board is opt-in because many task types do not fit a full web vulnerability checklist. Open classes and untested campaign branches hold task completion once recorded.
- Proxy nodes are operator-defined; a failed validation does not fall back to direct target traffic. C2 handoff requires a manual task scope entry and an operator acknowledgement; it does not copy, deliver, or execute a binary. `exfil_data` indexes actual files in the task workspace with hashes and no outbound transfer. Cloud tools and the credential, movement, escalation, EDR, and cleanup lateral tools return scoped plans or inspect local evidence. `lateral_scan` now performs bounded TCP-connect measurements only for a host in operator-entered task scope, with interception-rule checks. No tool reports an unperformed action as successful.
- The optional Telegram plugin runs as a manually connected stdio MCP server. API credentials come from the environment, session files stay private, and `TELEGRAM_ALLOWED_CHATS` gates sending. The source's embedded API credential and unauthenticated HTTP wizard are absent.
- Native race, identity, and inflight request replay requires an operator-entered target scope, a passing asset interception gate, and an active recording proxy. The guard sends each replay call through interception, asking the operator by default and denying on timeout. Every distinct destination is checked before a batch starts, so scope checks do not stagger a race burst. Batches have request, body, and time limits; redirects are not followed, and missing capture fails closed. Guest identity replay strips credential headers, and candidate IDs are escaped inside URL, JSON, and form values.
- The capture proxy now verifies upstream target TLS certificates by default. An isolated lab can explicitly set `ARTEX_TRAFFIC_INSECURE_UPSTREAM=true` for a self-signed target. The imported origin, race, SSRF, and chaining playbooks were rewritten to use scoped evidence and bounded tools instead of direct raw-socket bursts, origin-range scans, or metadata-token retrieval.
- Source fork AMSI-bypass, reflective credential-loader, PHP web-shell/bulk-exploit, and WAF mutation files are preserved under `extras/source-assets/` for provenance. They are outside the runtime skill directory and excluded from the Docker build context. The active PHP and upload skills use inert-marker assessment workflows, the WAF skill respects rate limits and challenge responses, and CVE claims require current vendor advisory verification.
- KSIR's mock `deploy_payload`, `establish_persistence`, `lateral_move`, and `exfiltrate` success messages were not retained. Those source tools claimed outcomes without performing them. The handoff, assessment, and local artifact tools report only actual recorded work.
- Imported playbooks carry source attribution and local-scope notes. The dated CVE snapshot was replaced by a live-source playbook. The original release/update workflow does not point back to ScopeWeaver; release publication requires a reviewed Afterglow workflow.

## Verification on this checkout

| Check | Result |
| --- | --- |
| `go build ./...`; `go vet ./...` | Passed across the root application module. |
| `go test ./... -count=1` | Passed across all root packages against a fresh disposable PostgreSQL database selected by `ARTEX_TEST_DSN`; the database was removed afterward. This includes the native replay and ChatGPT packages. |
| `npm ci`; `npm test`; `npm run build:static` in `web/` | Passed; 84 frontend tests passed and the static export completed on the English-primary branch. |
| `node extension/background.test.mjs`; extension script syntax | Passed; all five extension tests passed. |
| `detections/tests/run-all.sh` | Passed all eight Docker suites: Sigma, Sigma matching, Sigma lint, backends, Suricata, ATT&CK, indicators, and MISP. The Sigma assertion now checks the actual ScopeWeaver self-update and guard strings in the integrated rules. |
| Telegram helper Python syntax | Passed with `python3 -m py_compile` during the initial import. |
| `node scripts/browser-runtime.test.cjs` | Passed using a temporary local Playwright/Chromium installation and the host's `certutil`. It verified isolated profiles, JS and screenshots, session restore, navigation, TLS rejection, trusted CA handling, and localhost proxy traversal. |

These checks exercise the application packages, frontend, extension, and detection content. The local Compose deployment has run with PostgreSQL, APTEX, and private Ollama using the existing host model read-only. Cold start and repeated start were verified. A real ChatGPT account sign-in and inference journey remains unverified.

## Independent client and server, 2026-11-05

APTEX now has its own `aptex`, `aptex-start`, and `aptex-stop` commands and
desktop entry. The full console opens with an independent browser profile.
The launcher owns only the APTEX Compose services, initializes only its own
administrator, and does not create or install an Odysseus gateway credential.
It retains the existing Docker project and data volumes. Eight launcher tests
passed, including lifecycle targeting, local model configuration, private
credential setup, and separate client/shortcut installation. A live repeated
start kept Odysseus's PID, its configuration, and all APTEX container IDs.

The earlier embedded Odysseus Cybersecurity mode completed a two-server browser
journey using a scoped gateway token, including mode switching, task creation,
and goal/constraint changes. That result describes the earlier interface;
the current architecture uses separate clients. The scoped gateway API remains
available for optional custom integrations.

The Odysseus gateway token additionally admits nine task-scoped Learning reads
(outcome, lessons, follow-up waves, indexed artifacts, C2 handoff summaries,
business map, campaign branches, hunt classes, and attack chains) plus lesson
extraction.
The new summary endpoint omits C2 shell URLs, and Odysseus filters the response
again before sending it to the browser. Handoff decisions, chain composition
and verification, and hunt-class edits remain
outside the token's allowlist. Gateway authorization tests and a desktop/mobile
Odysseus browser fixture cover the new view and lesson action.

## Application server boundaries, 2026-11-06

The [server review](server-architecture-review.md) inspected Odysseus revision
`7a7d6faea699` for its bootstrap, readiness, revocable session and detached
observation patterns. APTEX implements its own versioned API, persisted session
revocation, browser cookie migration and optional model-runtime startup in Go;
no Odysseus server source or runtime was imported. Existing PostgreSQL chat
admissions, history revisions, recovery and scoped gateway restrictions remain
authoritative. The new observer replays persisted active activities and does not
own execution. See the [client contract](client-api-v1.md).

Validation includes the complete root Go suite with a disposable database,
`go build ./...`, `go vet ./...`, 93 frontend tests, the static export and 12
launcher tests. Session tests cover independent devices, logout, password-change
races, legacy migration, origin checks and revocation of live observers. No
dependency, module identity or patched Norma replacement was changed.

## Independent iOS client, 2026-11-06

The native chat comparison also covers the current Odysseus message renderer,
document/photo import, model picker and adaptive iPad navigation. APTEX adapts
the Markdown/code segmentation and document-copy/Transferable patterns, retaining
their AGPL notices; its own API, upload cancellation and message-detail hydration
follow the APTEX server contract. See
[the client comparison](../clients/ios/odysseus-parity.md).

`clients/ios/` adapts the credential-vault pattern, path helper, SSE parsing, and
dependency-free Xcode/project-test script structure from
`afterglow-labs/odysseus-ios` revision
`f81c2cd09b90c49c308343c7569f795de2bf40c5`, under AGPL-3.0. Adapted files retain
source notices and the application includes the complete license. The existing
APTEX vector mark supplies the icon; native labels reuse matching web translations
and add context-specific translations for the new screens.

APTEX's iOS API, cybersecurity screens, persisted cursor handling, pending-input
recovery and full-console transport target this application's version 1 contract.
The client has a separate bundle identity, server-scoped Keychain entries and an
ephemeral WebKit store. The console receives an HttpOnly cookie directly, with
no credential in JavaScript or URL parameters. Native requests reject redirects
to another origin. No Odysseus server, accounts or runtime are prerequisites.

The server migration was merged and deployed before the client implementation
resumed. The existing Go engine, local Qwen installation, provider accounts and
patched Norma replacement remain intact. See the [client guide](../clients/ios/README.md)
and its verification record for native and real-backend checks.

## Restored red and purple workflow integration, 2026-11-07

The browser-chat fork's five workflow-management tools (`list_workflows`,
`create_workflow`, `update_workflow`, `delete_workflow`, `run_workflow`) are wired
to the same PostgreSQL handlers as the UI. Running a saved prompt now returns an
error if its conversation agent is unavailable, instead of reporting a false
success and updating its last-run timestamp. Availability resolves the selected
agent's own local or hosted profile even when no global profile is active.

**Run workflows** is a separate task-backed catalog with 29 red, purple, and
blue entries and 40 bundled skill references. Each entry has a specific
procedure, source revision, prerequisites, and five persisted phases. Admission
stores the operator's exact scope and phase goals in the same transaction as
the task before workers can run. A retry with the same request ID reuses that
task; a changed payload with that ID is rejected. Phase progress comes from the
task graph and requires a distinct linked confirmed fact or finding for each
phase in order. The
old twelve prompt starters remain under **Scheduled prompts**.

Installed imported skills are bound once to Worker and Pentest, with the
`agent-manager` skill bound to Auto. A migration marker per agent and skill
preserves later operator visibility changes. No model-format filter was added:
an existing local Ollama profile can be selected for the task, and existing
agent-specific routing keeps its precedence. This is application wiring; it
does not establish that every model will use every tool reliably.

The KSIR WAF and chaining skills now carry the source technique and chain
inventories with concrete baseline, variant, and verification steps. The
`lateral_scan` tool now makes actual bounded TCP-connect observations of
operator-scoped management ports; the remaining Windows movement tools still
require an external lab executor for live action. The UI reports BBOT, REA MCP,
Windows lab, and C2 handoff prerequisites when those paths are selected.
The defensive image includes the bundled detection files, pinned pySigma 2.0.0,
and a JSONL matcher that evaluates one atomic or correlation rule against
supplied events. It reports matched line numbers or a correlation result plus
the input hash, without echoing raw events. The source fixture suite exercises
both positive and negative cases. This reference matcher does not claim a
production SIEM collected or mapped those events.
The REA MCP is not bundled, BBOT is not installed by this Dockerfile, and a
Windows execution connector is not configured by the repository. A playbook
being installed does not mean those paths executed.

Verification used disposable PostgreSQL databases: the full root Go test suite,
Go build and vet, the patched Norma module tests, 101 frontend tests, TypeScript
checking, and the static frontend build passed. The eight Docker detection suites
passed, including positive and negative matcher cases. Browser smoke covered
catalog admission, retries, five-phase persistence, reload selection, and six
locales using a binary built with `-tags embedui`. An isolated task using the
installed `qwen3:32b` model made real planner and worker tool calls, including a
recorded fact; the 220-second observation window ended before task completion,
so that check does not establish end-to-end workflow completion or model speed.

The source KSIR `deploy_payload`, `establish_persistence`, `lateral_move`, and
`exfiltrate` handlers returned success text without carrying out those actions;
their names are not evidence of functionality. The archived PHP shells,
credential loaders, and automatic WAF rotation lists remain available as
source material outside the runtime image. The active runbooks use an exact
test target rather than the original race playbook's hardcoded public host,
and report actual evidence rather than treating a status code as proof.

## Afterglow appearance presets

APTEX adapts the Afterglow Dreamsicle, Afterglow Odysseus, and Afterglow
Orangurple palettes from Terminally's `ios/Terminally/TerminalThemes.json`
at `3dadac214102d3a3e3333afafd2c3e5b72c45647` and Odysseus's
`static/js/theme.js` at `448be9021ca3ec9f297a3ed045c8709005038b47`
(AGPL-3.0). The preset files retain both source credits. APTEX maps the
colors to its own light and dark interface roles, with text colors chosen
for contrast. The web console, including the Mac client's web view, uses
the full palettes. The iOS native picker uses the corresponding accent colors.

## Shared APTEX app icon

The Mac package uses the orange A artwork from the APTEX iOS 1.2.0 (build 3)
delivery, `APTEX_1.2.0_3_20261113-011855_d65625fb.ipa`. Both builds use
`clients/ios/APTEX/Resources/Assets.xcassets/AppIcon.appiconset/AppIcon.png`.
The Mac packager generates the required icon sizes from that PNG and retains
the 1024-pixel original in the icon set.
