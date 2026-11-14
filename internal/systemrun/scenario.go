package systemrun

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
	"software-studio/internal/delivery"
	"software-studio/internal/model"
	"software-studio/internal/state"
)

type StateCall struct {
	Command          string `json:"command,omitempty"`
	TaskID           string `json:"taskId"`
	CommandID        string `json:"commandId"`
	ExpectedRevision *int64 `json:"expectedRevision"`
	Fault            string `json:"fault,omitempty"`
}

func ScenarioEngineVersion(plan Plan, scenario Scenario) string {
	if len(plan.Workflows) > 0 {
		return "system-scenario-4"
	}
	if len(plan.Channels) > 0 {
		return "system-scenario-3"
	}
	if plan.EngineVersion == MultiCommandVersion {
		return "system-scenario-2"
	}
	for _, step := range scenario.Steps {
		if step.Kind == "command" {
			return "system-scenario-2"
		}
	}
	return "system-scenario-1"
}

type ScenarioStep struct {
	Workflow               *WorkflowCall     `json:"workflow,omitempty"`
	ExpectedWorkflowStatus string            `json:"expectedWorkflowStatus,omitempty"`
	Message                *MessageCall      `json:"message,omitempty"`
	ExpectedMessageStatus  string            `json:"expectedMessageStatus,omitempty"`
	Kind                   string            `json:"kind"`
	TaskID                 string            `json:"taskId,omitempty"`
	Inputs                 []Inputs          `json:"inputs,omitempty"`
	Calls                  []StateCall       `json:"calls,omitempty"`
	ExpectedStatuses       map[string]string `json:"expectedStatuses,omitempty"`
}
type FinalExpectation struct {
	TaskID   string                      `json:"taskId"`
	State    map[string]model.TypedValue `json:"state"`
	Revision *int64                      `json:"revision"`
	Receipts *int                        `json:"receipts"`
	Effects  *int                        `json:"effects"`
}
type Scenario struct {
	ExpectedWorkflows []WorkflowExpectation `json:"expectedWorkflows,omitempty"`
	ExpectedMessages  []MessageExpectation  `json:"expectedMessages,omitempty"`
	Name              string                `json:"name"`
	Steps             []ScenarioStep        `json:"steps"`
	Expected          []FinalExpectation    `json:"expected"`
}
type ScenarioAssertion struct {
	Path    string `json:"path"`
	Passed  bool   `json:"passed"`
	Message string `json:"message"`
}
type FinalState struct {
	TaskID   string               `json:"taskId"`
	Instance state.InstanceResult `json:"instance"`
}
type ScenarioOutcome struct {
	Workflows  []FinalWorkflow     `json:"workflows,omitempty"`
	Messages   []FinalMessages     `json:"messages,omitempty"`
	Verified   bool                `json:"verified"`
	Passed     bool                `json:"passed"`
	Assertions []ScenarioAssertion `json:"assertions"`
	Final      []FinalState        `json:"final"`
}
type scenarioRuntime struct {
	workflows                      map[string]*runningWorkflow
	workflowOrder                  []string
	brokers                        map[string]*delivery.SharedSession
	sessions                       map[string]*state.Session
	budget                         *state.Budget
	calls                          map[string]StateCall
	index, operations, invocations int
}

