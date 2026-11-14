package workflow

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"software-studio/internal/behavior"
	"software-studio/internal/model"
	"software-studio/internal/state"
)

type host struct {
	session *state.Session
	bytes   int
}
type engine struct {
	instanceIDs                                                       map[string]string
	namespace                                                         string
	commandFaults                                                     map[string]string
	ctx                                                               context.Context
	definition                                                        model.WorkflowDefinition
	prepared                                                          model.WorkflowContext
	inputs                                                            map[string]model.TypedValue
	result                                                            Result
	budget                                                            *state.Budget
	hosts                                                             []*host
	owners                                                            map[string]*host
	active                                                            int
	commandCount                                                      int
	receipts                                                          map[string]int
	taskSizes                                                         []int
	taskBytes, stepBytes, receiptBytes, eventBytes, pendingEventBytes int
}

// Run deterministically replays an immutable, finite experiment. It has no
// external adapters, wall clock, durable clinical session, or implicit input.
func Run(ctx context.Context, lock model.WorkflowLock, scenario model.WorkflowScenario) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if issues := model.ValidateWorkflowScenario(lock, scenario); len(issues) > 0 {
		return Result{}, fmt.Errorf("invalid workflow scenario at %s: %s", issues[0].Path, issues[0].Message)
	}
	prepared, issues := model.BuildWorkflowContext(lock.Workflow)
	if len(issues) > 0 {
		return Result{}, fmt.Errorf("invalid workflow at %s: %s", issues[0].Path, issues[0].Message)
	}
	encoded, err := json.Marshal(scenario)
	if err != nil {
		return Result{}, err
	}
	e := &engine{ctx: ctx, definition: lock.Workflow, prepared: prepared, inputs: scenario.Inputs, budget: &state.Budget{}, owners: map[string]*host{}, active: -1, receipts: map[string]int{}, hosts: []*host{}, taskSizes: []int{},
		result: Result{EngineVersion: Version, WorkflowHash: lock.Digest, ScenarioHash: hash(encoded), Status: "running", CurrentNode: lock.Workflow.Entry,
			Variables: map[string]model.TypedValue{}, Outputs: map[string]model.TypedValue{}, Tasks: []Task{}, Owners: []state.InstanceResult{}, Receipts: []EventReceipt{}, Events: []EventResult{}, Steps: []StepResult{}, Issues: []model.ValidationIssue{}}}
	for _, variable := range lock.Workflow.Variables {
		e.result.Variables[variable.Name] = variable.Initial
	}
	if err := e.validateContext(e.result.Variables); err != nil {
		return Result{}, err
	}
	if err := e.createOwners(); err != nil {
		return Result{}, err
	}
	if err := e.admit(e.project()); err != nil {
		return Result{}, err
	}
	e.result.Variables = copyValues(e.result.Variables)
	if err := e.drain(); err != nil {
		return Result{}, err
	}
	for i, event := range scenario.Events {
		if err := e.event(i, event); err != nil {
			return Result{}, err
		}
	}
	if err := e.expect(scenario.Expected); err != nil {
		return Result{}, err
	}
	if err := e.admit(e.project()); err != nil {
		return Result{}, err
	}
	byID := map[string]state.InstanceResult{}
	for _, owner := range e.hosts {
		snapshot, err := owner.session.Snapshot(ctx)
		if err != nil {
			return Result{}, err
		}
		for _, instance := range snapshot {
			byID[instance.ID] = instance
		}
	}
	for _, owner := range e.definition.Owners {
		e.result.Owners = append(e.result.Owners, byID[owner.ID])
	}
	if e.active >= 0 {
		task := copyTask(e.result.Tasks[e.active])
		e.result.Task = &task
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	return e.result, nil
}

func (e *engine) createOwners() error {
	groups := map[model.DeliveryMachineRef][]model.StateInstance{}
	order := []model.DeliveryMachineRef{}
	for _, owner := range e.definition.Owners {
		initial := e.resolve(owner.Initial, e.inputs, e.result.Variables)
		if _, exists := groups[owner.Machine]; !exists {
			order = append(order, owner.Machine)
		}
		groups[owner.Machine] = append(groups[owner.Machine], model.StateInstance{ID: owner.ID, Name: owner.Name, Initial: initial})
	}
	for _, ref := range order {
		instances := groups[ref]
		session, err := state.NewSession(e.ctx, e.prepared.Owners[instances[0].ID], instances, e.budget)
		if err != nil {
			return err
		}
		n, err := session.InstanceBytes()
		if err != nil {
			return err
		}
		owner := &host{session: session, bytes: n}
		e.hosts = append(e.hosts, owner)
		for _, instance := range instances {
			e.owners[instance.ID] = owner
		}
	}
	return nil
}

