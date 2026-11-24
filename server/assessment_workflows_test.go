package server

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/afterglow-labs/APTEX/db"
)

func assessmentTestUUID(t *testing.T) string {
	t.Helper()
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func TestAssessmentScopeNormalization(t *testing.T) {
	scopes, err := assessmentScopes([]string{"https://API.Example.test:8443/path?q=1", "api.example.test", "*.example.test", "192.0.2.10", "2001:db8::1", "192.0.2.99/24"})
	if err != nil {
		t.Fatal(err)
	}
	if len(scopes) != 5 || scopes[0].Kind != "subdomain" || scopes[0].Domain != "api.example.test" || scopes[1].Kind != "root_domain" || scopes[2].Net != "192.0.2.10/32" || scopes[3].Net != "2001:db8::1/128" || scopes[4].Net != "192.0.2.0/24" {
		t.Fatalf("bad exact scopes: %+v", scopes)
	}
	for _, bad := range []string{"", "https://user:password@example.test", "file:///etc/passwd", "https://example.test\\@other.test", "bad host.test", "*.127.0.0.1", "*.example.test/path", "https://bad_.test", "https://-bad.test"} {
		if _, err := assessmentScopes([]string{bad}); err == nil {
			t.Errorf("accepted invalid scope %q", bad)
		}
	}
}

func TestAssessmentCatalogSourceCoverage(t *testing.T) {
	if len(assessmentCatalog.Workflows) != 29 || len(assessmentCatalog.BundledSkills) != 40 {
		t.Fatal("catalog lost merged capabilities")
	}
	_, file, _, _ := runtime.Caller(0)
	skills := filepath.Join(filepath.Dir(file), "..", "skills")
	seen := map[string]bool{}
	for _, item := range assessmentCatalog.Workflows {
		if seen[item.ID] || item.Title == "" || len(item.Sources) == 0 || len(item.Procedure) < 3 {
			t.Fatalf("invalid blueprint: %+v", item)
		}
		seen[item.ID] = true
		if item.Team != "red" && item.Team != "purple" && item.Team != "blue" {
			t.Fatal(item.Team)
		}
		for _, name := range item.Skills {
			matches, err := filepath.Glob(filepath.Join(skills, name, "SKILL.md"))
			if err != nil || len(matches) != 1 {
				t.Fatalf("missing source skill %q", name)
			}
		}
		if len(assessmentPhases(item)) != 5 {
			t.Fatal(item.ID)
		}
	}
}

func assessmentHTTPFixture(t *testing.T) (*Server, func(string, string, any) *httptest.ResponseRecorder) {
	t.Helper()
	m := backportTestManager(t)
	_, file, _, _ := runtime.Caller(0)
	ctx, cancel := context.WithCancel(t.Context())
	s := New(ctx, m, filepath.Join(filepath.Dir(file), "..", "skills"), t.TempDir(), t.TempDir())
	// These tests isolate admission, transactions, and phase recovery. The owned
	// model/target integration check exercises execution separately.
	cancel()
	token, err := signJWT(s.jwtKey)
	if err != nil {
		t.Fatal(err)
	}
	handler := s.Handler()
	return s, func(method, path string, body any) *httptest.ResponseRecorder {
		raw, _ := json.Marshal(body)
		r := httptest.NewRequest(method, path, bytes.NewReader(raw))
		r.Header.Set("Authorization", "Bearer "+token)
		r.Header.Set("Content-Type", "application/json")
		out := httptest.NewRecorder()
		handler.ServeHTTP(out, r)
		return out
	}
}

func TestAssessmentAdmissionRetryAndPersistedPhases(t *testing.T) {
	s, request := assessmentHTTPFixture(t)
	nonce := assessmentTestUUID(t)
	input := map[string]any{"client_request_id": nonce, "targets": []string{"https://fixture.example.test:8443/api"}, "context": "fixture " + nonce, "timeout_seconds": 120}
	var before int
	if err := s.m.pg.QueryRow(`SELECT count(*) FROM tasks WHERE description LIKE '%' || $1 || '%'`, nonce).Scan(&before); err != nil {
		t.Fatal(err)
	}
	type reply struct {
		code int
		body map[string]any
	}
	replies := make(chan reply, 8)
	var group sync.WaitGroup
	for range 8 {
		group.Add(1)
		go func() {
			defer group.Done()
			out := request("POST", "/api/assessment-workflows/full-web/run", input)
			var body map[string]any
			_ = json.Unmarshal(out.Body.Bytes(), &body)
			replies <- reply{out.Code, body}
		}()
	}
	group.Wait()
	close(replies)
	var taskID string
	for r := range replies {
		if r.code != 200 && r.code != 201 && r.code != 202 {
			t.Fatalf("admission %d: %v", r.code, r.body)
		}
		task, ok := r.body["task"].(map[string]any)
		if !ok {
			t.Fatal(r.body)
		}
		id, _ := task["id"].(string)
		if id == "" || (taskID != "" && taskID != id) {
			t.Fatalf("duplicate admission: %s, %s", taskID, id)
		}
		taskID = id
	}
	id, err := strconv.ParseInt(taskID, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = s.m.DeleteTask(taskID, DeleteTaskOptions{}) })
	var after int
	if err := s.m.pg.QueryRow(`SELECT count(*) FROM tasks WHERE description LIKE '%' || $1 || '%'`, nonce).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if after-before != 1 {
		t.Fatalf("retry created %d tasks", after-before)
	}
	run, err := s.m.pg.GetAssessmentWorkflowRun(id)
	if err != nil || run == nil || len(run.GoalIDs) != 5 {
		t.Fatalf("missing phase snapshot: %+v %v", run, err)
	}
	task, ok := s.m.Task(taskID)
	if !ok {
		t.Fatal("task not registered")
	}
	scopes, err := s.m.Assets().ListTaskScopeWithSources(id)
	if err != nil {
		t.Fatal(err)
	}
	manual := false
	for _, scope := range scopes {
		if scope.Source == "manual" && scope.Domain == "fixture.example.test" {
			manual = true
		}
	}
	if !manual {
		t.Fatal("operator scope was not committed before execution")
	}
	specs := s.createGoals(context.Background(), task, nil)
	if len(specs) != 5 {
		t.Fatal("recovery replaced phase goals")
	}
	recovered, _ := s.m.pg.GetAssessmentWorkflowRun(id)
	for i, v := range recovered.GoalIDs {
		if v != run.GoalIDs[i] {
			t.Fatal("phase IDs changed on recovery")
		}
	}
	changed := map[string]any{}
	for key, value := range input {
		changed[key] = value
	}
	changed["context"] = "different inputs"
	if out := request("POST", "/api/assessment-workflows/full-web/run", changed); out.Code != 409 {
		t.Fatalf("UUID reused for changed inputs: %d %s", out.Code, out.Body)
	}
	if out := request("GET", "/api/assessment-workflow-runs", nil); out.Code != 200 {
		t.Fatal(out.Code, out.Body)
	}
}

