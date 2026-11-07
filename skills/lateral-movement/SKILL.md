---
name: lateral-movement
description: "Scoped management-path discovery, designated identity review, and Windows lab validation"
---

# Lateral movement assessment

Adapted from the KSIR-ARTEX lateral movement playbook. `lateral_scan` now runs a bounded TCP-connect inventory against a host in operator-entered task scope, honoring task interception rules. The other named tools review prerequisites or local evidence; they do not authenticate to another host, dump credentials, disable EDR, erase logs, or clean a target. Never describe their plans as completed actions.

1. Confirm the task's authorized hosts, network range, accounts, methods, and time window. A target outside a manually declared scope must not be used for a handoff.
2. Use `lateral_scan` for up to 12 explicitly selected TCP ports on one scoped host. The default ports cover SSH, RPC, NetBIOS, LDAP, SMB, LDAPS, RDP, WinRM HTTP, and WinRM HTTPS. It pins a hostname's resolved IP for the scan and requires separate operator-entered IP scope when that address is private or local. Save the returned address, open-port observations, and timestamp; a successful TCP connection proves reachability, not authentication or vulnerability. The tool sends TCP connects directly, not through the HTTP recording proxy, so apply the engagement's egress controls and test window.
3. Compare the actual services with the operator's expected management paths. For each host, map the protocol, designated account, expected authorization boundary, and available sensor events. An open port is a candidate path only.
4. Use the encrypted task account store for designated test credentials. `harvest_credentials` describes potential exposure paths but must not return password or token values. Reference a credential by its task-scoped ID when preparing a plan.
5. Use `move_lateral`, `escalate_privilege`, or `domain_escalate` to record an authorized hypothesis, prerequisites, target, and expected evidence. A Windows lab connector is needed for a live movement test; record its actual output and correlated host events before claiming access.
6. Use `check_edr` to document observed protection status. The tool does not bypass or disable a product.
7. Use `cleanup_artifacts` to inventory test files for operator review. Keep system audit logs and finding evidence intact. Confirm cleanup from independent evidence before recording it as complete.

When reporting, distinguish **proposed**, **attempted**, and **verified** outcomes. Include the exact scoped host and evidence reference. Avoid raw credential values, service tokens, and unverified success claims in model-visible output.