func (e *engine) resolve(bindings map[string]model.WorkflowBinding, inputs, variables map[string]model.TypedValue) map[string]model.TypedValue {
	values := map[string]model.TypedValue{}
	for name, binding := range bindings {
		switch {
		case binding.Literal != nil:
			values[name] = *binding.Literal
		case binding.Input != "":
			values[name] = inputs[binding.Input]
		default:
			values[name] = variables[binding.Variable]
		}
	}
	return values
}
func (e *engine) validateContext(values map[string]model.TypedValue) error {
	if issues := model.ValidateWorkflowValues(values, e.prepared.Contracts, 48, 256<<10); len(issues) > 0 {
		return fmt.Errorf("%w: context at %s: %s", ErrLimit, issues[0].Path, issues[0].Message)
	}
	return nil
}
func assignments(mapping map[string]string, outputs map[string]model.TypedValue) map[string]model.TypedValue {
	values := map[string]model.TypedValue{}
	for target, source := range mapping {
		values[target] = outputs[source]
	}
	return values
}
func stagedValues(original, changes map[string]model.TypedValue) map[string]model.TypedValue {
	values := references(original)
	for name, value := range changes {
		values[name] = value
	}
	return values
}
func nextNode(node model.WorkflowNode, values map[string]model.TypedValue) string {
	if node.Branch == nil {
		return node.Next
	}
	if *values[node.Branch.Variable].Boolean {
		return node.Branch.Then
	}
	return node.Branch.Else
}
func (e *engine) taskID() string {
	if e.active < 0 {
		return ""
	}
	return e.result.Tasks[e.active].ID
}

