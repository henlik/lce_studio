package model

import (
	"encoding/json"
	"fmt"
)

// ValidateStateScenario checks independently supplied experiment inputs and
// expectations; it never executes a command or derives expected outcomes.
func ValidateStateScenario(lock StateMachineLock, scenario StateScenario) []ValidationIssue {
	issues := []ValidationIssue{}
	add := func(path, message string) { issues = append(issues, ValidationIssue{path, message}) }
	if scenario.SchemaVersion != 1 {
		add("schemaVersion", "Only state scenario schema version 1 is supported.")
	}
	if !behaviorTextValid(scenario.Name, 200, true) {
		add("name", "Scenario name requires nonblank valid text of at most 200 bytes.")
	}
	if scenario.Instances == nil || len(scenario.Instances) < 1 || len(scenario.Instances) > 8 {
		add("instances", "A scenario requires 1–8 instances.")
	}
	if scenario.Steps == nil || len(scenario.Steps) < 1 || len(scenario.Steps) > 64 {
		add("steps", "A scenario requires 1–64 ordered steps.")
	}
	if scenario.Expected == nil || len(scenario.Expected) > 8 {
		add("expected", "Expected must be a nonnull array of at most 8 expectations.")
	}
	if len(issues) != 0 {
		return issues
	}
	issues = append(issues, prefixBehaviorIssues("machine", ValidateStateMachineLock(lock))...)
	if len(issues) != 0 {
		return issues
	}
	m := lock.Machine
	contracts, _ := StateMachineDataContracts(m)
	slots := map[string]BehaviorValueType{}
	defaults := map[string]TypedValue{}
	for _, slot := range m.State {
		slots[slot.Name] = stateSlotType(slot)
		defaults[slot.Name] = slot.Initial
	}
	commands := map[string]StateCommand{}
	for _, c := range m.Commands {
		commands[c.Name] = c
	}
	instances := map[string]bool{}
	lowerBytes := 0
	boundedValues := func(path string, values map[string]TypedValue) bool {
		local := ValidateStateValues(values, contracts)
		issues = append(issues, prefixBehaviorIssues(path, local)...)
		if len(local) != 0 {
			return false
		}
		encoded, _ := json.Marshal(values)
		lowerBytes += len(encoded)
		if lowerBytes > MaxStateScenarioBytes {
			add("", "Scenario canonical JSON must not exceed 512 KiB.")
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
	for i, instance := range scenario.Instances {
		at := fmt.Sprintf("instances[%d]", i)
		if !behaviorName.MatchString(instance.ID) {
			add(at+".id", "Instance ID must be a valid local identifier.")
		}
		if instances[instance.ID] {
			add(at+".id", "Instance IDs must be unique.")
		}
		instances[instance.ID] = true
		if !behaviorTextValid(instance.Name, 200, true) {
			add(at+".name", "Instance name requires nonblank valid text of at most 200 bytes.")
		}
		if !boundedValues(at+".initial", instance.Initial) {
			if lowerBytes > MaxStateScenarioBytes {
				return issues
			}
			continue
		}
		matchValues(at+".initial", instance.Initial, slots, false)
		// A small override map can combine with defaults into an oversized state.
		// Validate the complete effective state before the runtime can copy it.
		effective := make(map[string]TypedValue, len(defaults))
		for name, value := range defaults {
			effective[name] = value
		}
		for name, value := range instance.Initial {
			effective[name] = value
		}
		issues = append(issues, prefixBehaviorIssues(at+".initial", ValidateStateValues(effective, contracts))...)
	}
	for i, step := range scenario.Steps {
		at := fmt.Sprintf("steps[%d]", i)
		if !instances[step.InstanceID] {
			add(at+".instanceId", "Step must name a declared instance.")
		}
		switch step.Kind {
		case "restart":
			if step.CommandID != "" || step.Command != "" || step.ExpectedRevision != nil || step.Inputs != nil || step.Fault != "" {
				add(at, "Restart permits only kind and instanceId; inputs must be omitted or null.")
			}
		case "command":
			if !behaviorName.MatchString(step.CommandID) {
				add(at+".commandId", "Command ID must be a valid local identifier.")
			}
			command, ok := commands[step.Command]
			if !ok {
				add(at+".command", "Step must name a declared command.")
			}
			if step.ExpectedRevision == nil || *step.ExpectedRevision < 0 || *step.ExpectedRevision > MaxBehaviorInteger {
				add(at+".expectedRevision", "Expected revision must be a nonnegative browser-safe integer; zero must be supplied explicitly.")
			}
			if step.Fault != "" && step.Fault != "before_commit" && step.Fault != "after_commit_before_reply" {
				add(at+".fault", "Fault must be absent, before_commit or after_commit_before_reply.")
			}
			if !boundedValues(at+".inputs", step.Inputs) {
				if lowerBytes > MaxStateScenarioBytes {
					return issues
				}
				continue
			}
			if ok {
				types := map[string]BehaviorValueType{}
				for _, port := range command.Inputs {
					types[port.Name] = PortBehaviorType(port)
				}
				matchValues(at+".inputs", step.Inputs, types, true)
			}
		default:
			add(at+".kind", "Step kind must be command or restart.")
		}
	}
	expected := map[string]bool{}
	for i, expectation := range scenario.Expected {
		at := fmt.Sprintf("expected[%d]", i)
		if !instances[expectation.InstanceID] {
			add(at+".instanceId", "Expectation must name a declared instance.")
		}
		if expected[expectation.InstanceID] {
			add(at+".instanceId", "At most one expectation is allowed per instance.")
		}
		expected[expectation.InstanceID] = true
		if expectation.Revision < 0 || expectation.Revision > MaxBehaviorInteger {
			add(at+".revision", "Expected revision must be a nonnegative browser-safe integer.")
		}
		if expectation.Receipts < 0 || int64(expectation.Receipts) > MaxBehaviorInteger {
			add(at+".receipts", "Expected receipt count must be a nonnegative browser-safe integer.")
		}
		if expectation.Effects < 0 || int64(expectation.Effects) > MaxBehaviorInteger {
			add(at+".effects", "Expected effect count must be a nonnegative browser-safe integer.")
		}
		if !boundedValues(at+".state", expectation.State) {
			if lowerBytes > MaxStateScenarioBytes {
				return issues
			}
			continue
		}
		matchValues(at+".state", expectation.State, slots, true)
	}
	if len(issues) == 0 {
		encoded, err := json.Marshal(scenario)
		if err != nil || len(encoded) > MaxStateScenarioBytes {
			add("", "Scenario canonical JSON must not exceed 512 KiB.")
		}
	}
	return issues
}
