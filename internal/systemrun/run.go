// Package systemrun executes bounded acyclic pure and single-command state graphs.
// It performs no storage effects, timing simulation or contract verification.
package systemrun

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"software-studio/internal/behavior"
	"software-studio/internal/model"
	"software-studio/internal/state"
	"software-studio/internal/system"
	"sort"
)

const Version = "system-dataflow-2"
const MaxBytes = 4 << 20

type StateTask struct {
	Machine  model.StateMachineLock   `json:"machine"`
	Provider system.ExecutionProvider `json:"provider"`
}
type StateEvidence struct {
	Provider system.ExecutionProvider `json:"provider"`
	Step     state.StepResult         `json:"step"`
	Instance state.InstanceResult     `json:"instance"`
}
type Task struct {
	State       *StateTask            `json:"state,omitempty"`
	ID          string                `json:"id"`
	ComponentID string                `json:"componentId"`
	Slot        string                `json:"slot"`
	Name        string                `json:"name"`
	Bundle      model.ComponentBundle `json:"bundle"`
}
type Link struct {
	ID           string `json:"id"`
	Source       string `json:"source"`
	Target       string `json:"target"`
	Output       string `json:"output"`
	Input        string `json:"input"`
	ConnectionID string `json:"connectionId,omitempty"`
}
type RequiredInput struct {
	TaskID string `json:"taskId"`
	model.BehaviorPort
	Fields []model.DataField `json:"fields,omitempty"`
}
type Plan struct {
	EngineVersion string          `json:"engineVersion"`
	Tasks         []Task          `json:"tasks"`
	Links         []Link          `json:"links"`
	Order         []string        `json:"order"`
	Inputs        []RequiredInput `json:"inputs"`
}
type Inputs struct {
	TaskID string                      `json:"taskId"`
	Values map[string]model.TypedValue `json:"values"`
}
type Event struct {
	ScenarioStep int                         `json:"scenarioStep,omitempty"`
	State        *StateEvidence              `json:"state,omitempty"`
	Sequence     int                         `json:"sequence"`
	Kind         string                      `json:"kind"`
	TaskID       string                      `json:"taskId"`
	Target       string                      `json:"target,omitempty"`
	LinkID       string                      `json:"linkId,omitempty"`
	ConnectionID string                      `json:"connectionId,omitempty"`
	Values       map[string]model.TypedValue `json:"values"`
}
type TaskResult struct {
	Status       string                      `json:"status,omitempty"`
	ScenarioStep int                         `json:"scenarioStep,omitempty"`
	State        *StateEvidence              `json:"state,omitempty"`
	TaskID       string                      `json:"taskId"`
	Outputs      map[string]model.TypedValue `json:"outputs"`
	BundleHash   string                      `json:"bundleHash"`
	Operations   int                         `json:"operations"`
	Invocations  int                         `json:"invocations"`
}
type Report struct {
	Scenario        *ScenarioOutcome `json:"scenario,omitempty"`
	EngineVersion   string           `json:"engineVersion"`
	Mode            string           `json:"mode"`
	ClaimsEvaluated bool             `json:"claimsEvaluated"`
	Events          []Event          `json:"events"`
	Results         []TaskResult     `json:"results"`
	Limitations     []string         `json:"limitations"`
}
type Invalid struct{ Issues []model.ValidationIssue }

func (e *Invalid) Error() string {
	return "System execution requires a valid acyclic dataflow plan and complete typed inputs"
}
func Key(component, slot string) string {
	b, _ := json.Marshal([]string{component, slot})
	return string(b)
}
func root(t Task) model.ComponentPackage {
	for _, p := range t.Bundle.Packages {
		if p.Package.ID == t.Bundle.Root.PackageID && p.Package.Version == t.Bundle.Root.Version && p.Digest == t.Bundle.Root.Digest {
			return p.Package
		}
	}
	return model.ComponentPackage{}
}