func TestAssessmentPhaseOrderAndEvidence(t *testing.T) {
	s, request := assessmentHTTPFixture(t)
	out := request("POST", "/api/assessment-workflows/full-web/run", map[string]any{"client_request_id": assessmentTestUUID(t), "targets": []string{"fixture.example.test"}})
	if out.Code != 201 {
		t.Fatal(out.Code, out.Body)
	}
	var body struct {
		Task struct {
			ID string `json:"id"`
		} `json:"task"`
	}
	if err := json.Unmarshal(out.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	taskID, _ := strconv.ParseInt(body.Task.ID, 10, 64)
	task, _ := s.m.Task(body.Task.ID)
	t.Cleanup(func() { _, _ = s.m.DeleteTask(body.Task.ID, DeleteTaskOptions{}) })
	run, err := s.m.pg.GetAssessmentWorkflowRun(taskID)
	if err != nil {
		t.Fatal(err)
	}
	proof, err := task.Store.AddNode(db.KindFact, map[string]any{"text": "Synthetic fixture evidence; no target test performed"}, 0, "confirmed", "test", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := task.Store.ProveGoal(run.GoalIDs[1], proof); err == nil {
		t.Fatal("phase order was bypassed")
	}
	if err := task.Store.UpdateGoalPayload(run.GoalIDs[1], "Operator-edited phase text", ""); err != nil {
		t.Fatal(err)
	}
	if err := task.Store.ProveGoal(run.GoalIDs[1], proof); err == nil {
		t.Fatal("editing text erased phase order")
	}
	other, err := s.m.CreateTask("unrelated fixture", "evidence must not leak", nil, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = s.m.DeleteTask(other.ID, DeleteTaskOptions{}) })
	foreign, _ := other.Store.AddNode(db.KindFact, map[string]any{"text": "Unrelated evidence"}, 0, "confirmed", "test", nil)
	if err := task.Store.ProveGoal(run.GoalIDs[0], foreign); err == nil {
		t.Fatal("unrelated task proved a phase")
	}
	if err := task.Store.ProveGoal(run.GoalIDs[0], proof); err != nil {
		t.Fatal(err)
	}
	if err := task.Store.ProveGoal(run.GoalIDs[1], proof); err == nil {
		t.Fatal("one fact proved two workflow phases")
	}
	for i, goal := range run.GoalIDs[1:4] {
		phaseProof, err := task.Store.AddNode(db.KindFact, map[string]any{"text": fmt.Sprintf("Distinct synthetic phase %d outcome", i+1)}, 0, "confirmed", "test", nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := task.Store.ProveGoal(goal, phaseProof); err != nil {
			t.Fatal(err)
		}
	}
	if ready, err := task.Store.AssessmentWorkflowComplete(); err != nil || ready {
		t.Fatal("incomplete workflow reported complete", ready, err)
	}
	finalProof, err := task.Store.AddNode(db.KindFact, map[string]any{"text": "Distinct synthetic report outcome"}, 0, "confirmed", "test", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := task.Store.ProveGoal(run.GoalIDs[4], finalProof); err != nil {
		t.Fatal(err)
	}
	if ready, err := task.Store.AssessmentWorkflowComplete(); err != nil || !ready {
		t.Fatal("all five distinct phases did not complete", ready, err)
	}
	if err := task.Store.DeleteGoal(run.GoalIDs[4]); err != nil {
		t.Fatal(err)
	}
	if ready, err := task.Store.AssessmentWorkflowComplete(); err != nil || ready {
		t.Fatal("deleted phase reported complete", ready, err)
	}
}

func TestRestoredWorkflowToolsCRUDAndUnavailableRun(t *testing.T) {
	s, _ := assessmentHTTPFixture(t)
	tools := names(s.workflowTools())
	call := func(key string, input map[string]any) (string, bool) {
		t.Helper()
		raw, _ := json.Marshal(input)
		result, err := tools[key].Call(context.Background(), raw, nil)
		if err != nil {
			t.Fatal(err)
		}
		return result.Flatten(), result.IsError
	}
	if len(tools) != 5 {
		t.Fatalf("restored %d workflow tools, want 5", len(tools))
	}
	name := "fixture workflow " + assessmentTestUUID(t)
	createdText, failed := call("create_workflow", map[string]any{
		"name": name, "prompt": "Review owned fixture evidence", "enabled": false, "schedule_kind": "manual", "agent_key": "auto",
	})
	if failed {
		t.Fatal(createdText)
	}
	var created db.Workflow
	if err := json.Unmarshal([]byte(createdText), &created); err != nil || created.ID <= 0 {
		t.Fatal("create_workflow returned no stored workflow", err, createdText)
	}
	t.Cleanup(func() { _ = s.m.pg.DeleteWorkflow(created.ID) })
	if created.Name != name || created.Enabled || created.ScheduleKind != "manual" {
		t.Fatalf("unexpected workflow defaults: %+v", created)
	}
	listed, failed := call("list_workflows", map[string]any{})
	if failed || !strings.Contains(listed, name) {
		t.Fatal("list_workflows omitted the created item", listed)
	}
	updated, failed := call("update_workflow", map[string]any{"id": created.ID, "prompt": "Preserve this edit"})
	if failed || !strings.Contains(updated, "Preserve this edit") {
		t.Fatal("update_workflow did not persist edit", updated)
	}
	if run, failed := call("run_workflow", map[string]any{"id": created.ID}); !failed || !strings.Contains(run, "unavailable") {
		t.Fatal("run_workflow reported success without an available agent", run)
	}
	stored, err := s.m.pg.GetWorkflow(created.ID)
	if err != nil || stored.LastFire != nil {
		t.Fatal("unavailable workflow was marked fired", err, stored)
	}
	if _, failed := call("update_workflow", map[string]any{"id": created.ID, "schedule_kind": "invalid"}); !failed {
		t.Fatal("update_workflow bypassed schedule validation")
	}
	if deleted, failed := call("delete_workflow", map[string]any{"id": created.ID}); failed || !strings.Contains(deleted, "deleted") {
		t.Fatal("delete_workflow failed", deleted)
	}
	if _, err := s.m.pg.GetWorkflow(created.ID); err == nil {
		t.Fatal("deleted workflow remained in the database")
	}
}

func TestWorkflowAgentProfileWithoutGlobalModel(t *testing.T) {
	t.Setenv("APTEX_LLM_PROVIDER", "")
	t.Setenv("ARTEX_LLM_PROVIDER", "")
	m := backportTestManager(t)
	_, file, _, _ := runtime.Caller(0)
	s := New(t.Context(), m, filepath.Join(filepath.Dir(file), "..", "skills"), t.TempDir(), t.TempDir())
	if s.chatAgentRef() != nil {
		t.Fatal("fixture unexpectedly configured a global model")
	}
	if s.triggerAgentAvailable("auto") {
		t.Fatal("agent without a model was admitted")
	}
	id, err := m.pg.SaveProfile(&db.LLMProfile{Name: "workflow-bound-fixture-" + assessmentTestUUID(t), Format: "ollama", Model: "fixture:latest", BaseURL: "http://127.0.0.1:1", ContextWindowK: 16})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = m.pg.SetAgentLLMProfile("auto", nil)
		_ = m.pg.DeleteProfile(id)
	})
	if err := m.pg.SetAgentLLMProfile("auto", &id); err != nil {
		t.Fatal(err)
	}
	if !s.triggerAgentAvailable("auto") {
		t.Fatal("agent-bound local model was rejected without a global model")
	}
}

func TestMergedSkillBindingPreservesLaterOptOut(t *testing.T) {
	s, _ := assessmentHTTPFixture(t)
	worker, err := s.m.pg.GetAgentByKey("worker")
	if err != nil {
		t.Fatal(err)
	}
	original, _ := s.m.pg.AgentSkillNames(worker.ID)
	name := "fixture-source-" + assessmentTestUUID(t)
	marker := "merged_skill_binding_v1.worker." + name
	t.Cleanup(func() {
		_ = s.m.pg.SetAgentSkillVisibility(worker.ID, original)
		_, _ = s.m.pg.Exec(`DELETE FROM settings WHERE key=$1`, marker)
	})
	bindings := map[string][]string{"worker": {name}}
	if err := s.m.pg.SeedMergedSkillBindings(bindings); err != nil {
		t.Fatal(err)
	}
	if err := s.m.pg.ToggleSkillVisibility(worker.ID, name, false); err != nil {
		t.Fatal(err)
	}
	if err := s.m.pg.SeedMergedSkillBindings(bindings); err != nil {
		t.Fatal(err)
	}
	names, _ := s.m.pg.AgentSkillNames(worker.ID)
	for _, value := range names {
		if value == name {
			t.Fatal("operator opt-out was overwritten on restart")
		}
	}
}
