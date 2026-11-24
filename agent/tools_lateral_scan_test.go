package agent

import (
	"encoding/json"
	"net"
	"strconv"
	"strings"
	"testing"

	"github.com/afterglow-labs/APTEX/db"
)

func TestLateralScanObservedPortAndScopeGate(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port
	d := testDB(t)
	defer d.Close()
	task, err := d.CreateTaskWithOptions("owned local fixture", "inventory one port", db.TaskCreateOptions{
		OperatorScopes: []db.TaskScope{{Kind: "ip", Net: "127.0.0.1/32"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.DeleteTask(task.ID) })
	tools := NewToolSet(d.Exploration(task.ExplorationID), "worker")
	tools.SetTaskID(task.ID)
	tools.SetAssetStore(d.Assets(), d.Companies())
	call := func(target string) (string, bool) {
		raw, _ := json.Marshal(map[string]any{"target": target, "ports": []int{port}})
		result, err := tools.lateralScan().Call(t.Context(), raw, nil)
		if err != nil {
			t.Fatal(err)
		}
		return result.Flatten(), result.IsError
	}
	observed, failed := call("127.0.0.1")
	if failed || !strings.Contains(observed, `"state":"open"`) || !strings.Contains(observed, strconv.Itoa(port)) {
		t.Fatal("owned listener was not observed", observed)
	}
	if denied, failed := call("127.0.0.2"); !failed || !strings.Contains(denied, "operator-entered") {
		t.Fatal("outside address escaped manual scope", denied)
	}
	if _, err := d.Assets().CreateTaskInterceptRule(task.ID, "block", "exact_ip", "127.0.0.1", "fixture block", true); err != nil {
		t.Fatal(err)
	}
	if denied, failed := call("127.0.0.1"); !failed || !strings.Contains(denied, "interception") {
		t.Fatal("interception rule did not block TCP scan", denied)
	}
}

func TestLateralScanLocalResolvedAddressNeedsIPScope(t *testing.T) {
	d := testDB(t)
	defer d.Close()
	task, err := d.CreateTaskWithOptions("owned hostname fixture", "check DNS target pinning", db.TaskCreateOptions{
		OperatorScopes: []db.TaskScope{{Kind: "subdomain", Domain: "localhost"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.DeleteTask(task.ID) })
	tools := NewToolSet(d.Exploration(task.ExplorationID), "worker")
	tools.SetTaskID(task.ID)
	tools.SetAssetStore(d.Assets(), d.Companies())
	raw, _ := json.Marshal(map[string]any{"target": "localhost", "ports": []int{1}})
	result, err := tools.lateralScan().Call(t.Context(), raw, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError || !strings.Contains(result.Flatten(), "operator-entered IP scope") {
		t.Fatal("hostname reached a local IP without explicit IP scope", result.Flatten())
	}
}
