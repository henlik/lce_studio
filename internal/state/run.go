package state

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
)

type ownedInstance struct {
	id           string
	initial      map[string]model.TypedValue
	values       map[string]model.TypedValue
	revision     int64
	receipts     []Receipt
	receiptIndex map[string]int
	effects      []EffectIntent
	receiptBytes int
	effectBytes  int
}

type work struct{ operations, invocations int }

type engine struct {
	ctx          context.Context
	machine      model.StateMachine
	contracts    []model.DataContractLock
	commands     map[string]model.StateCommand
	work         map[string]work
	instances    map[string]*ownedInstance
	order        []string
	result       Result
	reserved     work
	receiptCount int
	effectCount  int
	stepBytes    int
	shared       *Budget
	session      bool
	admit        func(Projection) error
	last         StepResult
	beforeWork   work
}

func emptyValues() map[string]model.TypedValue { return map[string]model.TypedValue{} }

// Run starts a fresh experiment. The only durable state is owned by this run;
// modeled restarts preserve it, and a subsequent Run initializes it anew.
func Run(ctx context.Context, lock model.StateMachineLock, scenario model.StateScenario) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if issues := model.ValidateStateMachineLock(lock); len(issues) != 0 {
		return Result{}, fmt.Errorf("invalid machine at %s: %s", issues[0].Path, issues[0].Message)
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if issues := model.ValidateStateScenario(lock, scenario); len(issues) != 0 {
		return Result{}, fmt.Errorf("invalid scenario at %s: %s", issues[0].Path, issues[0].Message)
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	encoded, err := json.Marshal(scenario)
	if err != nil {
		return Result{}, fmt.Errorf("encode scenario: %w", err)
	}
	contracts, issues := model.StateMachineDataContracts(lock.Machine)
	if len(issues) != 0 {
		return Result{}, fmt.Errorf("invalid data union at %s: %s", issues[0].Path, issues[0].Message)
	}
	e := engine{ctx: ctx, machine: lock.Machine, contracts: contracts,
		commands: map[string]model.StateCommand{}, work: map[string]work{}, instances: map[string]*ownedInstance{}, order: []string{},
		result: Result{EngineVersion: Version, MachineHash: lock.Digest, ScenarioHash: hashBytes(encoded),
			Instances: []InstanceResult{}, Steps: []StepResult{}, Expectations: []ExpectationResult{}, Verified: len(scenario.Expected) != 0, Passed: len(scenario.Expected) != 0}}
	for _, command := range lock.Machine.Commands {
		e.commands[command.Name], e.work[command.Name] = command, expandedWork(command.Decision)
	}
	for _, instance := range scenario.Instances {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		values := emptyValues()
		for _, slot := range lock.Machine.State {
			values[slot.Name] = slot.Initial
		}
		for name, value := range instance.Initial {
			values[name] = value
		}
		if err := e.validateValues("effective initial state", values); err != nil {
			return Result{}, err
		}
		e.instances[instance.ID] = &ownedInstance{id: instance.ID, initial: cloneValues(values), values: cloneValues(values), receipts: []Receipt{}, receiptIndex: map[string]int{}, effects: []EffectIntent{}}
		e.order = append(e.order, instance.ID)
	}
	if err := e.checkResultBudget("", nil, 0, 0); err != nil {
		return Result{}, err
	}
	for index, step := range scenario.Steps {
		if err := e.step(index, step); err != nil {
			return Result{}, fmt.Errorf("step %d: %w", index, err)
		}
	}
	for _, expected := range scenario.Expected {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		actual := e.instances[expected.InstanceID]
		verdict := ExpectationResult{InstanceID: expected.InstanceID, Passed: true, Issues: []model.ValidationIssue{}}
		add := func(path, message string) {
			verdict.Issues = append(verdict.Issues, model.ValidationIssue{Path: path, Message: message})
		}
		for _, name := range sortedKeys(expected.State) {
			if !reflect.DeepEqual(expected.State[name], actual.values[name]) {
				add("state."+name, "Final owned value differs from the independent expectation.")
			}
		}
		if expected.Revision != actual.revision {
			add("revision", fmt.Sprintf("Expected revision %d, got %d.", expected.Revision, actual.revision))
		}
		if expected.Receipts != len(actual.receipts) {
			add("receipts", fmt.Sprintf("Expected %d receipts, got %d.", expected.Receipts, len(actual.receipts)))
		}
		if expected.Effects != len(actual.effects) {
			add("effects", fmt.Sprintf("Expected %d effects, got %d.", expected.Effects, len(actual.effects)))
		}
		verdict.Passed = len(verdict.Issues) == 0
		e.result.Passed = e.result.Passed && verdict.Passed
		e.result.Expectations = append(e.result.Expectations, verdict)
	}
	if err := e.checkResultBudget("", nil, 0, 0); err != nil {
		return Result{}, err
	}
	for _, id := range e.order {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		instance := e.instances[id]
		out := InstanceResult{ID: id, Revision: instance.revision, State: cloneValues(instance.values), Receipts: make([]Receipt, len(instance.receipts)), Effects: make([]EffectIntent, len(instance.effects))}
		for i, receipt := range instance.receipts {
			out.Receipts[i] = cloneReceipt(receipt)
		}
		for i, effect := range instance.effects {
			out.Effects[i] = cloneEffect(effect)
		}
		e.result.Instances = append(e.result.Instances, out)
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	return e.result, nil
}

func (e *engine) step(index int, step model.StateStep) error {
	if err := e.ctx.Err(); err != nil {
		return err
	}
	current := e.instances[step.InstanceID]
	result := StepResult{Index: index, Kind: step.Kind, InstanceID: step.InstanceID, CommandID: step.CommandID,
		BeforeRevision: current.revision, AfterRevision: current.revision, Before: current.values, After: current.values}
	if step.Kind == "restart" {
		next := *current
		next.values = references(current.values)
		for _, slot := range e.machine.State {
			if slot.Durability == "volatile" {
				next.values[slot.Name] = current.initial[slot.Name]
			}
		}
		if err := e.validateValues("restarted state", next.values); err != nil {
			return err
		}
		result.Status, result.After = "restarted", next.values
		return e.retain(result, &next, nil, nil)
	}
	requestHash, err := fingerprint(step)
	if err != nil {
		return err
	}
	if receiptIndex, found := current.receiptIndex[step.CommandID]; found {
		receipt := current.receipts[receiptIndex]
		if receipt.RequestHash != requestHash {
			result.Status, result.Message = "id_conflict", "Command ID already identifies a different request."
		} else {
			result.Status, result.Receipt = "replayed", &receipt
		}
		return e.retain(result, nil, nil, nil)
	}
	if *step.ExpectedRevision != current.revision {
		result.Status, result.Message = "revision_conflict", fmt.Sprintf("Expected revision %d, current revision is %d.", *step.ExpectedRevision, current.revision)
		return e.retain(result, nil, nil, nil)
	}
	command := e.commands[step.Command]
	inputs := emptyValues()
	for _, name := range sortedKeys(command.Bindings) {
		binding := command.Bindings[name]
		switch {
		case binding.State != "":
			inputs[name] = current.values[binding.State]
		case binding.Input != "":
			inputs[name] = step.Inputs[binding.Input]
		default:
			inputs[name] = *binding.Literal
		}
	}
	// The binding map may have sixteen ports, unlike the eight-entry state
	// maps. Validate its aggregate bound before making the immutable snapshot.
	if issues := model.ValidateStructuredValueMap(inputs, e.contracts); len(issues) != 0 {
		return fmt.Errorf("%w: decision inputs at %s: %s", ErrLimit, issues[0].Path, issues[0].Message)
	}
	planned := e.work[command.Name]
	reserved := &e.reserved
	if e.shared != nil {
		reserved = &e.shared.reserved
	}
	if reserved.operations+planned.operations > MaxOperations || reserved.invocations+planned.invocations > MaxInvocations {
		return fmt.Errorf("%w: global decision work exceeds %d operations or %d invocations", ErrLimit, MaxOperations, MaxInvocations)
	}
	reserved.operations += planned.operations
	reserved.invocations += planned.invocations
	if err := e.ctx.Err(); err != nil {
		return err
	}
	decision, err := behavior.Evaluate(e.ctx, command.Decision, cloneValues(inputs))
	if contextErr := e.ctx.Err(); contextErr != nil {
		return contextErr
	}
	if err != nil {
		if errors.Is(err, behavior.ErrLimit) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		result.Status, result.Message = "evaluation_error", err.Error()
		return e.retain(result, nil, nil, nil)
	}
	e.result.Operations += decision.Operations
	e.result.Invocations += decision.Invocations
	result.Decision = &decision
	outputs := mappedValues(command.Results, decision.Outputs)
	if err := e.validateValues("command results", outputs); err != nil {
		return err
	}
	accepted := *decision.Outputs[command.AcceptedOutput].Boolean
	next := *current
	next.values = references(current.values)
	effects := []EffectIntent{}
	if accepted {
		if current.revision >= model.MaxBehaviorInteger {
			return fmt.Errorf("%w: revision exceeds safe integer range", ErrLimit)
		}
		next.revision++
		for name, source := range command.Writes {
			next.values[name] = decision.Outputs[source]
		}
		if err := e.validateValues("proposed state", next.values); err != nil {
			return err
		}
		for _, binding := range command.Effects {
			if err := e.ctx.Err(); err != nil {
				return err
			}
			if binding.WhenOutput != "" && !*decision.Outputs[binding.WhenOutput].Boolean {
				continue
			}
			payload := mappedValues(binding.Payload, decision.Outputs)
			if err := e.validateValues("effect payload", payload); err != nil {
				return err
			}
			encoded, err := json.Marshal([]string{step.InstanceID, step.CommandID, binding.Effect})
			if err != nil {
				return err
			}
			effects = append(effects, EffectIntent{ID: hashBytes(encoded), InstanceID: step.InstanceID, CommandID: step.CommandID, Effect: binding.Effect, Revision: next.revision, Payload: payload})
		}
	}
	if err := e.ctx.Err(); err != nil {
		return err
	}
	if step.Fault == "before_commit" {
		result.Status, result.FaultApplied = "rolled_back", true
		return e.retain(result, nil, nil, nil)
	}
	receiptCount, effectCount := e.receiptCount, e.effectCount
	if e.shared != nil {
		receiptCount, effectCount = e.shared.receipts, e.shared.effects
	}
	if receiptCount >= MaxReceipts || effectCount+len(effects) > MaxEffects {
		return fmt.Errorf("%w: receipt or effect capacity exceeded", ErrLimit)
	}
	receipt := Receipt{CommandID: step.CommandID, RequestHash: requestHash, Command: step.Command, Accepted: accepted, Revision: next.revision, Outputs: outputs, EffectIDs: []string{}}
	for _, effect := range effects {
		receipt.EffectIDs = append(receipt.EffectIDs, effect.ID)
	}
	next.receipts = append(append([]Receipt{}, current.receipts...), receipt)
	next.effects = append(append([]EffectIntent{}, current.effects...), effects...)
	receiptBytes, err := jsonSize(receipt)
	if err != nil {
		return err
	}
	next.receiptBytes += receiptBytes
	for _, effect := range effects {
		n, err := jsonSize(effect)
		if err != nil {
			return err
		}
		next.effectBytes += n
	}
	result.Status = "committed"
	if !accepted {
		result.Status = "denied"
	}
	result.After, result.AfterRevision, result.Receipt = next.values, next.revision, &receipt
	if step.Fault == "after_commit_before_reply" {
		result.ReplyLost, result.FaultApplied = true, true
	}
	return e.retain(result, &next, &receipt, effects)
}

// retain checks the complete projected result before deep-copying repeated
// evidence or publishing staged state. A rejected budget exposes no partial run.
func (e *engine) retain(step StepResult, candidate *ownedInstance, receipt *Receipt, effects []EffectIntent) error {
	if err := e.ctx.Err(); err != nil {
		return err
	}
	n, err := stepSize(step)
	if err != nil {
		return fmt.Errorf("encode step evidence: %w", err)
	}
	id := ""
	if candidate != nil {
		id = candidate.id
	}
	if e.session && e.admit != nil {
		projection := Projection{StepBytes: n, Status: step.Status, Message: step.Message, FaultApplied: step.FaultApplied,
			Operations: e.result.Operations - e.beforeWork.operations, Invocations: e.result.Invocations - e.beforeWork.invocations}
		for _, owner := range e.order {
			instance := e.instances[owner]
			if owner == id {
				instance = candidate
			}
			bytes, err := instanceSize(instance)
			if err != nil {
				return err
			}
			projection.InstanceBytes += bytes
		}
		if step.Receipt != nil {
			projection.ReceiptBytes, err = jsonSize(step.Receipt)
			if err != nil {
				return err
			}
		}
		if err := e.admit(projection); err != nil {
			return err
		}
	} else {
		if err := e.checkResultBudget(id, candidate, n, 1); err != nil {
			return err
		}
	}
	snapshot := cloneStep(step)
	if candidate != nil {
		candidate.values = cloneValues(candidate.values)
		if receipt != nil {
			candidate.receipts[len(candidate.receipts)-1] = cloneReceipt(*receipt)
			candidate.receiptIndex[receipt.CommandID] = len(candidate.receipts) - 1
			e.receiptCount++
			if e.shared != nil {
				e.shared.receipts++
			}
			for i := range effects {
				position := len(candidate.effects) - len(effects) + i
				candidate.effects[position] = cloneEffect(effects[i])
			}
			e.effectCount += len(effects)
			if e.shared != nil {
				e.shared.effects += len(effects)
			}
		}
		e.instances[id] = candidate
	}
	if e.session {
		e.last = snapshot
	} else {
		e.result.Steps = append(e.result.Steps, snapshot)
		e.stepBytes += n
	}
	return e.ctx.Err()
}

func (e *engine) validateValues(label string, values map[string]model.TypedValue) error {
	if issues := model.ValidateStateValues(values, e.contracts); len(issues) != 0 {
		return fmt.Errorf("%w: %s at %s: %s", ErrLimit, label, issues[0].Path, issues[0].Message)
	}
	return nil
}

func mappedValues(mapping map[string]string, values map[string]model.TypedValue) map[string]model.TypedValue {
	result := emptyValues()
	for target, source := range mapping {
		result[target] = values[source]
	}
	return result
}

func fingerprint(step model.StateStep) (string, error) {
	request := struct {
		Command          string                      `json:"command"`
		ExpectedRevision int64                       `json:"expectedRevision"`
		Inputs           map[string]model.TypedValue `json:"inputs"`
	}{step.Command, *step.ExpectedRevision, step.Inputs}
	encoded, err := json.Marshal(request)
	return hashBytes(encoded), err
}

func hashBytes(encoded []byte) string {
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}

// A valid pure graph executes every node eagerly. Its exact expanded work is
// known before evaluation. Reserve that maximum even on an ordinary failure;
// public counters report only work from completed decision Results.
func expandedWork(bundle model.ComponentBundle) work {
	packages := make(map[model.ComponentRef]model.ComponentPackage, len(bundle.Packages))
	for _, lock := range bundle.Packages {
		packages[model.ComponentRef{PackageID: lock.Package.ID, Version: lock.Package.Version, Digest: lock.Digest}] = lock.Package
	}
	cache := map[model.ComponentRef]work{}
	var visit func(model.ComponentRef) work
	visit = func(ref model.ComponentRef) work {
		if prior, found := cache[ref]; found {
			return prior
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
