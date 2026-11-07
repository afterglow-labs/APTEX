package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// AssessmentWorkflowInput is a creation-time snapshot. Later catalog edits do
// not change the phases, provenance, or targets of an existing assessment.
type AssessmentWorkflowInput struct {
	RequestID     string            `json:"request_id,omitempty"`
	RequestHash   string            `json:"-"`
	Key           string            `json:"key"`
	Title         string            `json:"title"`
	Team          string            `json:"team"`
	Skills        []string          `json:"skills"`
	Sources       []string          `json:"sources"`
	Procedure     []string          `json:"procedure"`
	Prerequisites []string          `json:"prerequisites"`
	Phases        []AssessmentPhase `json:"phases"`
	Targets       []string          `json:"targets"`
}

type AssessmentPhase struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Goal  string `json:"goal"`
}

type AssessmentWorkflowRun struct {
	RequestID   string                  `json:"request_id,omitempty"`
	RequestHash string                  `json:"-"`
	TaskID      int64                   `json:"task_id,string"`
	Definition  AssessmentWorkflowInput `json:"definition"`
	GoalIDs     []int64                 `json:"goal_ids"`
	CreatedAt   time.Time               `json:"created_at"`
}

func insertAssessmentWorkflow(tx *sql.Tx, taskID, explorationID int64, input *AssessmentWorkflowInput, scopes []TaskScope) error {
	for _, scope := range scopes {
		if scope.Kind != "subdomain" && scope.Kind != "root_domain" && scope.Kind != "ip" && scope.Kind != "cidr" {
			return fmt.Errorf("unsupported operator scope kind")
		}
		if _, err := tx.Exec(`INSERT INTO task_scope(task_id,kind,domain,net,source,reason)
VALUES ($1,$2,NULLIF($3,''),NULLIF($4,'')::cidr,'manual','Operator supplied at workflow launch') ON CONFLICT DO NOTHING`, taskID, scope.Kind, scope.Domain, scope.Net); err != nil {
			return err
		}
	}
	if input == nil {
		return nil
	}
	if input.Key == "" || len(input.Phases) == 0 || len(input.Phases) > 16 {
		return fmt.Errorf("invalid assessment workflow")
	}
	goalIDs := make([]int64, 0, len(input.Phases))
	for i, phase := range input.Phases {
		payload := map[string]any{"text": phase.Title + ": " + phase.Goal, "workflow_phase": phase.ID, "workflow_key": input.Key, "workflow_phase_index": i}
		if i > 0 {
			payload["workflow_previous_goal_id"] = goalIDs[i-1]
		}
		raw, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		var id int64
		if err := tx.QueryRow(`INSERT INTO exploration_nodes(exploration_id,kind,payload,state,origin) VALUES($1,'goal',$2,'open','workflow') RETURNING id`, explorationID, string(raw)).Scan(&id); err != nil {
			return err
		}
		goalIDs = append(goalIDs, id)
	}
	raw, err := json.Marshal(input)
	if err != nil {
		return err
	}
	ids, err := json.Marshal(goalIDs)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT INTO assessment_workflow_runs(task_id,workflow_key,definition_json,goal_ids,request_id,request_hash) VALUES($1,$2,$3,$4,NULLIF($5,''),$6)`, taskID, input.Key, string(raw), string(ids), input.RequestID, input.RequestHash)
	return err
}

func (d *DB) GetAssessmentWorkflowRun(taskID int64) (*AssessmentWorkflowRun, error) {
	var row AssessmentWorkflowRun
	var definition, ids []byte
	err := d.QueryRow(`SELECT task_id,definition_json,goal_ids,created_at,COALESCE(request_id,''),request_hash FROM assessment_workflow_runs WHERE task_id=$1`, taskID).Scan(&row.TaskID, &definition, &ids, &row.CreatedAt, &row.RequestID, &row.RequestHash)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(definition, &row.Definition); err != nil {
		return nil, err
	}
	if err = json.Unmarshal(ids, &row.GoalIDs); err != nil {
		return nil, err
	}
	return &row, nil
}

func (d *DB) ListAssessmentWorkflowTaskIDs() ([]int64, error) {
	rows, err := d.Query(`SELECT run.task_id FROM assessment_workflow_runs run JOIN tasks ON tasks.id=run.task_id AND tasks.deleted_at IS NULL ORDER BY run.created_at DESC,run.task_id DESC LIMIT 100`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (d *DB) GetAssessmentWorkflowRunByRequestID(requestID string) (*AssessmentWorkflowRun, error) {
	var taskID int64
	err := d.QueryRow(`SELECT task_id FROM assessment_workflow_runs WHERE request_id=$1`, requestID).Scan(&taskID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return d.GetAssessmentWorkflowRun(taskID)
}

// AssessmentWorkflowComplete also checks missing or abandoned snapshot goals.
// Deleting a phase cannot turn an unfinished assessment into a completed one.
func (s *ExplorationStore) AssessmentWorkflowComplete() (bool, error) {
	var ready bool
	err := s.db.QueryRow(`SELECT NOT EXISTS(
 SELECT 1 FROM assessment_workflow_runs run JOIN tasks ON tasks.id=run.task_id
 WHERE tasks.exploration_id=$1 AND EXISTS(
  SELECT 1 FROM jsonb_array_elements_text(run.goal_ids) goal_id
  LEFT JOIN exploration_nodes node ON node.id=goal_id::bigint AND node.exploration_id=$1
  WHERE node.state IS DISTINCT FROM 'met'))`, s.expID).Scan(&ready)
	return ready, err
}

// SeedMergedSkillBindings grants each shipped skill once, without overwriting
// subsequent operator changes. Each installation/agent/skill has its own marker,
// so a partially installed directory does not reset earlier opt-outs on restart.
func (d *DB) SeedMergedSkillBindings(bindings map[string][]string) error {
	tx, err := d.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for key, names := range bindings {
		var agentID int64
		if err := tx.QueryRow(`SELECT id FROM agents WHERE key=$1`, key).Scan(&agentID); err != nil {
			return err
		}
		for _, name := range names {
			marker := "merged_skill_binding_v1." + key + "." + name
			result, err := tx.Exec(`INSERT INTO settings(key,value) VALUES($1,'true') ON CONFLICT(key) DO NOTHING`, marker)
			if err != nil {
				return err
			}
			n, err := result.RowsAffected()
			if err != nil {
				return err
			}
			if n == 0 {
				continue
			}
			if _, err := tx.Exec(`INSERT INTO agent_skill_visibility(agent_id,skill_name,enabled) VALUES($1,$2,true) ON CONFLICT(agent_id,skill_name) DO NOTHING`, agentID, name); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}
