package agent

import (
	"context"
	"encoding/json"
	"net"
	"strconv"
	"strings"
	"time"

	actool "github.com/Autumn-27/norma/tool"
	"github.com/afterglow-labs/afterglow-cybersecurity/db"
)

func validPlanMethod(method string, allowed ...string) bool {
	for _, candidate := range allowed {
		if method == candidate {
			return true
		}
	}
	return false
}

func (t *ToolSet) lateralScan() actool.CoreTool {
	return writeTool("lateral_scan", "Run a bounded TCP connect inventory of a host in operator-entered task scope. Returns observed open ports; it does not attempt authentication or remote commands.",
		obj(map[string]any{"target": str("host or IP in operator-entered task scope"), "ports": map[string]any{"type": "array", "items": map[string]any{"type": "integer", "minimum": 1, "maximum": 65535}, "maxItems": 12}}, "target"),
		func(ctx context.Context, input json.RawMessage) (actool.Result, error) {
			var a struct {
				Target string `json:"target"`
				Ports  []int  `json:"ports"`
			}
			if json.Unmarshal(input, &a) != nil {
				return actool.Errorf("invalid input"), nil
			}
			target, ok := t.targetInOperatorScope(a.Target)
			if !ok {
				return actool.Errorf("target is outside operator-entered task scope"), nil
			}
			resolved := target
			if net.ParseIP(target) == nil {
				lookupCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
				addresses, err := net.DefaultResolver.LookupIPAddr(lookupCtx, target)
				cancel()
				if err != nil || len(addresses) == 0 || len(addresses) > 16 {
					return actool.Errorf("target DNS resolution failed or returned too many addresses"), nil
				}
				// Pin one address for the whole inventory. A second resolution at
				// dial time could move an approved hostname to a blocked address.
				resolved = addresses[0].IP.String()
				ip := addresses[0].IP
				if ip.IsUnspecified() || ip.IsMulticast() {
					return actool.Errorf("target DNS resolved to an unusable address"), nil
				}
				if (ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()) && !t.operatorIPInScope(ip) {
					return actool.Errorf("private or local resolved address requires its own operator-entered IP scope"), nil
				}
			}
			global, err := t.as.ListAssetInterceptRules()
			if err != nil {
				return actool.Errorf("interception rules unavailable"), nil
			}
			blocked, allowed, err := t.as.TaskInterceptRulesSplit(t.taskID)
			if err != nil {
				return actool.Errorf("task interception rules unavailable"), nil
			}
			var domains []string
			if net.ParseIP(target) == nil {
				domains = []string{target}
			}
			if !db.EvaluateAssetGate(append(global, blocked...), allowed, domains, []string{resolved}, nil).Allowed {
				return actool.Errorf("target is blocked by interception rules"), nil
			}
			ports := a.Ports
			if len(ports) == 0 {
				ports = []int{22, 135, 139, 389, 445, 636, 3389, 5985, 5986}
			}
			if len(ports) > 12 {
				return actool.Errorf("at most 12 ports per scan"), nil
			}
			seen := map[int]bool{}
			results := make([]map[string]any, 0, len(ports))
			for _, port := range ports {
				if port < 1 || port > 65535 || seen[port] {
					return actool.Errorf("ports must be unique integers from 1 to 65535"), nil
				}
				seen[port] = true
			}
			for _, port := range ports {
				probeCtx, cancel := context.WithTimeout(ctx, 750*time.Millisecond)
				conn, err := (&net.Dialer{}).DialContext(probeCtx, "tcp", net.JoinHostPort(resolved, strconv.Itoa(port)))
				cancel()
				state := "not_open"
				if err == nil {
					state = "open"
					_ = conn.Close()
				}
				results = append(results, map[string]any{"port": port, "state": state})
				if ctx.Err() != nil {
					return actool.Errorf("scan cancelled"), nil
				}
			}
			return jsonResult(map[string]any{"target": target, "resolved_ip": resolved, "method": "tcp_connect", "ports": results, "observed_at": time.Now().UTC().Format(time.RFC3339), "authentication_attempted": false, "remote_command_attempted": false})
		})
}