func (e *engine) drain() error {
	for e.result.Status == "running" {
		if err := e.ctx.Err(); err != nil {
			return err
		}
		if len(e.result.Steps) >= MaxTransitions {
			return fmt.Errorf("%w: at most %d automatic transitions", ErrLimit, MaxTransitions)
		}
		node := e.prepared.Nodes[e.result.CurrentNode]
		step := StepResult{Index: len(e.result.Steps), NodeID: node.ID, Kind: node.Kind, Assignments: map[string]model.TypedValue{}}
		var err error
		switch node.Kind {
		case "task":
			err = e.openTask(node, step)
		case "decision":
			err = e.decision(node, step)
		case "command":
			err = e.command(node, step)
		case "end":
			err = e.end(node, step)
		default:
			err = fmt.Errorf("unknown validated workflow node %q", node.ID)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (e *engine) openTask(node model.WorkflowNode, step StepResult) error {
	if len(e.result.Tasks) >= MaxTasks {
		return fmt.Errorf("%w: at most %d task activations", ErrLimit, MaxTasks)
	}
	request := e.resolve(node.Task.Request, e.inputs, e.result.Variables)
	if issues := model.ValidateWorkflowValues(request, e.prepared.Contracts, 8, 64<<10); len(issues) > 0 {
		return fmt.Errorf("%w: task request exceeds bounds", ErrLimit)
	}
	task := Task{ID: fmt.Sprintf("task%d", len(e.result.Tasks)+1), NodeID: node.ID, Kind: node.Task.Kind, Title: node.Task.Title, Instructions: node.Task.Instructions, Request: request, Response: node.Task.Response, Status: "pending", Values: map[string]model.TypedValue{}}
	n := size(task)
	step.Status, step.TaskID = "waiting", task.ID
	p := e.project()
	p.status = "waiting"
	p.tasks++
	p.taskBytes += n
	p.activeTaskBytes = n
	if err := e.appendStep(step, p); err != nil {
		return err
	}
	e.result.Tasks = append(e.result.Tasks, copyTask(task))
	e.taskSizes = append(e.taskSizes, n)
	e.taskBytes += n
	e.active = len(e.result.Tasks) - 1
	e.result.Status = "waiting"
	return nil
}

func (e *engine) decision(node model.WorkflowNode, step StepResult) error {
	bundle := e.prepared.Decisions[node.Decision.Component]
	inputs := e.resolve(node.Decision.Inputs, e.inputs, e.result.Variables)
	work := expandedWork(bundle)
	if err := e.budget.ReserveWork(work.operations, work.invocations); err != nil {
		return fmt.Errorf("%w: %v", ErrLimit, err)
	}
	result, err := behavior.Evaluate(e.ctx, bundle, inputs)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		if errors.Is(err, behavior.ErrLimit) {
			return fmt.Errorf("%w: %v", ErrLimit, err)
		}
		step.Status, step.Message = "failed", err.Error()
		p := e.project()
		p.status = "failed"
		if err := e.appendStep(step, p); err != nil {
			return err
		}
		e.result.Status = "failed"
		return nil
	}
	step.Decision = &result
	step.Assignments = assignments(node.Decision.Assign, result.Outputs)
	values := stagedValues(e.result.Variables, step.Assignments)
	if err := e.validateContext(values); err != nil {
		return err
	}
	step.Status, step.Next = "evaluated", nextNode(node, values)
	p := e.project()
	p.variables = size(values)
	p.node = step.Next
	p.operations += result.Operations
	p.invocations += result.Invocations
	if err := e.appendStep(step, p); err != nil {
		return err
	}
	e.result.Variables = copyValues(values)
	e.result.CurrentNode = step.Next
	e.result.Operations = p.operations
	e.result.Invocations = p.invocations
	return nil
}

func (e *engine) command(node model.WorkflowNode, step StepResult) error {
	if e.commandCount >= 64 {
		return fmt.Errorf("%w: at most 64 owned command calls", ErrLimit)
	}
	owner := e.owners[node.Command.Owner]
	revision, err := owner.session.Revision(e.stateID(node.Command.Owner))
	if err != nil {
		return err
	}
	inputs := e.resolve(node.Command.Inputs, e.inputs, e.result.Variables)
	command := model.StateStep{Kind: "command", InstanceID: e.stateID(node.Command.Owner), CommandID: fmt.Sprintf("%swf_command%d", e.namespace, e.commandCount+1), Fault: e.commandFaults[node.ID], Command: node.Command.Command, ExpectedRevision: &revision, Inputs: inputs}
	var admitted projection
	var stepBytes int
	result, err := owner.session.Apply(e.ctx, e.commandCount, command, func(proposed state.Projection) error {
		p := e.project()
		p.owner, p.ownerBytes = owner, proposed.InstanceBytes
		p.operations += proposed.Operations
		p.invocations += proposed.Invocations
		// Scalar metadata permits exact map sizing and routing without exposing
		// or copying proposed owned values before admission.
		accepted := model.TypedValue{Type: "boolean", Boolean: &proposed.ReceiptAccepted}
		changes := map[string]int{node.Command.AcceptedVariable: size(accepted)}
		bools := map[string]bool{node.Command.AcceptedVariable: proposed.ReceiptAccepted}
		if proposed.ReceiptPresent {
			for target, source := range node.Command.Assign {
				changes[target] = proposed.OutputBytes[source]
				if value, ok := proposed.BooleanOutputs[source]; ok {
					bools[target] = value
				}
			}
		}
		assignmentBytes := 2 + commas(len(changes))
		for target, n := range changes {
			p.variables += n - size(e.result.Variables[target])
			assignmentBytes += mapEntrySize(target, n)
		}
		if p.variables > 256<<10 {
			return fmt.Errorf("%w: context exceeds 262144 bytes", ErrLimit)
		}
		step.Status, step.Message = proposed.Status, proposed.Message
		if !proposed.ReceiptPresent {
			step.Next = node.Command.ErrorNext
		} else if node.Branch != nil {
			condition, changed := bools[node.Branch.Variable]
			if !changed {
				condition = *e.result.Variables[node.Branch.Variable].Boolean
			}
			if condition {
				step.Next = node.Branch.Then
			} else {
				step.Next = node.Branch.Else
			}
		} else {
			step.Next = node.Next
		}
		p.node = step.Next
		stepBytes = size(step) + assignmentBytes - 2 + len(",\"state\":") + proposed.StepBytes
		p.steps++
		p.stepBytes += stepBytes
		if err := e.admit(p); err != nil {
			return err
		}
		admitted = p
		return nil
	})
	if err != nil {
		if errors.Is(err, state.ErrLimit) {
			return fmt.Errorf("%w: %v", ErrLimit, err)
		}
		return err
	}
	e.commandCount++
	step.State = &result
	step.Assignments = map[string]model.TypedValue{}
	accepted := false
	if result.Receipt != nil {
		accepted = result.Receipt.Accepted
		step.Assignments = assignments(node.Command.Assign, result.Receipt.Outputs)
	}
	step.Assignments[node.Command.AcceptedVariable] = model.TypedValue{Type: "boolean", Boolean: &accepted}
	values := stagedValues(e.result.Variables, step.Assignments)
	if err := e.validateContext(values); err != nil {
		return err
	}
	// The exact encoding must agree with the scalar projection before retain.
	if size(step) != stepBytes {
		return fmt.Errorf("workflow command accounting mismatch")
	}
	step.Assignments = copyValues(step.Assignments)
	e.result.Steps = append(e.result.Steps, step)
	e.stepBytes += stepBytes
	e.result.Variables = copyValues(values)
	e.result.CurrentNode = step.Next
	e.result.Operations = admitted.operations
	e.result.Invocations = admitted.invocations
	owner.bytes = admitted.ownerBytes
	return nil
}

func (e *engine) end(node model.WorkflowNode, step StepResult) error {
	outputs := e.resolve(node.End.Outputs, e.inputs, e.result.Variables)
	if issues := model.ValidateStructuredBehaviorValues(outputs, e.definition.Outputs, e.prepared.Contracts); len(issues) > 0 {
		return fmt.Errorf("%w: final outputs exceed their contract", ErrLimit)
	}
	step.Status = "completed"
	p := e.project()
	p.status = "completed"
	p.outcome = node.End.Outcome
	p.outputs = size(outputs)
	if err := e.appendStep(step, p); err != nil {
		return err
	}
	e.result.Outputs = copyValues(outputs)
	e.result.Status = "completed"
	e.result.Outcome = node.End.Outcome
	return nil
}

type work struct{ operations, invocations int }

func expandedWork(bundle model.ComponentBundle) work {
	packages := map[model.ComponentRef]model.ComponentPackage{}
	for _, lock := range bundle.Packages {
		packages[model.ComponentRef{PackageID: lock.Package.ID, Version: lock.Package.Version, Digest: lock.Digest}] = lock.Package
	}
	cache := map[model.ComponentRef]work{}
	var visit func(model.ComponentRef) work
	visit = func(ref model.ComponentRef) work {
		if known, ok := cache[ref]; ok {
			return known
		}
		result := work{invocations: 1}
		for _, node := range packages[ref].Behavior.Nodes {
			if node.Component == nil {
				result.operations++
			} else {
				child := visit(*node.Component)
				result.operations += child.operations
				result.invocations += child.invocations
			}
		}
		cache[ref] = result
		return result
	}
	return visit(bundle.Root)
}
func hash(bytes []byte) string { sum := sha256.Sum256(bytes); return hex.EncodeToString(sum[:]) }

func (e *engine) expect(expected *model.WorkflowExpectation) error {
	if expected == nil {
		return nil
	}
	e.result.Verified = true
	add := func(path, message string) {
		e.result.Issues = append(e.result.Issues, model.ValidationIssue{Path: path, Message: message})
	}
	if expected.Status != e.result.Status {
		add("expected.status", fmt.Sprintf("Expected %q, got %q.", expected.Status, e.result.Status))
	}
	if expected.Outcome != e.result.Outcome {
		add("expected.outcome", fmt.Sprintf("Expected %q, got %q.", expected.Outcome, e.result.Outcome))
	}
	active := ""
	if e.active >= 0 {
		active = e.result.Tasks[e.active].NodeID
	}
	if expected.ActiveTaskNode != active {
		add("expected.activeTaskNode", fmt.Sprintf("Expected %q, got %q.", expected.ActiveTaskNode, active))
	}
	for _, name := range keys(expected.Variables) {
		if !reflect.DeepEqual(expected.Variables[name], e.result.Variables[name]) {
			add("expected.variables."+name, "Final variable differs from the independent expectation.")
		}
	}
	for i, owner := range expected.Owners {
		actual, err := e.owners[owner.InstanceID].session.Expectation(owner)
		if err != nil {
			return err
		}
		for _, issue := range actual.Issues {
			add(fmt.Sprintf("expected.owners[%d].%s", i, issue.Path), issue.Message)
		}
	}
	completed, cancelled := 0, 0
	for _, task := range e.result.Tasks {
		if task.Status == "completed" {
			completed++
		}
		if task.Status == "cancelled" {
			cancelled++
		}
	}
	for _, count := range []struct {
		name             string
		expected, actual int
	}{{"tasks", expected.Tasks, len(e.result.Tasks)}, {"completedTasks", expected.CompletedTasks, completed}, {"cancelledTasks", expected.CancelledTasks, cancelled}, {"eventReceipts", expected.EventReceipts, len(e.result.Receipts)}} {
		if count.expected != count.actual {
			add("expected."+count.name, fmt.Sprintf("Expected %d, got %d.", count.expected, count.actual))
		}
	}
	e.result.Passed = len(e.result.Issues) == 0
	return nil
}
