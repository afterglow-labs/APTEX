---
name: vuln-chaining
description: "Compose and verify multi-step attack paths from confirmed findings and exact prerequisites"
---

# Vulnerability chaining

Adapted from the KSIR-ARTEX chain playbook (`240ae7c`) and the Deep Hunter attack-chain board (`abbe8e4`). A chain is a graph of observed findings. Each edge must show that one result supplies the next step's exact prerequisite for the same test identity and authorized scope. A list of plausible attacks is a hypothesis, not demonstrated impact.

## Build the chain

1. Use `attack_chain` with `action=list` to review existing chains, then gather confirmed findings, request evidence, affected assets, roles, and limitations from the current task or explicitly linked source tasks.
2. For each step record **input → action → observed output → next prerequisite**. Mark a missing identity, host, credential, permission, or tool as a gap; do not silently substitute it.
3. Use `attack_chain` with `action=add` for two or more evidence-linked findings. Put the predicted end-to-end impact in the hypothesis field. The source's dated CVE examples are leads only; confirm affected versions and behavior with current vendor advisories and the scoped target.
4. Launch the chain board's verification task for a worthwhile proposed chain. Reproduce each transition with fresh, disposable data and save the request, resulting state, and negative control. Close the chain as verified only with end-to-end evidence, or rejected with the broken step.

## Source chain patterns and required proof

| Pattern | Prerequisites to establish | End-to-end proof |
| --- | --- | --- |
| Object authorization → account recovery | A designated low-privilege account can change another **test** account's recovery field, and a reset is deliverable to an operator-owned mailbox | The owned test account's recovery state changes and a reset succeeds under the test conditions; a normal denied control does not. |
| SSRF → internal service | A server-side fetch is confirmed by an owned canary; a specific internal destination is explicitly in scope | A unique marker reaches the scoped internal service and the resulting application behavior is observed. Reachability alone is not command execution. |
| Upload → inclusion or execution | The application stores an inert test file and a separate route can retrieve or include it | The server actually interprets a harmless marker in an isolated environment. Extension acceptance or a URL alone is not RCE. |
| Cloud trust → privilege | An exact issuer, audience, subject, and role assignment are known, with an operator-controlled test principal | The role can be assumed by that principal and a bounded read-only operation succeeds. A broad policy string alone is not assumability. |
| Client artifact → backend API | An authorized app package exposes a real endpoint or test-only credential, and that credential is still valid in the declared environment | A designated test identity reaches a backend action and the resulting state is confirmed. Static strings alone are not access. |
| Cache or WAF mismatch → cross-user action | The same resource is shared across test accounts and a specific normalization or cache-key difference is observed | A second designated test account receives data or an action it should not, with request and cache or edge IDs. |

Record the role, host, and time boundary for every transition. Use the existing task account store for test credentials, and cite evidence IDs rather than pasting secrets into a chain. A chain may stop at any step; preserve the partial result and exact blocked prerequisite. Report remediation for the earliest reliable boundary, then describe retests for all downstream links.
