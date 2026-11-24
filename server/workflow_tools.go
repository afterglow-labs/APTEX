package server

// Restores artex-browser-chat 13fabbb's five workflow management tools.
// Use the application handlers so agents and clients have identical validation,
// timezone handling, graph limits, scheduling, and execution behavior.
import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	actool "github.com/Autumn-27/norma/tool"
	"github.com/afterglow-labs/APTEX/locale"
)

type workflowToolReply struct {
	header http.Header
	body   bytes.Buffer
	status int
}

func (w *workflowToolReply) Header() http.Header    { return w.header }
func (w *workflowToolReply) WriteHeader(status int) { w.status = status }
func (w *workflowToolReply) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.body.Write(data)
}

func (s *Server) workflowTools(langs ...locale.Lang) []actool.CoreTool {
	lang := locale.First(langs)
	properties := map[string]any{
		"name":          map[string]any{"type": "string", "maxLength": 160},
		"description":   map[string]any{"type": "string", "maxLength": 4000},
		"agent_key":     map[string]any{"type": "string"},
		"prompt":        map[string]any{"type": "string", "maxLength": 16000},
		"enabled":       map[string]any{"type": "boolean"},
		"schedule_kind": map[string]any{"type": "string", "enum": []string{"manual", "interval", "daily"}},
		"interval_sec":  map[string]any{"type": "integer", "minimum": 60},
		"daily_hour":    map[string]any{"type": "integer", "minimum": 0, "maximum": 23},
		"daily_minute":  map[string]any{"type": "integer", "minimum": 0, "maximum": 59},
		"timezone":      map[string]any{"type": "string"},
		"graph_json":    map[string]any{"type": "object"},
	}
	makeTool := func(key, description, method string, withID, fields bool, required []string, handler http.HandlerFunc) actool.CoreTool {
		props := map[string]any{}
		if fields {
			for k, v := range properties {
				props[k] = v
			}
		}
		if withID {
			props["id"] = map[string]any{"type": "integer", "minimum": 1}
			required = append(required, "id")
		}
		call := func(ctx context.Context, raw json.RawMessage) (actool.Result, error) {
			if len(raw) > 192*1024 {
				return actool.Errorf("workflow input exceeds 192 KiB"), nil
			}
			var input map[string]json.RawMessage
			if err := json.Unmarshal(raw, &input); err != nil || input == nil {
				return actool.Errorf("invalid workflow input"), nil
			}
			id := int64(0)
			if withID {
				id = parseProfileID(input["id"])
				if id <= 0 {
					return actool.Errorf("invalid workflow id"), nil
				}
			}
			delete(input, "id")
			body, err := json.Marshal(input)
			if err != nil {
				return actool.Errorf(err.Error()), nil
			}
			r, err := http.NewRequestWithContext(ctx, method, "/api/workflows", bytes.NewReader(body))
			if err != nil {
				return actool.Errorf(err.Error()), nil
			}
			r.Header.Set("Content-Type", "application/json")
			if withID {
				r.SetPathValue("id", strconv.FormatInt(id, 10))
			}
			out := &workflowToolReply{header: http.Header{}}
			handler(out, r)
			if out.status >= 400 {
				return actool.Errorf(out.body.String()), nil
			}
			return actool.Text(out.body.String()), nil
		}
		if method == http.MethodGet {
			return roTool(key, locale.Text(lang, description), objSchema(props, required...), call)
		}
		return wrTool(key, locale.Text(lang, description), objSchema(props, required...), call)
	}
	return []actool.CoreTool{
		makeTool("list_workflows", "List saved workflows, schedules, agents, instructions, and last execution times.", http.MethodGet, false, false, nil, s.pgListWorkflows),
		makeTool("create_workflow", "Create a workflow with an agent, instructions, and a manual, interval, or daily schedule.", http.MethodPost, false, true, []string{"name", "prompt"}, s.pgCreateWorkflow),
		makeTool("update_workflow", "Update a saved workflow's instructions, agent, schedule, timezone, or enabled state.", http.MethodPatch, true, true, nil, s.pgUpdateWorkflow),
		makeTool("delete_workflow", "Delete a saved workflow by its identifier.", http.MethodDelete, true, false, nil, s.pgDeleteWorkflow),
		makeTool("run_workflow", "Run a saved workflow now using its configured agent and instructions.", http.MethodPost, true, false, nil, s.pgRunWorkflow),
	}
}
