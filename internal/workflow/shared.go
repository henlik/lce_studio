package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"software-studio/internal/model"
	"software-studio/internal/state"
)

// SharedSession drives the existing workflow interpreter against system-owned
// sessions. A modeled workflow checkpoint retains its position and identities;
// it never replays business commands when restarted.
type SharedOwner struct {
	Session    *state.Session
	InstanceID string
}
type SharedSession struct {
	engine *engine
	lock   model.WorkflowLock
	inputs map[string]model.TypedValue
	events int
}

func NewSharedSession(ctx context.Context, lock model.WorkflowLock, inputs map[string]model.TypedValue, owners map[string]SharedOwner, budget *state.Budget, namespace string, faults map[string]string) (*SharedSession, error) {
	scenario := model.WorkflowScenario{SchemaVersion: 1, Name: "Shared workflow", Inputs: inputs, Events: []model.WorkflowEvent{}}
	if issues := model.ValidateWorkflowScenario(lock, scenario); len(issues) > 0 {
		return nil, fmt.Errorf("invalid shared workflow at %s: %s", issues[0].Path, issues[0].Message)
	}
	prepared, _ := model.BuildWorkflowContext(lock.Workflow)
	if len(owners) != len(lock.Workflow.Owners) {
		return nil, fmt.Errorf("every workflow owner requires a shared mapping")
	}
	e := &engine{ctx: ctx, definition: lock.Workflow, prepared: prepared, inputs: copyValues(inputs), budget: budget, owners: map[string]*host{}, active: -1, receipts: map[string]int{}, instanceIDs: map[string]string{}, namespace: namespace, commandFaults: faults,
		result: Result{EngineVersion: "workflow-shared-1", WorkflowHash: lock.Digest, Status: "running", CurrentNode: lock.Workflow.Entry, Variables: map[string]model.TypedValue{}, Outputs: map[string]model.TypedValue{}, Tasks: []Task{}, Owners: []state.InstanceResult{}, Receipts: []EventReceipt{}, Events: []EventResult{}, Steps: []StepResult{}, Issues: []model.ValidationIssue{}}}
	for _, o := range lock.Workflow.Owners {
		if len(o.Initial) > 0 {
			return nil, fmt.Errorf("shared workflow owners cannot overwrite initial state")
		}
		shared, ok := owners[o.ID]
		if !ok || shared.Session == nil {
			return nil, fmt.Errorf("missing shared owner %s", o.ID)
		}
		if _, err := shared.Session.Revision(shared.InstanceID); err != nil {
			return nil, err
		}
		h := &host{session: shared.Session}
		e.hosts = append(e.hosts, h)
		e.owners[o.ID] = h
		e.instanceIDs[o.ID] = shared.InstanceID
	}
	for _, v := range lock.Workflow.Variables {
		e.result.Variables[v.Name] = v.Initial
	}
	e.result.Variables = copyValues(e.result.Variables)
	s := &SharedSession{engine: e, lock: lock, inputs: e.inputs}
	if err := s.refresh(ctx); err != nil {
		return nil, err
	}
	if err := e.drain(); err != nil {
		return nil, err
	}
	return s, nil
}
func (e *engine) stateID(alias string) string {
	if id := e.instanceIDs[alias]; id != "" {
		return id
	}
	return alias
}
func (s *SharedSession) refresh(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.engine.ctx = ctx
	for _, h := range s.engine.hosts {
		n, err := h.session.InstanceBytes()
		if err != nil {
			return err
		}
		h.bytes = n
	}
	return s.engine.admit(s.engine.project())
}
func (s *SharedSession) Apply(ctx context.Context, event model.WorkflowEvent) (Result, error) {
	if s.events >= 64 {
		return Result{}, fmt.Errorf("workflow event limit exceeded")
	}
	validation := model.WorkflowScenario{SchemaVersion: 1, Name: "Shared workflow event", Inputs: s.inputs, Events: []model.WorkflowEvent{event}}
	if issues := model.ValidateWorkflowScenario(s.lock, validation); len(issues) > 0 {
		return Result{}, fmt.Errorf("invalid workflow event: %s", issues[0].Message)
	}
	if err := s.refresh(ctx); err != nil {
		return Result{}, err
	}
	if err := s.engine.event(s.events, event); err != nil {
		return Result{}, err
	}
	s.events++
	return s.Snapshot(ctx)
}
func (s *SharedSession) Snapshot(ctx context.Context) (Result, error) {
	if err := s.refresh(ctx); err != nil {
		return Result{}, err
	}
	result := s.engine.result
	result.Owners = []state.InstanceResult{}
	for _, o := range s.lock.Workflow.Owners {
		rows, err := s.engine.owners[o.ID].session.Snapshot(ctx)
		if err != nil {
			return Result{}, err
		}
		for _, r := range rows {
			if r.ID == s.engine.stateID(o.ID) {
				result.Owners = append(result.Owners, r)
			}
		}
	}
	if s.engine.active >= 0 {
		task := copyTask(result.Tasks[s.engine.active])
		result.Task = &task
	}
	b, err := json.Marshal(result)
	if err != nil {
		return Result{}, err
	}
	if len(b) > MaxResultBytes {
		return Result{}, ErrLimit
	}
	var frozen Result
	err = json.Unmarshal(b, &frozen)
	return frozen, err
}
func (s *SharedSession) Restart(ctx context.Context) (Result, error) {
	if err := s.refresh(ctx); err != nil {
		return Result{}, err
	}
	// Roundtrip the durable journal/context. Interpreter indexes and shared owner
	// handles retain their identities; no call to drain or command is made.
	b, err := json.Marshal(s.engine.result)
	if err != nil {
		return Result{}, err
	}
	var restored Result
	if err = json.Unmarshal(b, &restored); err != nil {
		return Result{}, err
	}
	s.engine.result = restored
	return s.Snapshot(ctx)
}
