package model

import (
	"encoding/json"
	"fmt"
	"strings"
)

func ValidateDeliveryScenario(lock DeliveryModelLock, scenario DeliveryScenario) []ValidationIssue {
	issues := []ValidationIssue{}
	add := func(path, message string) { issues = append(issues, ValidationIssue{path, message}) }
	if scenario.SchemaVersion != 1 {
		add("schemaVersion", "Only delivery scenario schema version 1 is supported.")
	}
	if !behaviorTextValid(scenario.Name, 200, true) {
		add("name", "Scenario name requires nonblank valid text of at most 200 bytes.")
	}
	if scenario.Initial == nil || len(scenario.Initial) > 8 {
		add("initial", "Initial must be a nonnull array of at most 8 entries, one per instance.")
	}
	if scenario.Steps == nil || len(scenario.Steps) < 1 || len(scenario.Steps) > 64 {
		add("steps", "A delivery scenario requires 1–64 ordered steps.")
	}
	if scenario.Expected == nil || len(scenario.Expected) > 8 {
		add("expected", "Expected must be a nonnull array of at most 8 state expectations.")
	}
	if scenario.ExpectedMessages == nil || len(scenario.ExpectedMessages) > 32 {
		add("expectedMessages", "Expected messages must be a nonnull array of at most 32 assertions.")
	}
	if len(issues) != 0 {
		return issues
	}
	issues = append(issues, prefixBehaviorIssues("model", ValidateDeliveryModelLock(lock))...)
	if len(issues) != 0 {
		return issues
	}
	m := lock.Model
	contracts, _ := DeliveryModelDataContracts(m)
	machines := map[compositionIdentity]StateMachine{}
	for _, machine := range m.Machines {
		machines[compositionIdentity{machine.Machine.ID, machine.Machine.Version}] = machine.Machine
	}
	instances := map[string]StateMachine{}
	for _, instance := range m.Instances {
		instances[instance.ID] = machines[compositionIdentity{instance.Machine.MachineID, instance.Machine.Version}]
	}
	routes := map[string]DeliveryRoute{}
	for _, route := range m.Routes {
		routes[route.ID] = route
	}
	lowerBytes := 0
	validateValues := func(path string, values map[string]TypedValue) bool {
		local := ValidateStateValues(values, contracts)
		issues = append(issues, prefixBehaviorIssues(path, local)...)
		if len(local) != 0 {
			return false
		}
		encoded, _ := json.Marshal(values)
		lowerBytes += len(encoded)
		if lowerBytes > MaxDeliveryScenarioBytes {
			add("", "Delivery scenario canonical JSON must not exceed 512 KiB.")
			return false
		}
		return true
	}
	matchValues := func(path string, values map[string]TypedValue, types map[string]BehaviorValueType, complete bool) {
		if complete {
			for _, name := range sortedBehaviorKeys(types) {
				if _, ok := values[name]; !ok {
					add(path+"."+name, "Every declared value is required.")
				}
			}
		}
		for _, name := range sortedBehaviorKeys(values) {
			typ, ok := types[name]
			if !ok {
				add(path+"."+name, "Value does not name a declared destination.")
			} else if !SameBehaviorType(ValueBehaviorType(values[name]), typ) {
				add(path+"."+name, "Value must match the destination's exact nominal type.")
			}
		}
	}
	slotTypes := func(machine StateMachine) map[string]BehaviorValueType {
		types := map[string]BehaviorValueType{}
		for _, slot := range machine.State {
			types[slot.Name] = stateSlotType(slot)
		}
		return types
	}
	initialized := map[string]bool{}
	for i, initial := range scenario.Initial {
		at := fmt.Sprintf("initial[%d]", i)
		machine, ok := instances[initial.InstanceID]
		if !ok {
			add(at+".instanceId", "Initial entry must name a declared instance.")
		}
		if initialized[initial.InstanceID] {
			add(at+".instanceId", "Each instance requires exactly one initialization entry.")
		}
		initialized[initial.InstanceID] = true
		if !validateValues(at+".initial", initial.Initial) {
			if lowerBytes > MaxDeliveryScenarioBytes {
				return issues
			}
			continue
		}
		if ok {
			matchValues(at+".initial", initial.Initial, slotTypes(machine), false)
			effective := map[string]TypedValue{}
			for _, slot := range machine.State {
				effective[slot.Name] = slot.Initial
			}
			for name, value := range initial.Initial {
				effective[name] = value
			}
			issues = append(issues, prefixBehaviorIssues(at+".initial", ValidateStateValues(effective, contracts))...)
		}
	}
	for _, instance := range m.Instances {
		if !initialized[instance.ID] {
			add("initial", "Every declared instance requires exactly one initialization entry: "+instance.ID)
		}
	}
	type selector struct{ route, command, message string }
	aliases := map[string]selector{}
	selectors := map[selector]string{}
	revision := func(value *int64, path string) {
		if value == nil || *value < 0 || *value > MaxBehaviorInteger {
			add(path, "Expected revision must be an explicit nonnegative browser-safe integer.")
		}
	}
	for i, step := range scenario.Steps {
		at := fmt.Sprintf("steps[%d]", i)
		switch step.Kind {
		case "command":
			if step.RouteID != "" || step.SourceCommandID != "" || step.SourceMessage != "" || step.Message != "" {
				add(at, "Command steps cannot contain routing or message fields.")
			}
			machine, ok := instances[step.InstanceID]
			if !ok {
				add(at+".instanceId", "Command step must name a declared instance.")
			}
			if !behaviorName.MatchString(step.CommandID) || strings.HasPrefix(step.CommandID, "msg_") {
				add(at+".commandId", "Direct command IDs require a local identifier without the reserved msg_ prefix.")
			}
			revision(step.ExpectedRevision, at+".expectedRevision")
			if step.Fault != "" && step.Fault != "before_commit" && step.Fault != "after_commit_before_reply" {
				add(at+".fault", "Command fault must be absent, before_commit or after_commit_before_reply.")
			}
			var command *StateCommand
			for j := range machine.Commands {
				if machine.Commands[j].Name == step.Command {
					command = &machine.Commands[j]
					break
				}
			}
			if command == nil {
				add(at+".command", "Command must be declared by the selected instance's machine.")
			}
			if !validateValues(at+".inputs", step.Inputs) {
				if lowerBytes > MaxDeliveryScenarioBytes {
					return issues
				}
				continue
			}
			if command != nil {
				types := map[string]BehaviorValueType{}
				for _, port := range command.Inputs {
					types[port.Name] = PortBehaviorType(port)
				}
				matchValues(at+".inputs", step.Inputs, types, true)
			}
		case "restart":
			if _, ok := instances[step.InstanceID]; !ok {
				add(at+".instanceId", "Restart must name a declared instance.")
			}
			if step.CommandID != "" || step.Command != "" || step.ExpectedRevision != nil || step.Inputs != nil || step.Fault != "" || step.RouteID != "" || step.SourceCommandID != "" || step.SourceMessage != "" || step.Message != "" {
				add(at, "Restart permits only kind and instanceId; inputs must be omitted or null.")
			}
		case "dispatch":
			if step.InstanceID != "" || step.CommandID != "" || step.Command != "" || step.Inputs != nil || step.Fault != "" {
				add(at, "Dispatch cannot contain owner command fields, inputs or faults.")
			}
			route, ok := routes[step.RouteID]
			if !ok {
				add(at+".routeId", "Dispatch must name a declared route.")
			}
			if !behaviorName.MatchString(step.Message) {
				add(at+".message", "Message aliases must be valid local identifiers.")
			}
			revision(step.ExpectedRevision, at+".expectedRevision")
			if (step.SourceCommandID == "") == (step.SourceMessage == "") {
				add(at, "Dispatch requires exactly one sourceCommandId or sourceMessage.")
			}
			if step.SourceCommandID != "" && (!behaviorName.MatchString(step.SourceCommandID) || strings.HasPrefix(step.SourceCommandID, "msg_")) {
				add(at+".sourceCommandId", "Direct source command ID must be a local identifier without the reserved msg_ prefix.")
			}
			if step.SourceMessage != "" {
				previous, exists := aliases[step.SourceMessage]
				if !exists {
					add(at+".sourceMessage", "Source message must name an earlier declared dispatch alias.")
				} else if previousRoute, found := routes[previous.route]; ok && found && previousRoute.TargetInstanceID != route.SourceInstanceID {
					add(at+".sourceMessage", "Earlier message target must be this route's source instance.")
				}
			}
			key := selector{step.RouteID, step.SourceCommandID, step.SourceMessage}
			if previous, exists := aliases[step.Message]; exists && previous != key {
				add(at+".message", "A repeated dispatch alias must retain its route and source selector.")
			}
			if previous, exists := selectors[key]; exists && previous != step.Message {
				add(at+".message", "One route/source selector may be declared under only one alias.")
			}
			if _, exists := aliases[step.Message]; !exists {
				aliases[step.Message] = key
			}
			if _, exists := selectors[key]; !exists {
				selectors[key] = step.Message
			}
		case "deliver":
			if step.InstanceID != "" || step.CommandID != "" || step.Command != "" || step.ExpectedRevision != nil || step.Inputs != nil || step.RouteID != "" || step.SourceCommandID != "" || step.SourceMessage != "" {
				add(at, "Deliver permits only kind, message alias and an optional delivery fault; inputs must be null.")
			}
			if _, exists := aliases[step.Message]; !exists {
				add(at+".message", "Delivery must name an earlier declared dispatch alias.")
			}
			if step.Fault != "" && step.Fault != "drop_before_receive" && step.Fault != "before_recipient_commit" && step.Fault != "after_recipient_commit_before_ack" {
				add(at+".fault", "Unsupported delivery fault.")
			}
		default:
			add(at+".kind", "Step kind must be command, restart, dispatch or deliver.")
		}
	}
	seen := map[string]bool{}
	for i, expectation := range scenario.Expected {
		at := fmt.Sprintf("expected[%d]", i)
		machine, ok := instances[expectation.InstanceID]
		if !ok {
			add(at+".instanceId", "Expectation must name a declared instance.")
		}
		if seen[expectation.InstanceID] {
			add(at+".instanceId", "At most one state expectation is allowed per instance.")
		}
		seen[expectation.InstanceID] = true
		if expectation.Revision < 0 || expectation.Revision > MaxBehaviorInteger {
			add(at+".revision", "Expected revision must be a nonnegative browser-safe integer.")
		}
		if expectation.Receipts < 0 || int64(expectation.Receipts) > MaxBehaviorInteger {
			add(at+".receipts", "Expected receipt count must be a nonnegative browser-safe integer.")
		}
		if expectation.Effects < 0 || int64(expectation.Effects) > MaxBehaviorInteger {
			add(at+".effects", "Expected effect count must be a nonnegative browser-safe integer.")
		}
		if !validateValues(at+".state", expectation.State) {
			if lowerBytes > MaxDeliveryScenarioBytes {
				return issues
			}
			continue
		}
		if ok {
			matchValues(at+".state", expectation.State, slotTypes(machine), true)
		}
	}
	messageNames := map[string]bool{}
	for i, expectation := range scenario.ExpectedMessages {
		at := fmt.Sprintf("expectedMessages[%d]", i)
		if !behaviorName.MatchString(expectation.Name) {
			add(at+".name", "Expected message name must be a valid local identifier.")
		}
		if messageNames[expectation.Name] {
			add(at+".name", "Expected message names must be unique.")
		}
		messageNames[expectation.Name] = true
		if expectation.Attempts < 0 || int64(expectation.Attempts) > MaxBehaviorInteger {
			add(at+".attempts", "Expected attempts must be a nonnegative browser-safe integer.")
		}
		switch expectation.Status {
		case "absent":
			if expectation.Attempts != 0 || expectation.Accepted != nil {
				add(at, "An absent message requires zero attempts and no accepted assertion.")
			}
		case "pending":
			if expectation.Accepted != nil {
				add(at+".accepted", "A pending message cannot assert a business acceptance value.")
			}
		case "acknowledged":
			if expectation.Attempts < 1 {
				add(at+".attempts", "An acknowledged message requires at least one attempt.")
			}
		default:
			add(at+".status", "Expected status must be absent, pending or acknowledged.")
		}
		if expectation.Status == "pending" || expectation.Status == "acknowledged" {
			if _, ok := aliases[expectation.Name]; !ok {
				add(at+".name", "Pending or acknowledged expectations must name a declared dispatch alias.")
			}
		}
	}
	if len(issues) == 0 {
		encoded, err := json.Marshal(scenario)
		if err != nil || len(encoded) > MaxDeliveryScenarioBytes {
			add("", "Delivery scenario canonical JSON must not exceed 512 KiB.")
		}
	}
	return issues
}
