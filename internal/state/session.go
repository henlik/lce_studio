package state

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"software-studio/internal/model"
)

// Budget is shared by every host in an incrementally executed experiment.
// It reserves failed decision work too, and never silently evicts durable facts.
// Sessions and their Budget are sequential and are not safe for concurrent use.
type Budget struct {
	reserved work
	receipts int
	effects  int
}

// Projection contains only scalar accounting, never mutable owned values. An
// outer experiment admits its complete projected evidence before state commits
// or snapshots are deep-copied. InstanceBytes excludes array delimiters/commas.
type Projection struct {
	StepBytes, InstanceBytes, ReceiptBytes int
	Operations, Invocations                int
	Status, Message                        string
	FaultApplied                           bool
}

// Session reuses the exact v0.6 transition kernel without retaining a history.
// Its caller retains each returned step once and must discard the experiment
// after any error. A nil admission callback applies the legacy local byte bound.
type Session struct {
	engine  *engine
	lock    model.StateMachineLock
	initial []model.StateInstance
	steps   int
	failed  error
}

func NewSession(ctx context.Context, lock model.StateMachineLock, instances []model.StateInstance, budget *Budget) (*Session, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(instances) < 1 || len(instances) > 8 {
		return nil, fmt.Errorf("session requires1–8 instances")
	}
	validation := model.StateScenario{SchemaVersion: 1, Name: "Incremental session", Instances: instances,
		Steps: []model.StateStep{{Kind: "restart", InstanceID: instances[0].ID}}, Expected: []model.StateExpectation{}}
	if issues := model.ValidateStateScenario(lock, validation); len(issues) > 0 {
		return nil, fmt.Errorf("invalid session at %s: %s", issues[0].Path, issues[0].Message)
	}
	// Validation bounded the definition and initial values before this one-time
	// ownership copy. Later caller edits cannot rewrite a running session.
	owned, err := json.Marshal(struct {
		Lock      model.StateMachineLock
		Instances []model.StateInstance
	}{lock, instances})
	if err != nil {
		return nil, err
	}
	var snapshot struct {
		Lock      model.StateMachineLock
		Instances []model.StateInstance
	}
	if err := json.Unmarshal(owned, &snapshot); err != nil {
		return nil, err
	}
	lock, instances = snapshot.Lock, snapshot.Instances
	contracts, issues := model.StateMachineDataContracts(lock.Machine)
	if len(issues) > 0 {
		return nil, fmt.Errorf("invalid data union at %s: %s", issues[0].Path, issues[0].Message)
	}
	if budget == nil {
		budget = &Budget{}
	}
	e := &engine{ctx: ctx, machine: lock.Machine, contracts: contracts, commands: map[string]model.StateCommand{}, work: map[string]work{}, instances: map[string]*ownedInstance{}, order: []string{}, shared: budget, session: true,
		result: Result{EngineVersion: Version, MachineHash: lock.Digest, Instances: []InstanceResult{}, Steps: []StepResult{}, Expectations: []ExpectationResult{}}}
	for _, command := range lock.Machine.Commands {
		e.commands[command.Name], e.work[command.Name] = command, expandedWork(command.Decision)
	}
	for _, instance := range instances {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		values := emptyValues()
		for _, slot := range lock.Machine.State {
			values[slot.Name] = slot.Initial
		}
		for name, value := range instance.Initial {
			values[name] = value
		}
		if err := e.validateValues("effective initial state", values); err != nil {
			return nil, err
		}
		e.instances[instance.ID] = &ownedInstance{id: instance.ID, initial: cloneValues(values), values: cloneValues(values), receipts: []Receipt{}, receiptIndex: map[string]int{}, effects: []EffectIntent{}}
		e.order = append(e.order, instance.ID)
	}
	if err := e.checkResultBudget("", nil, 0, 0); err != nil {
		return nil, err
	}
	return &Session{engine: e, lock: lock, initial: instances}, nil
}

func (s *Session) Apply(ctx context.Context, index int, step model.StateStep, admit func(Projection) error) (StepResult, error) {
	if s.failed != nil {
		return StepResult{}, s.failed
	}
	fail := func(err error) (StepResult, error) { s.failed = err; return StepResult{}, err }
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	if s.steps >= 64 || index < 0 || index >= 64 {
		return fail(fmt.Errorf("%w: at most64 indexed session steps", ErrLimit))
	}
	validation := model.StateScenario{SchemaVersion: 1, Name: "Incremental session", Instances: s.initial, Steps: []model.StateStep{step}, Expected: []model.StateExpectation{}}
	if issues := model.ValidateStateScenario(s.lock, validation); len(issues) > 0 {
		return fail(fmt.Errorf("invalid step at %s: %s", issues[0].Path, issues[0].Message))
	}
	e := s.engine
	e.ctx, e.admit = ctx, admit
	e.beforeWork = work{e.result.Operations, e.result.Invocations}
	if err := e.step(index, step); err != nil {
		return fail(err)
	}
	s.steps++
	result := e.last
	e.last, e.admit = StepResult{}, nil
	return result, nil
}

func (s *Session) InstanceBytes() (int, error) {
	total := 0
	for _, id := range s.engine.order {
		n, err := instanceSize(s.engine.instances[id])
		if err != nil {
			return 0, err
		}
		total += n
	}
	return total, nil
}

func (s *Session) Snapshot(ctx context.Context) ([]InstanceResult, error) {
	if s.failed != nil {
		return nil, s.failed
	}
	results := make([]InstanceResult, 0, len(s.engine.order))
	for _, id := range s.engine.order {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		instance := s.engine.instances[id]
		result := InstanceResult{ID: id, Revision: instance.revision, State: cloneValues(instance.values), Receipts: make([]Receipt, len(instance.receipts)), Effects: make([]EffectIntent, len(instance.effects))}
		for i, receipt := range instance.receipts {
			result.Receipts[i] = cloneReceipt(receipt)
		}
		for i, effect := range instance.effects {
			result.Effects[i] = cloneEffect(effect)
		}
		results = append(results, result)
	}
	return results, nil
}

func (s *Session) Effect(instanceID, commandID, effectName string) (EffectIntent, bool) {
	instance, found := s.engine.instances[instanceID]
	if !found {
		return EffectIntent{}, false
	}
	for _, effect := range instance.effects {
		if effect.CommandID == commandID && effect.Effect == effectName {
			return cloneEffect(effect), true
		}
	}
	return EffectIntent{}, false
}

func (s *Session) Expectation(expected model.StateExpectation) (ExpectationResult, error) {
	actual, found := s.engine.instances[expected.InstanceID]
	if !found {
		return ExpectationResult{}, fmt.Errorf("unknown expectation instance")
	}
	if issues := model.ValidateStateValues(expected.State, s.engine.contracts); len(issues) > 0 {
		return ExpectationResult{}, fmt.Errorf("invalid expected values: %s", issues[0].Message)
	}
	result := ExpectationResult{InstanceID: expected.InstanceID, Issues: []model.ValidationIssue{}}
	add := func(path, message string) {
		result.Issues = append(result.Issues, model.ValidationIssue{Path: path, Message: message})
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
	result.Passed = len(result.Issues) == 0
	return result, nil
}
