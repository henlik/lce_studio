package systemrun

import (
	"context"
	"errors"
	"software-studio/internal/model"
	"software-studio/internal/state"
	"software-studio/internal/system"
	"testing"
)

func TestRepeatedWorkflowStartCancellation(t *testing.T) {
	base := messagePlan(t)
	example := system.M1Example()
	f := WorkflowTask{ID: Key("stock", "flow"), Name: "Flow", Lock: example.Workflow, Provider: example.Design.ExecutionProviders[2], Owners: []WorkflowOwner{{OwnerID: "stock", TaskID: Key("stock", "stock")}}, Routes: []WorkflowRoute{{NodeID: "handoff", ChannelID: Key("stock", "notices"), RouteID: "reservationToLedger", SourceNodeID: "reserve", Response: map[string]string{"accepted": "accepted"}}}}
	p, err := PrepareUnified(base.Tasks, base.Links, base.Channels, []WorkflowTask{f})
	if err != nil {
		t.Fatal(err)
	}
	cases, err := M1Scenarios(p)
	if err != nil {
		t.Fatal(err)
	}
	runtime := &scenarioRuntime{workflows: map[string]*runningWorkflow{}, sessions: map[string]*state.Session{}, budget: &state.Budget{}}
	for _, task := range p.Tasks {
		session, err := state.NewSession(context.Background(), task.State.Machine, []model.StateInstance{{ID: ownerID(task.ID), Name: task.Name, Initial: map[string]model.TypedValue{}}}, runtime.budget)
		if err != nil {
			t.Fatal(err)
		}
		runtime.sessions[task.ID] = session
	}
	step := cases[0].Scenario.Steps[0]
	if _, _, err = runWorkflowStep(context.Background(), p, step, runtime); err != nil {
		t.Fatal(err)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err = runWorkflowStep(cancelled, p, step, runtime); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled repeat must return without rendering an empty checkpoint", err)
	}
}