func (t *ToolSet) operatorIPInScope(ip net.IP) bool {
	if t.as == nil || t.taskID <= 0 {
		return false
	}
	rows, err := t.as.ListTaskScopeWithSources(t.taskID)
	if err != nil {
		return false
	}
	for _, scope := range rows {
		if scope.Source != "manual" || (scope.Kind != "ip" && scope.Kind != "cidr") {
			continue
		}
		_, network, err := net.ParseCIDR(scope.Net)
		if err == nil && network.Contains(ip) {
			return true
		}
	}
	return false
}

func (t *ToolSet) harvestCredentials() actool.CoreTool {
	return readTool("harvest_credentials", "Plan credential exposure verification for a scoped host. Does not collect, display, or save secrets; record only real test accounts with save_task_credential.",
		obj(map[string]any{"target": str("scoped host"), "method": str("lsass, sam, ntds, dpapi, kerberos, files, or history")}, "target", "method"),
		func(_ context.Context, input json.RawMessage) (actool.Result, error) {
			var a struct {
				Target string `json:"target"`
				Method string `json:"method"`
			}
			if json.Unmarshal(input, &a) != nil {
				return actool.Errorf("invalid input"), nil
			}
			target, ok := t.targetInDeclaredScope(a.Target)
			if !ok {
				return actool.Errorf("target is outside this task's declared scope"), nil
			}
			method := strings.ToLower(strings.TrimSpace(a.Method))
			if !validPlanMethod(method, "lsass", "sam", "ntds", "dpapi", "kerberos", "files", "history") {
				return actool.Errorf("unsupported collection method"), nil
			}
			return jsonResult(map[string]any{"target": target, "method": method, "checks": []string{"Confirm the target and credential material are covered by the engagement", "Prefer metadata and exposure proof over copying credential contents", "Use save_task_credential only for an actual authorized test account", "Keep observed secrets out of tool responses and findings"}, "credentials_saved": 0, "status": "plan_only"})
		})
}

func (t *ToolSet) moveLateral() actool.CoreTool {
	return readTool("move_lateral", "Review a scoped access path using a real task credential reference. Does not execute a remote command or reveal the credential.",
		obj(map[string]any{"target": str("scoped host"), "method": str("wmi, winrm, smb, ssh, psexec, dcom, or rdp"), "credential_id": intp("existing credential ID for this task")}, "target", "method", "credential_id"),
		func(_ context.Context, input json.RawMessage) (actool.Result, error) {
			var a struct {
				Target       string `json:"target"`
				Method       string `json:"method"`
				CredentialID int64  `json:"credential_id"`
			}
			if json.Unmarshal(input, &a) != nil {
				return actool.Errorf("invalid input"), nil
			}
			target, ok := t.targetInDeclaredScope(a.Target)
			if !ok {
				return actool.Errorf("target is outside this task's declared scope"), nil
			}
			method := strings.ToLower(strings.TrimSpace(a.Method))
			if !validPlanMethod(method, "wmi", "winrm", "smb", "ssh", "psexec", "dcom", "rdp") {
				return actool.Errorf("unsupported access method"), nil
			}
			if t.ts == nil || t.taskID <= 0 || a.CredentialID <= 0 {
				return actool.Errorf("task credential is required"), nil
			}
			exists, err := t.ts.HasTaskCredential(t.taskID, a.CredentialID)
			if err != nil || !exists {
				return actool.Errorf("credential does not belong to this task"), nil
			}
			return jsonResult(map[string]any{"target": target, "method": method, "credential_id": a.CredentialID, "checks": []string{"Confirm this account is allowed on the target", "Verify access with the least intrusive approved method", "Record observed access and identity without placing the password in the transcript"}, "status": "plan_only"})
		})
}