func Prepare(tasks []Task, links []Link) (Plan, error) {
	plan := Plan{EngineVersion: "system-pure-1", Tasks: tasks, Links: links, Order: []string{}, Inputs: []RequiredInput{}}
	issues := []model.ValidationIssue{}
	add := func(path, message string) {
		issues = append(issues, model.ValidationIssue{Path: path, Message: message})
	}
	if len(tasks) == 0 || len(tasks) > 64 || len(links) > 256 {
		return plan, &Invalid{[]model.ValidationIssue{{Path: "components", Message: "Execution supports 1–64 tasks and at most 256 links"}}}
	}
	bundleBytes := 0
	byID := map[string]Task{}
	indegree := map[string]int{}
	bound := map[string]map[string]bool{}
	for i, t := range tasks {
		encoded, err := json.Marshal(t)
		if err != nil {
			return plan, err
		}
		bundleBytes += len(encoded)
		if bundleBytes > MaxBytes {
			return plan, &Invalid{[]model.ValidationIssue{{Path: "tasks", Message: "Task bundles exceed 4 MiB"}}}
		}
		path := fmt.Sprintf("tasks[%d]", i)
		if t.ID == "" {
			add(path, "Task identity required")
		}
		if _, ok := byID[t.ID]; ok {
			add(path, "Duplicate task identity")
		}
		byID[t.ID] = t
		indegree[t.ID] = 0
		bound[t.ID] = map[string]bool{}
		validation := model.ValidateComponentBundle(t.Bundle)
		if t.State != nil {
			plan.EngineVersion = Version
			validation = model.ValidateStateMachineLock(t.State.Machine)
			if t.Bundle.SchemaVersion != 0 || len(t.Bundle.Packages) > 0 || t.Bundle.Root != (model.ComponentRef{}) {
				add(path, "A task cannot contain both pure and state definitions")
			}
			if len(t.State.Machine.Machine.Commands) != 1 {
				add(path, "State tasks require exactly one command; multi-command scheduling is not implemented")
			}
			if t.State.Provider.ID == "" || t.State.Provider.Kind != "modeled-state" || t.State.Provider.Version != "1.0.0" {
				add(path, "State requires an exact modeled-state@1.0.0 provider")
			}
		}
		for _, issue := range validation {
			add(path+".bundle."+issue.Path, issue.Message)
		}
	}
	if len(issues) > 0 {
		return plan, &Invalid{issues}
	}
	ids := map[string]bool{}
	for i, l := range links {
		path := fmt.Sprintf("links[%d]", i)
		if l.ID == "" || ids[l.ID] {
			add(path, "Link identities must be unique")
		}
		ids[l.ID] = true
		s, sok := byID[l.Source]
		t, tok := byID[l.Target]
		if !sok || !tok {
			add(path, "Unknown task endpoint")
			continue
		}
		var out, in *model.BehaviorPort
		for _, p := range ports(s, false) {
			if p.Name == l.Output {
				v := p
				out = &v
			}
		}
		for _, p := range ports(t, true) {
			if p.Name == l.Input {
				v := p
				in = &v
			}
		}
		if out == nil || in == nil {
			add(path, "Missing input or output port")
		} else if !model.SameBehaviorType(model.PortBehaviorType(*out), model.PortBehaviorType(*in)) {
			add(path, "Exact input and output types differ")
		}
		if bound[l.Target][l.Input] {
			add(path, "Multiple producers for one input")
		}
		bound[l.Target][l.Input] = true
		indegree[l.Target]++
	}
	if len(issues) > 0 {
		return plan, &Invalid{issues}
	}
	for len(plan.Order) < len(tasks) {
		ready := []string{}
		for id, n := range indegree {
			if n == 0 {
				ready = append(ready, id)
			}
		}
		sort.Strings(ready)
		if len(ready) == 0 {
			add("connections", "Cycle detected; feedback loops need explicit execution semantics")
			break
		}
		id := ready[0]
		plan.Order = append(plan.Order, id)
		indegree[id] = -1
		for _, l := range links {
			if l.Source == id {
				indegree[l.Target]--
			}
		}
	}
	for _, id := range plan.Order {
		t := byID[id]
		contracts, _ := contracts(t)
		for _, p := range ports(t, true) {
			if !bound[id][p.Name] {
				r := RequiredInput{TaskID: id, BehaviorPort: p}
				if p.RecordRef != nil {
					record, err := model.ResolveDataRecord(*p.RecordRef, contracts)
					if err != nil {
						add("inputs", err.Error())
					} else {
						r.Fields = record.Fields
					}
				}
				plan.Inputs = append(plan.Inputs, r)
			}
		}
	}
	if len(issues) > 0 {
		return plan, &Invalid{issues}
	}
	return plan, nil
}