// RunScenario orders whole-graph invocations and explicit owner restarts. It
// explores one declared interleaving, not all concurrent schedules.
func RunScenario(ctx context.Context, original Plan, scenario Scenario) (Report, error) {
	fail := func(path, message string) (Report, error) {
		return Report{}, &Invalid{Issues: []model.ValidationIssue{{Path: path, Message: message}}}
	}
	if err := ctx.Err(); err != nil {
		return Report{}, err
	}
	if scenario.Name == "" || len(scenario.Name) > 200 || len(scenario.Steps) < 1 || len(scenario.Steps) > 64 || len(scenario.Expected) > 64 || len(scenario.ExpectedMessages) > 32 {
		return fail("scenario", "Name, 1–64 steps and at most 64 final expectations required")
	}
	encoded, err := json.Marshal(scenario)
	if err != nil {
		return Report{}, err
	}
	if len(encoded) > 1<<20 {
		return fail("scenario", "Scenario exceeds 1 MiB")
	}
	plan, err := PrepareUnified(original.Tasks, original.Links, original.Channels, original.Workflows)
	if err != nil {
		return Report{}, err
	}
	runtime := &scenarioRuntime{workflows: map[string]*runningWorkflow{}, sessions: map[string]*state.Session{}, budget: &state.Budget{}}
	tasks := map[string]Task{}
	for _, task := range plan.Tasks {
		tasks[task.ID] = task
		if task.State != nil {
			owner := fmt.Sprintf("owner%x", sha256.Sum256([]byte(task.ID)))[:37]
			session, err := state.NewSession(ctx, task.State.Machine, []model.StateInstance{{ID: owner, Name: task.Name, Initial: map[string]model.TypedValue{}}}, runtime.budget)
			if err != nil {
				return Report{}, err
			}
			runtime.sessions[task.ID] = session
		}
	}
	if len(runtime.sessions) == 0 && len(plan.Workflows) == 0 {
		return fail("scenario", "A shared scenario requires at least one state task")
	}
	if err := prepareMessages(ctx, plan, scenario, runtime); err != nil {
		return Report{}, err
	}
	if err := validateWorkflowSteps(plan, scenario); err != nil {
		return Report{}, err
	}
	// Validate all envelopes and independent expectations before any transition.
	activations := 0
	for i, step := range scenario.Steps {
		path := fmt.Sprintf("scenario.steps[%d]", i)
		switch step.Kind {
		case "workflow_start", "workflow_event", "workflow_restart", "workflow_deliver", "workflow_retry":
			activations++
		case "dispatch", "deliver", "retry", "broker_restart":
			activations++
		case "invoke":
			activations += len(plan.Tasks)
			if step.TaskID != "" || len(step.Calls) != len(runtime.sessions) {
				return fail(path, "Invoke requires one envelope per state task and no restart target")
			}
			seen := map[string]bool{}
			for _, call := range step.Calls {
				if call.Command != "" {
					return fail(path+".calls", "Graph invocations use the pinned entry command; use an owner command step to call another command")
				}
				task, ok := tasks[call.TaskID]
				if !ok || task.State == nil || seen[call.TaskID] {
					return fail(path+".calls", "Unknown or duplicate state task")
				}
				seen[call.TaskID] = true
				if message := validateCall(call); message != "" {
					return fail(path+".calls", message)
				}
			}
		case "command":
			activations++
			if runtime.sessions[step.TaskID] == nil || len(step.Calls) != 1 || step.Calls[0].TaskID != step.TaskID || len(step.Inputs) != 1 || step.Inputs[0].TaskID != step.TaskID {
				return fail(path, "Owner command requires one matching owner, envelope and input set")
			}
			call := step.Calls[0]
			if message := validateCall(call); message != "" {
				return fail(path+".calls", message)
			}
			command, ok := namedCommand(tasks[step.TaskID], call.Command)
			if !ok {
				return fail(path+".calls", "Choose an existing command on this state owner")
			}
			if issues := model.ValidateStructuredBehaviorValues(step.Inputs[0].Values, command.Inputs, mustContracts(tasks[step.TaskID])); len(issues) > 0 {
				return fail(path+".inputs", issues[0].Message)
			}
		case "restart":
			activations++
			if runtime.sessions[step.TaskID] == nil || len(step.Inputs) > 0 || len(step.Calls) > 0 {
				return fail(path, "Restart requires only an owned state task target")
			}
		default:
			return fail(path+".kind", "Choose a graph, owner, message or workflow step")
		}
		for id, status := range step.ExpectedStatuses {
			if _, ok := tasks[id]; !ok || (step.Kind != "invoke" && id != step.TaskID) {
				return fail(path+".expectedStatuses", "Expected task is absent from this step")
			}
			switch status {
			case "completed", "committed", "denied", "replayed", "id_conflict", "revision_conflict", "rolled_back", "evaluation_error", "reply_lost", "skipped", "restarted":
			default:
				return fail(path+".expectedStatuses", "Unknown expected status")
			}
		}
	}
	if activations > 256 {
		return fail("scenario.steps", "Scenario exceeds 256 task activations")
	}
	expectedIDs := map[string]bool{}
	for i, e := range scenario.Expected {
		path := fmt.Sprintf("scenario.expected[%d]", i)
		task, ok := tasks[e.TaskID]
		if !ok || task.State == nil || expectedIDs[e.TaskID] || e.Revision == nil || e.Receipts == nil || e.Effects == nil || *e.Revision < 0 || *e.Revision > model.MaxBehaviorInteger || *e.Receipts < 0 || *e.Receipts > 64 || *e.Effects < 0 || *e.Effects > 128 {
			return fail(path, "Unique state task and explicit bounded revision/receipt/effect counts required")
		}
		expectedIDs[e.TaskID] = true
		ps := []model.BehaviorPort{}
		for _, s := range task.State.Machine.Machine.State {
			ps = append(ps, model.BehaviorPort{Name: s.Name, Type: s.Type, RecordRef: s.RecordRef})
		}
		if issues := model.ValidateStructuredBehaviorValues(e.State, ps, mustContracts(task)); len(issues) > 0 {
			return fail(path+".state", issues[0].Message)
		}
	}
	report := Report{EngineVersion: "system-scenario-1", Mode: "shared-scenario", Events: []Event{}, Results: []TaskResult{}, Scenario: &ScenarioOutcome{Assertions: []ScenarioAssertion{}, Final: []FinalState{}}, Limitations: []string{"One explicitly ordered interleaving; no exhaustive race detection or parallel timing.", "State persists between steps within this run only. Each state aspect still exposes one command; invocations may repeat it.", "Lost replies and missing receipts block downstream transfers; independent branches may still execute. Effects are intents, not deliveries.", "Scenario expectations are checked independently; system contract claims, live providers and deployment are not evaluated."}}
	report.EngineVersion = ScenarioEngineVersion(plan, scenario)
	if report.EngineVersion == "system-scenario-2" || report.EngineVersion == "system-scenario-3" || report.EngineVersion == "system-scenario-4" {
		report.Limitations[1] = "State persists between steps within this run only. Graph invocations use each owner's selected entry; explicit owner commands share its revision and receipt history without invoking graph links."
	}
	if len(plan.Channels) > 0 {
		report.Limitations[2] = "Durable offline broker: explicit dispatch and retry, immutable recipient request, acknowledgement only on a recipient receipt. No network clock, automatic retries or live service."
	}
	assert := func(path string, passed bool, message string) {
		report.Scenario.Assertions = append(report.Scenario.Assertions, ScenarioAssertion{Path: path, Passed: passed, Message: message})
	}
	traceBytes := 0
	appendEvent := func(event Event, index int) error {
		event.Sequence = len(report.Events) + 1
		event.ScenarioStep = index + 1
		b, err := json.Marshal(event)
		if err != nil {
			return err
		}
		traceBytes += len(b)
		if traceBytes > MaxBytes/2 {
			return fmt.Errorf("scenario trace exceeds 2 MiB")
		}
		report.Events = append(report.Events, event)
		if len(plan.Channels) > 0 && event.State != nil && event.Kind == "completed" && event.State.Step.Status == "committed" && event.State.Step.Receipt != nil && len(event.State.Step.Receipt.EffectIDs) > 0 {
			intent := Event{Sequence: len(report.Events) + 1, ScenarioStep: index + 1, Kind: "intent_created", IntentIDs: event.State.Step.Receipt.EffectIDs, TaskID: event.TaskID, Values: map[string]model.TypedValue{}}
			b, _ := json.Marshal(intent)
			traceBytes += len(b)
			if traceBytes > MaxBytes/2 {
				return fmt.Errorf("scenario trace exceeds 2 MiB")
			}
			report.Events = append(report.Events, intent)
		}
		return nil
	}
	for i, step := range scenario.Steps {
		if err := ctx.Err(); err != nil {
			return Report{}, err
		}
		runtime.index = i
		statuses := map[string]string{}
		if step.Workflow != nil {
			events, result, err := runWorkflowStep(ctx, plan, step, runtime)
			if err != nil {
				return Report{}, err
			}
			for _, event := range events {
				if event.Workflow != nil && event.Workflow.Step != nil {
					activations++
					if activations > 256 {
						return fail("scenario.steps", "Scenario exceeds 256 task and workflow activations")
					}
				}
				if err = appendEvent(event, i); err != nil {
					return Report{}, err
				}
			}
			if step.ExpectedWorkflowStatus != "" {
				assert(fmt.Sprintf("steps[%d].workflow", i), result.Status == step.ExpectedWorkflowStatus, fmt.Sprintf("Expected %s; observed %s", step.ExpectedWorkflowStatus, result.Status))
			}
		} else if step.Message != nil {
			events, status, err := runMessageStep(ctx, plan, step, runtime)
			if err != nil {
				return Report{}, err
			}
			for _, event := range events {
				if err = appendEvent(event, i); err != nil {
					return Report{}, err
				}
			}
			if step.ExpectedMessageStatus != "" {
				assert(fmt.Sprintf("steps[%d].message", i), status == step.ExpectedMessageStatus, fmt.Sprintf("Expected %s; observed %s", step.ExpectedMessageStatus, status))
			}
		} else if step.Kind == "restart" {
			task := tasks[step.TaskID]
			owner := fmt.Sprintf("owner%x", sha256.Sum256([]byte(task.ID)))[:37]
			result, err := runtime.sessions[task.ID].Apply(ctx, i, model.StateStep{Kind: "restart", InstanceID: owner}, nil)
			if err != nil {
				return Report{}, err
			}
			snapshots, err := runtime.sessions[task.ID].Snapshot(ctx)
			if err != nil {
				return Report{}, err
			}
			evidence := &StateEvidence{Provider: task.State.Provider, Step: result, Instance: snapshots[0]}
			if err = appendEvent(Event{Kind: "restarted", TaskID: task.ID, Values: map[string]model.TypedValue{}, State: evidence}, i); err != nil {
				return Report{}, err
			}
			statuses[task.ID] = "restarted"
		} else if step.Kind == "command" {
			events, result, err := runOwnerCommand(ctx, tasks[step.TaskID], step, runtime)
			if err != nil {
				return Report{}, err
			}
			for _, event := range events {
				if err = appendEvent(event, i); err != nil {
					return Report{}, err
				}
			}
			result.ScenarioStep = i + 1
			report.Results = append(report.Results, result)
			statuses[result.TaskID] = result.Status
		} else {
			runtime.calls = map[string]StateCall{}
			for _, call := range step.Calls {
				runtime.calls[call.TaskID] = call
			}
			part, err := runGraph(ctx, plan, step.Inputs, runtime)
			if err != nil {
				return Report{}, fmt.Errorf("scenario step %d: %w", i+1, err)
			}
			for _, e := range part.Events {
				if err = appendEvent(e, i); err != nil {
					return Report{}, err
				}
			}
			for _, r := range part.Results {
				statuses[r.TaskID] = r.Status
				r.ScenarioStep = i + 1
				report.Results = append(report.Results, r)
			}
		}
		// Stable assertion order follows declared plan order, never map iteration.
		for _, id := range plan.Order {
			if expected, ok := step.ExpectedStatuses[id]; ok {
				actual := statuses[id]
				assert(fmt.Sprintf("steps[%d].%s", i, id), actual == expected, fmt.Sprintf("Expected %s; observed %s", expected, actual))
			}
		}
		b, err := json.Marshal(report)
		if err != nil {
			return Report{}, err
		}
		if len(b) > MaxBytes {
			return Report{}, fmt.Errorf("scenario report exceeds 4 MiB")
		}
	}
	for _, id := range plan.Order {
		session := runtime.sessions[id]
		if session == nil {
			continue
		}
		snapshots, err := session.Snapshot(ctx)
		if err != nil {
			return Report{}, err
		}
		actual := snapshots[0]
		report.Scenario.Final = append(report.Scenario.Final, FinalState{TaskID: id, Instance: actual})
		for _, e := range scenario.Expected {
			if e.TaskID != id {
				continue
			}
			passed := reflect.DeepEqual(actual.State, e.State) && actual.Revision == *e.Revision && len(actual.Receipts) == *e.Receipts && len(actual.Effects) == *e.Effects
			assert("final."+id, passed, "Compare complete owned state, revision, receipts and effect counts with the declared expectation")
		}
	}
	for _, channel := range plan.Channels {
		messages := runtime.brokers[channel.ID].Messages()
		report.Scenario.Messages = append(report.Scenario.Messages, FinalMessages{ChannelID: channel.ID, Messages: messages})
		for i, e := range scenario.ExpectedMessages {
			if e.ChannelID != channel.ID {
				continue
			}
			status, attempts := "absent", 0
			acceptedOK := e.Accepted == nil
			for _, m := range messages {
				if m.Name == e.Name {
					status, attempts = m.Status, m.Attempts
					acceptedOK = e.Accepted == nil || (m.Acknowledgement != nil && m.Acknowledgement.Accepted == *e.Accepted)
				}
			}
			assert(fmt.Sprintf("expectedMessages[%d]", i), status == e.Status && attempts == e.Attempts && acceptedOK, "Compare transport status, attempts and optional acknowledged business verdict")
		}
	}
	for _, id := range runtime.workflowOrder {
		running := runtime.workflows[id]
		result, err := running.session.Snapshot(ctx)
		if err != nil {
			return Report{}, err
		}
		report.Scenario.Workflows = append(report.Scenario.Workflows, FinalWorkflow{WorkflowID: running.task.ID, InstanceID: id, Result: result})
		for _, expected := range scenario.ExpectedWorkflows {
			if expected.InstanceID == id {
				assert("workflows."+id, matchWorkflowExpectation(expected, result), "Compare workflow status, outcome, waiting node and independently declared values")
			}
		}
	}
	report.Scenario.Verified = len(report.Scenario.Assertions) > 0
	report.Scenario.Passed = report.Scenario.Verified
	for _, a := range report.Scenario.Assertions {
		report.Scenario.Passed = report.Scenario.Passed && a.Passed
	}
	b, err := json.Marshal(report)
	if err != nil {
		return Report{}, err
	}
	if len(b) > MaxBytes {
		return Report{}, fmt.Errorf("scenario report exceeds 4 MiB")
	}
	var frozen Report
	err = json.Unmarshal(b, &frozen)
	return frozen, err
}