func (t *ToolSet) escalatePrivilege() actool.CoreTool {
	return readTool("escalate_privilege", "Review a scoped privilege escalation hypothesis. Does not execute a payload or assert elevated access.",
		obj(map[string]any{"target": str("scoped host"), "method": str("token_steal, service_hijack, uac_bypass, potato, or kernel")}, "target", "method"),
		func(_ context.Context, input json.RawMessage) (actool.Result, error) {
			var a struct {
				Target string `json:"target"`
				Method string `json:"method"`
			}
			if json.Unmarshal(input, &a) != nil {
				return actool.Errorf("invalid input"), nil
			}
			target, ok := t.targetInDeclaredScope(a.Target)
			if !ok {
				return actool.Errorf("target is outside this task's declared scope"), nil
			}
			method := strings.ToLower(strings.TrimSpace(a.Method))
			if !validPlanMethod(method, "token_steal", "service_hijack", "uac_bypass", "potato", "kernel") {
				return actool.Errorf("unsupported escalation method"), nil
			}
			return jsonResult(map[string]any{"target": target, "method": method, "checks": []string{"Identify the specific prerequisite and affected configuration", "Use a reversible proof within the authorized scope", "Confirm resulting identity before reporting escalation", "Record cleanup steps for any test artifact"}, "status": "plan_only"})
		})
}

func (t *ToolSet) domainEscalate() actool.CoreTool {
	return readTool("domain_escalate", "Review a scoped domain privilege path. Does not modify domain policy, request tickets, or assert compromise.",
		obj(map[string]any{"domain": str("domain in task scope"), "method": str("dcsync, golden_ticket, kerberoast, asrep_roast, or gpo")}, "domain", "method"),
		func(_ context.Context, input json.RawMessage) (actool.Result, error) {
			var a struct {
				Domain string `json:"domain"`
				Method string `json:"method"`
			}
			if json.Unmarshal(input, &a) != nil {
				return actool.Errorf("invalid input"), nil
			}
			domain, ok := t.targetInDeclaredScope(a.Domain)
			if !ok {
				return actool.Errorf("domain is outside this task's declared scope"), nil
			}
			method := strings.ToLower(strings.TrimSpace(a.Method))
			if !validPlanMethod(method, "dcsync", "golden_ticket", "kerberoast", "asrep_roast", "gpo") {
				return actool.Errorf("unsupported domain method"), nil
			}
			return jsonResult(map[string]any{"domain": domain, "method": method, "checks": []string{"Map the prerequisite privileges and trust boundary", "Inspect relevant policy and account exposure", "Use approved non-destructive verification", "Record direct evidence before claiming domain impact"}, "status": "plan_only"})
		})
}

func (t *ToolSet) checkEDR() actool.CoreTool {
	return readTool("check_edr", "Prepare a scoped endpoint protection inventory. No agent detection claim is made without observed data.",
		obj(map[string]any{"target": str("scoped host")}, "target"),
		func(_ context.Context, input json.RawMessage) (actool.Result, error) {
			var a struct {
				Target string `json:"target"`
			}
			if json.Unmarshal(input, &a) != nil {
				return actool.Errorf("invalid input"), nil
			}
			target, ok := t.targetInDeclaredScope(a.Target)
			if !ok {
				return actool.Errorf("target is outside this task's declared scope"), nil
			}
			return jsonResult(map[string]any{"target": target, "checks": []string{"Inspect approved endpoint management inventory", "Record observed service names and sensor health", "Avoid changing or disabling protection during assessment"}, "detected": "unknown", "status": "plan_only"})
		})
}

func (t *ToolSet) cleanupArtifacts() actool.CoreTool {
	return readTool("cleanup_artifacts", "Prepare a scoped cleanup record for test artifacts. Does not delete files or tamper with logs.",
		obj(map[string]any{"target": str("scoped host"), "paths": map[string]any{"type": "array", "items": str("absolute test artifact path")}}, "target", "paths"),
		func(_ context.Context, input json.RawMessage) (actool.Result, error) {
			var a struct {
				Target string   `json:"target"`
				Paths  []string `json:"paths"`
			}
			if json.Unmarshal(input, &a) != nil {
				return actool.Errorf("invalid input"), nil
			}
			target, ok := t.targetInDeclaredScope(a.Target)
			if !ok {
				return actool.Errorf("target is outside this task's declared scope"), nil
			}
			if len(a.Paths) == 0 || len(a.Paths) > 30 {
				return actool.Errorf("provide 1–30 test artifact paths"), nil
			}
			for _, path := range a.Paths {
				if !validC2TargetPath(path) {
					return actool.Errorf("invalid test artifact path"), nil
				}
			}
			return jsonResult(map[string]any{"target": target, "paths": a.Paths, "status": "review_required", "deleted": false, "note": "Operator should verify ownership and remove only test-created artifacts; preserve audit logs."})
		})
}