func Run(ctx context.Context, plan Plan, inputs []Inputs) (Report, error) {
	return runGraph(ctx, plan, inputs, nil)
}
func runGraph(ctx context.Context, plan Plan, inputs []Inputs, runtime *scenarioRuntime) (Report, error) {
	if err := ctx.Err(); err != nil {
		return Report{}, err
	}
	// Re-derive order and input requirements; never trust a caller-supplied schedule.
	plan, err := Prepare(plan.Tasks, plan.Links)
	if err != nil {
		return Report{}, err
	}
	values := map[string]map[string]model.TypedValue{}
	seen := map[string]bool{}
	issues := []model.ValidationIssue{}
	byID := map[string]Task{}
	for _, t := range plan.Tasks {
		byID[t.ID] = t
		values[t.ID] = map[string]model.TypedValue{}
	}
	if len(inputs) > 64 {
		return Report{}, &Invalid{[]model.ValidationIssue{{Path: "inputs", Message: "At most 64 task input sets"}}}
	}
	for i, input := range inputs {
		if _, ok := byID[input.TaskID]; !ok || seen[input.TaskID] {
			issues = append(issues, model.ValidationIssue{Path: fmt.Sprintf("inputs[%d]", i), Message: "Unknown or duplicate task input set"})
			continue
		}
		seen[input.TaskID] = true
		for k, v := range input.Values {
			values[input.TaskID][k] = v
		}
	}
	for _, id := range plan.Order {
		t := byID[id]
		ports := []model.BehaviorPort{}
		for _, p := range plan.Inputs {
			if p.TaskID == id {
				ports = append(ports, p.BehaviorPort)
			}
		}
		contracts, _ := contracts(t)
		for _, issue := range model.ValidateStructuredBehaviorValues(values[id], ports, contracts) {
			issues = append(issues, model.ValidationIssue{Path: "inputs." + id + "." + issue.Path, Message: issue.Message})
		}
	}
	if len(issues) > 0 {
		return Report{}, &Invalid{issues}
	}
	report := Report{EngineVersion: plan.EngineVersion, Mode: "pure-dataflow", Events: []Event{}, Results: []TaskResult{}, Limitations: []string{"Each pure task executes once in deterministic topological order; independent tasks are serialized.", "No virtual clock, resource simulation, external I/O, stateful effects, delivery semantics or deployment execution.", "System claims and assumptions are not evaluated. This is model execution, not implementation conformance."}}
	for _, t := range plan.Tasks {
		if t.State != nil {
			report.Mode = "stateful-dataflow"
		}
	}
	if report.Mode == "stateful-dataflow" {
		report.Limitations = []string{"One invocation per task; each state aspect owns a fresh instance initialized from its immutable definition.", "modeled-state@1.0.0 supplies in-memory atomic state, revisions, receipts and effect intents within this run; it is not live PostgreSQL storage.", "Independent tasks are serialized. Effects are not delivered; no clock, restart scenario, deployment or claim evaluation is performed."}
	}
	stateBudget := &state.Budget{}
	if runtime != nil {
		stateBudget = runtime.budget
	}
	eventBytes := 0
	operations, invocations := 0, 0
	emit := func(e Event) error {
		e.Sequence = len(report.Events) + 1
		b, err := json.Marshal(e)
		if err != nil {
			return err
		}
		eventBytes += len(b)
		if eventBytes > MaxBytes/2 {
			return fmt.Errorf("execution trace exceeds 2 MiB")
		}
		report.Events = append(report.Events, e)
		return nil
	}
	for _, id := range plan.Order {
		if err := ctx.Err(); err != nil {
			return Report{}, err
		}
		if runtime != nil && len(model.ValidateStructuredBehaviorValues(values[id], ports(byID[id], true), mustContracts(byID[id]))) > 0 {
			report.Results = append(report.Results, TaskResult{TaskID: id, Status: "skipped", Outputs: map[string]model.TypedValue{}})
			if err := emit(Event{Kind: "skipped", TaskID: id, Values: map[string]model.TypedValue{}}); err != nil {
				return Report{}, err
			}
			continue
		}
		if err := emit(Event{Kind: "started", TaskID: id, Values: values[id]}); err != nil {
			return Report{}, err
		}
		task := byID[id]
		result := TaskResult{TaskID: id}
		if task.State == nil {
			evaluated, err := behavior.Evaluate(ctx, task.Bundle, values[id])
			if err != nil {
				return Report{}, fmt.Errorf("task %s: %w", id, err)
			}
			result.Outputs, result.BundleHash, result.Operations, result.Invocations = evaluated.Outputs, evaluated.BundleHash, evaluated.Operations, evaluated.Invocations
			if runtime != nil {
				if err := runtime.budget.AccountPure(evaluated.Operations, evaluated.Invocations); err != nil {
					return Report{}, err
				}
			}
		} else {
			ownerID := fmt.Sprintf("owner%x", sha256.Sum256([]byte(id)))[:37]
			var session *state.Session
			if runtime != nil {
				session = runtime.sessions[id]
			} else {
				session, err = state.NewSession(ctx, task.State.Machine, []model.StateInstance{{ID: ownerID, Name: task.Name, Initial: map[string]model.TypedValue{}}}, stateBudget)
				if err != nil {
					return Report{}, err
				}
			}
			zero := int64(0)
			call := StateCall{TaskID: id, CommandID: "execute", ExpectedRevision: &zero}
			index := 0
			if runtime != nil {
				call = runtime.calls[id]
				index = runtime.index
			}
			step, err := session.Apply(ctx, index, model.StateStep{Kind: "command", InstanceID: ownerID, CommandID: call.CommandID, Command: task.State.Machine.Machine.Commands[0].Name, ExpectedRevision: call.ExpectedRevision, Inputs: values[id], Fault: call.Fault}, nil)
			if err != nil {
				return Report{}, err
			}
			if step.Receipt == nil && runtime == nil {
				return Report{}, fmt.Errorf("state task %s produced no receipt: %s %s", id, step.Status, step.Message)
			}

			snapshot, err := session.Snapshot(ctx)
			if err != nil {
				return Report{}, err
			}
			result.State = &StateEvidence{Provider: task.State.Provider, Step: step, Instance: snapshot[0]}
			result.Outputs, result.BundleHash = map[string]model.TypedValue{}, task.State.Machine.Digest
			if step.Receipt != nil && !(runtime != nil && step.ReplyLost) {
				result.Outputs = step.Receipt.Outputs
			}
			if runtime != nil {
				result.Status = step.Status
				if step.ReplyLost {
					result.Status = "reply_lost"
				}
			}
			if step.Decision != nil {
				result.Operations, result.Invocations = step.Decision.Operations, step.Decision.Invocations
			}
		}
		if runtime != nil {
			runtime.operations += result.Operations
			runtime.invocations += result.Invocations
			if runtime.operations > 4096 || runtime.invocations > 4096 {
				return Report{}, fmt.Errorf("scenario work budget exceeded")
			}
			if result.Status == "" {
				result.Status = "completed"
			}
		}
		operations += result.Operations
		invocations += result.Invocations
		if operations > 4096 || invocations > 4096 {
			return Report{}, fmt.Errorf("system operation or invocation budget exceeded")
		}
		report.Results = append(report.Results, result)
		if err := emit(Event{Kind: "completed", TaskID: id, Values: result.Outputs, State: result.State}); err != nil {
			return Report{}, err
		}

		for _, l := range plan.Links {
			if l.Source != id {
				continue
			}
			v, ok := result.Outputs[l.Output]
			if !ok {
				if runtime != nil {
					continue
				}
				return Report{}, fmt.Errorf("missing output for link %s", l.ID)
			}
			values[l.Target][l.Input] = v
			if err := emit(Event{Kind: "transfer", TaskID: id, Target: l.Target, LinkID: l.ID, ConnectionID: l.ConnectionID, Values: map[string]model.TypedValue{l.Input: v}}); err != nil {
				return Report{}, err
			}
		}
	}
	// Freeze all maps shared with inputs/outputs and cap the complete report.
	b, err := json.Marshal(report)
	if err != nil {
		return Report{}, err
	}
	if len(b) > MaxBytes {
		return Report{}, fmt.Errorf("execution report exceeds 4 MiB")
	}
	var frozen Report
	err = json.Unmarshal(b, &frozen)
	return frozen, err
}

func ports(t Task, input bool) []model.BehaviorPort {
	if t.State != nil {
		if len(t.State.Machine.Machine.Commands) != 1 {
			return nil
		}
		c := t.State.Machine.Machine.Commands[0]
		if input {
			return c.Inputs
		}
		return c.Outputs
	}
	b := root(t).Behavior
	if b == nil {
		return nil
	}
	if input {
		return b.Inputs
	}
	return b.Outputs
}
func contracts(t Task) ([]model.DataContractLock, []model.ValidationIssue) {
	if t.State != nil {
		return model.StateMachineDataContracts(t.State.Machine.Machine)
	}
	return model.BundleDataContracts(t.Bundle)
}

func mustContracts(t Task) []model.DataContractLock { c, _ := contracts(t); return c }
