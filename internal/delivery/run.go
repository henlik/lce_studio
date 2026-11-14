package delivery

import (
	"context"
	"crypto/sha256"
	"encoding/base32"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"software-studio/internal/model"
	"software-studio/internal/state"
)

type host struct {
	session *state.Session
	bytes   int
}
type intentRoute struct{ intent, route string }
type engine struct {
	instanceIDs   map[string]string
	namespace     string
	ctx           context.Context
	model         model.DeliveryModel
	contracts     []model.DataContractLock
	hosts         []*host
	owners        map[string]*host
	routes        map[string]model.DeliveryRoute
	commands      map[string]map[string]model.StateCommand
	messages      []Message
	messageBytes  []int
	byName        map[string]int
	byIntentRoute map[intentRoute]int
	result        Result
	stepBytes     int
}

func emptyInstances() []state.InstanceResult       { return []state.InstanceResult{} }
func emptyExpectations() []state.ExpectationResult { return []state.ExpectationResult{} }
func hashBytes(data []byte) string                 { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }

func Run(ctx context.Context, lock model.DeliveryModelLock, scenario model.DeliveryScenario) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if issues := model.ValidateDeliveryModelLock(lock); len(issues) > 0 {
		return Result{}, fmt.Errorf("invalid model at %s: %s", issues[0].Path, issues[0].Message)
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if issues := model.ValidateDeliveryScenario(lock, scenario); len(issues) > 0 {
		return Result{}, fmt.Errorf("invalid scenario at %s: %s", issues[0].Path, issues[0].Message)
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	encoded, err := json.Marshal(scenario)
	if err != nil {
		return Result{}, err
	}
	contracts, issues := model.DeliveryModelDataContracts(lock.Model)
	if len(issues) > 0 {
		return Result{}, fmt.Errorf("invalid data union at %s: %s", issues[0].Path, issues[0].Message)
	}
	verified := len(scenario.Expected)+len(scenario.ExpectedMessages) > 0
	e := engine{ctx: ctx, model: lock.Model, contracts: contracts, hosts: []*host{}, owners: map[string]*host{}, routes: map[string]model.DeliveryRoute{}, commands: map[string]map[string]model.StateCommand{}, messages: []Message{}, messageBytes: []int{}, byName: map[string]int{}, byIntentRoute: map[intentRoute]int{},
		result: Result{EngineVersion: Version, ModelHash: lock.Digest, ScenarioHash: hashBytes(encoded), Instances: emptyInstances(), Messages: []Message{}, Steps: []StepResult{}, Verified: verified, Passed: verified, Expectations: emptyExpectations(), MessageExpectations: []MessageExpectationResult{}}}
	initial := map[string]map[string]model.TypedValue{}
	for _, instance := range scenario.Initial {
		initial[instance.InstanceID] = instance.Initial
	}
	budget := &state.Budget{}
	for _, machine := range lock.Model.Machines {
		ref := model.DeliveryMachineRef{MachineID: machine.Machine.ID, Version: machine.Machine.Version, Digest: machine.Digest}
		instances := []model.StateInstance{}
		for _, instance := range lock.Model.Instances {
			if instance.Machine == ref {
				instances = append(instances, model.StateInstance{ID: instance.ID, Name: instance.Name, Initial: initial[instance.ID]})
			}
		}
		if len(instances) == 0 {
			continue
		}
		session, err := state.NewSession(ctx, machine, instances, budget)
		if err != nil {
			return Result{}, err
		}
		n, err := session.InstanceBytes()
		if err != nil {
			return Result{}, err
		}
		h := &host{session: session, bytes: n}
		e.hosts = append(e.hosts, h)
		commands := map[string]model.StateCommand{}
		for _, command := range machine.Machine.Commands {
			commands[command.Name] = command
		}
		for _, instance := range instances {
			e.owners[instance.ID], e.commands[instance.ID] = h, commands
		}
	}
	for _, route := range lock.Model.Routes {
		e.routes[route.ID] = route
	}
	if err := e.checkBudget(0, 0, nil, 0, -1, 0, 0, 0); err != nil {
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
		verdict, err := e.owners[expected.InstanceID].session.Expectation(expected)
		if err != nil {
			return Result{}, err
		}
		e.result.Expectations = append(e.result.Expectations, verdict)
		e.result.Passed = e.result.Passed && verdict.Passed
	}
	for _, expected := range scenario.ExpectedMessages {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		verdict := MessageExpectationResult{Name: expected.Name, Issues: []model.ValidationIssue{}}
		add := func(path, message string) {
			verdict.Issues = append(verdict.Issues, model.ValidationIssue{Path: path, Message: message})
		}
		status, attempts := "absent", 0
		var acknowledgement *state.Receipt
		if index, found := e.byName[expected.Name]; found {
			message := e.messages[index]
			status, attempts, acknowledgement = message.Status, message.Attempts, message.Acknowledgement
		}
		if expected.Status != status {
			add("status", fmt.Sprintf("Expected %s, got %s.", expected.Status, status))
		}
		if expected.Attempts != attempts {
			add("attempts", fmt.Sprintf("Expected %d attempts, got %d.", expected.Attempts, attempts))
		}
		if expected.Accepted != nil && (acknowledgement == nil || acknowledgement.Accepted != *expected.Accepted) {
			add("accepted", "Acknowledged business verdict differs from the independent expectation.")
		}
		verdict.Passed = len(verdict.Issues) == 0
		e.result.MessageExpectations = append(e.result.MessageExpectations, verdict)
		e.result.Passed = e.result.Passed && verdict.Passed
	}
	if err := e.checkBudget(0, 0, nil, 0, -1, 0, 0, 0); err != nil {
		return Result{}, err
	}
	instances := map[string]state.InstanceResult{}
	for _, h := range e.hosts {
		snapshots, err := h.session.Snapshot(ctx)
		if err != nil {
			return Result{}, err
		}
		for _, instance := range snapshots {
			instances[instance.ID] = instance
		}
	}
	for _, instance := range lock.Model.Instances {
		e.result.Instances = append(e.result.Instances, instances[instance.ID])
	}
	for _, message := range e.messages {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		e.result.Messages = append(e.result.Messages, cloneMessage(message))
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	return e.result, nil
}

func (e *engine) step(index int, step model.DeliveryStep) error {
	if err := e.ctx.Err(); err != nil {
		return err
	}
	result := StepResult{Index: index, Kind: step.Kind, InstanceID: step.InstanceID}
	switch step.Kind {
	case "command", "restart":
		return e.kernel(result, model.StateStep{Kind: step.Kind, InstanceID: step.InstanceID, CommandID: step.CommandID, Command: step.Command, ExpectedRevision: step.ExpectedRevision, Inputs: step.Inputs, Fault: step.Fault}, nil, -1, "")
	case "dispatch":
		return e.dispatch(result, step)
	case "deliver":
		result.MessageName = step.Message
		messageIndex, found := e.byName[step.Message]
		if !found {
			result.Status, result.Message = "missing_message", "The dispatch alias has no materialized message."
			return e.retain(result, nil, -1)
		}
		message := e.messages[messageIndex]
		result.InstanceID, result.MessageID = message.TargetInstanceID, message.ID
		result.BeforeStatus, result.AfterStatus = message.Status, message.Status
		result.BeforeAttempts, result.AfterAttempts = message.Attempts, message.Attempts
		if message.Status == "acknowledged" {
			result.Status = "already_acknowledged"
			return e.retain(result, nil, -1)
		}
		message.Attempts++
		result.AfterAttempts = message.Attempts
		if step.Fault == "drop_before_receive" {
			result.Status, result.Message, result.FaultApplied = "dropped", "Message was dropped before recipient execution.", true
			return e.retain(result, &message, messageIndex)
		}
		fault := ""
		if step.Fault == "before_recipient_commit" {
			fault = "before_commit"
		}
		revision := message.ExpectedRevision
		return e.kernel(result, model.StateStep{Kind: "command", InstanceID: message.TargetInstanceID, CommandID: message.ID, Command: message.Command, ExpectedRevision: &revision, Inputs: message.Inputs, Fault: fault}, &message, messageIndex, step.Fault)
	}
	return fmt.Errorf("unsupported delivery step")
}

func (e *engine) dispatch(result StepResult, step model.DeliveryStep) error {
	route := e.routes[step.RouteID]
	result.InstanceID, result.MessageName = route.SourceInstanceID, step.Message
	if prior, found := e.byName[step.Message]; found {
		message := e.messages[prior]
		result.MessageID, result.BeforeStatus, result.AfterStatus = message.ID, message.Status, message.Status
		result.BeforeAttempts, result.AfterAttempts = message.Attempts, message.Attempts
	}
	commandID := step.SourceCommandID
	if step.SourceMessage != "" {
		index, found := e.byName[step.SourceMessage]
		if !found {
			result.Status, result.Message = "missing_source_message", "The upstream dispatch alias has no materialized message."
			return e.retain(result, nil, -1)
		}
		commandID = e.messages[index].ID
	}
	intent, found := e.owners[route.SourceInstanceID].session.Effect(e.stateInstance(route.SourceInstanceID), commandID, route.Effect)
	if !found {
		result.Status, result.Message = "missing_intent", "No committed source effect exists for this selector and route."
		return e.retain(result, nil, -1)
	}
	identity := []string{intent.ID, route.ID}
	if e.namespace != "" {
		identity = append(identity, e.namespace)
	}
	encoded, err := json.Marshal(identity)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(encoded)
	id := "msg_" + strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(digest[:]))
	inputs := map[string]model.TypedValue{}
	for target, source := range route.Inputs {
		inputs[target] = intent.Payload[source]
	}
	if issues := model.ValidateStateValues(inputs, e.contracts); len(issues) > 0 {
		return fmt.Errorf("%w: mapped inputs at %s: %s", ErrLimit, issues[0].Path, issues[0].Message)
	}
	if issues := model.ValidateStructuredBehaviorValues(inputs, e.commands[route.TargetInstanceID][route.Command].Inputs, e.contracts); len(issues) > 0 {
		return fmt.Errorf("invalid mapped types at %s: %s", issues[0].Path, issues[0].Message)
	}
	message := Message{Name: step.Message, ID: id, RouteID: route.ID, SourceIntentID: intent.ID, SourceInstanceID: route.SourceInstanceID, SourceCommandID: commandID, Effect: route.Effect, TargetInstanceID: route.TargetInstanceID, Command: route.Command, ExpectedRevision: *step.ExpectedRevision, Inputs: inputs, Status: "pending"}
	prior, aliasFound := e.byName[step.Message]
	byKey, keyFound := e.byIntentRoute[intentRoute{intent.ID, route.ID}]
	if aliasFound || keyFound {
		if !aliasFound {
			prior = byKey
		}
		old := e.messages[prior]
		result.MessageID = old.ID
		result.Status = "dispatch_conflict"
		result.Message = "Dispatch differs from the existing immutable envelope."
		if aliasFound && keyFound && prior == byKey && sameEnvelope(old, message) {
			result.Status, result.Message = "dispatch_replayed", ""
		}
		return e.retain(result, nil, -1)
	}
	if len(e.messages) >= MaxMessages {
		return fmt.Errorf("%w: message capacity exceeds %d", ErrLimit, MaxMessages)
	}
	result.Status, result.MessageID, result.AfterStatus = "dispatched", id, "pending"
	return e.retain(result, &message, len(e.messages))
}

func sameEnvelope(left, right Message) bool {
	left.Status, left.Attempts, left.Acknowledgement = "pending", 0, nil
	right.Status, right.Attempts, right.Acknowledgement = "pending", 0, nil
	return reflect.DeepEqual(left, right)
}

func (e *engine) kernel(outer StepResult, request model.StateStep, message *Message, messageIndex int, fault string) error {
	h := e.owners[request.InstanceID]
	request.InstanceID = e.stateInstance(request.InstanceID)
	var projection state.Projection
	var stepBytes, messageBytes int
	stateResult, err := h.session.Apply(e.ctx, outer.Index, request, func(p state.Projection) error {
		projection = p
		outer.Status, outer.Message, outer.FaultApplied = p.Status, p.Message, p.FaultApplied
		ackBytes := 0
		if message != nil {
			outer.Status = "recipient_unacknowledged"
			if p.ReceiptBytes != 0 {
				if fault == "after_recipient_commit_before_ack" {
					outer.Status, outer.Message, outer.FaultApplied = "acknowledgement_lost", "Recipient processing completed, but its acknowledgement was lost.", true
				} else {
					outer.Status, outer.Message = "acknowledged", ""
					message.Status, ackBytes = "acknowledged", p.ReceiptBytes
				}
			}
			outer.AfterStatus, outer.AfterAttempts = message.Status, message.Attempts
			n, err := jsonSize(message)
			if err != nil {
				return err
			}
			messageBytes = n
			if ackBytes != 0 {
				messageBytes += len(",\"acknowledgement\":") + ackBytes
			}
		}
		n, err := jsonSize(outer)
		if err != nil {
			return err
		}
		stepBytes = n + len(",\"state\":") + p.StepBytes
		return e.checkBudget(stepBytes, 1, h, p.InstanceBytes, messageIndex, messageBytes, p.Operations, p.Invocations)
	})
	if err != nil {
		return err
	}
	outer.State = &stateResult
	if message != nil {
		if message.Status == "acknowledged" {
			message.Acknowledgement = cloneReceipt(*stateResult.Receipt)
		}
		// Frozen inputs are already privately owned; transport changes only its
		// counters/status. The acknowledgement is a separate historical copy.
		e.publishMessage(*message, messageIndex, messageBytes)
	}
	h.bytes = projection.InstanceBytes
	e.result.Operations += projection.Operations
	e.result.Invocations += projection.Invocations
	e.result.Steps = append(e.result.Steps, outer)
	e.stepBytes += stepBytes
	return e.ctx.Err()
}
