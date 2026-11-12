package model

import (
	"encoding/json"
	"fmt"
)

func workflowMatchValues(values map[string]TypedValue, types map[string]BehaviorValueType, complete bool) []ValidationIssue {
	issues := []ValidationIssue{}
	if complete {
		for _, name := range sortedBehaviorKeys(types) {
			if _, found := values[name]; !found {
				issues = append(issues, ValidationIssue{name, "Every declared value is required."})
			}
		}
	}
	for _, name := range sortedBehaviorKeys(values) {
		typ, found := types[name]
		if !found {
			issues = append(issues, ValidationIssue{name, "Value does not name a declared destination."})
		} else if !SameBehaviorType(ValueBehaviorType(values[name]), typ) {
			issues = append(issues, ValidationIssue{name, "Value must match the destination's exact nominal type."})
		}
	}
	return issues
}

// ValidateWorkflowScenario checks actual bounded supplied values without
// executing a graph. Unknown signals, stale task IDs and incompatible task or
// signal response schemas remain ordinary rejected events in the runtime.
func ValidateWorkflowScenario(lock WorkflowLock, scenario WorkflowScenario) []ValidationIssue {
	issues := []ValidationIssue{}
	add := func(path, message string) { issues = append(issues, ValidationIssue{path, message}) }
	appendAt := func(path string, local []ValidationIssue) {
		issues = append(issues, prefixBehaviorIssues(path, local)...)
	}
	if scenario.SchemaVersion != 1 {
		add("schemaVersion", "Only workflow scenario schema version 1 is supported.")
	}
	if !behaviorTextValid(scenario.Name, 200, true) {
		add("name", "Scenario name requires valid nonblank text of at most 200 bytes.")
	}
	if scenario.Inputs == nil || len(scenario.Inputs) > 16 {
		add("inputs", "Inputs require a nonnull map of at most 16 values.")
	}
	if scenario.Events == nil || len(scenario.Events) > 64 {
		add("events", "Events require a nonnull array of at most 64 entries.")
	}
	if scenario.Expected != nil {
		if scenario.Expected.Variables == nil || len(scenario.Expected.Variables) > 48 {
			add("expected.variables", "Expected variables require a nonnull map of at most 48 values.")
		}
		if scenario.Expected.Owners == nil || len(scenario.Expected.Owners) > 8 {
			add("expected.owners", "Expected owners require a nonnull array of at most 8 entries.")
		}
	}
	if len(issues) > 0 {
		return issues
	}
	appendAt("workflow", ValidateWorkflowLock(lock))
	if len(issues) > 0 {
		return issues
	}
	contracts, _ := WorkflowDataContracts(lock.Workflow)
	ctx := workflowContext(lock.Workflow, contracts)
	lowerBytes := 0
	checkValues := func(path string, values map[string]TypedValue, maxEntries, maxBytes int) bool {
		local := ValidateWorkflowValues(values, contracts, maxEntries, maxBytes)
		appendAt(path, local)
		if len(local) > 0 {
			return false
		}
		encoded, _ := json.Marshal(values)
		lowerBytes += len(encoded)
		if lowerBytes > MaxWorkflowScenarioBytes {
			add("", "Workflow scenario canonical JSON must not exceed 512 KiB.")
			return false
		}
		return true
	}
	if checkValues("inputs", scenario.Inputs, 16, MaxWorkflowVariablesBytes) {
		appendAt("inputs", workflowMatchValues(scenario.Inputs, ctx.Inputs, true))
	}
	if len(issues) > 0 {
		return issues
	}
	variables := map[string]TypedValue{}
	for _, variable := range lock.Workflow.Variables {
		variables[variable.Name] = variable.Initial
	}
	for i, owner := range lock.Workflow.Owners {
		values := map[string]TypedValue{}
		for _, slot := range ctx.Owners[owner.ID].Machine.State {
			values[slot.Name] = slot.Initial
		}
		for name, binding := range owner.Initial {
			switch {
			case binding.Input != "":
				values[name] = scenario.Inputs[binding.Input]
			case binding.Variable != "":
				values[name] = variables[binding.Variable]
			default:
				values[name] = *binding.Literal
			}
		}
		appendAt(fmt.Sprintf("owners[%d].initial", i), ValidateStateValues(values, contracts))
	}
	for i, event := range scenario.Events {
		at := fmt.Sprintf("events[%d]", i)
		if !behaviorName.MatchString(event.ID) {
			add(at+".id", "Event IDs must be valid local identifiers.")
		}
		switch event.Kind {
		case "response":
			if !behaviorName.MatchString(event.TaskID) {
				add(at+".taskId", "Response task ID must be a valid local identifier.")
			}
			if event.Signal != "" {
				add(at+".signal", "A response event cannot name a signal.")
			}
		case "signal":
			if !behaviorName.MatchString(event.Signal) {
				add(at+".signal", "Signal name must be a valid local identifier.")
			}
			if event.TaskID != "" {
				add(at+".taskId", "A signal event cannot name a task.")
			}
		default:
			add(at+".kind", "Event kind must be response or signal.")
		}
		checkValues(at+".values", event.Values, 8, MaxStateValuesBytes)
		if lowerBytes > MaxWorkflowScenarioBytes {
			return issues
		}
	}
	if expected := scenario.Expected; expected != nil {
		if expected.Status != "waiting" && expected.Status != "completed" && expected.Status != "failed" {
			add("expected.status", "Expected status must be waiting, completed or failed.")
		}
		if expected.Outcome != "" && !behaviorName.MatchString(expected.Outcome) {
			add("expected.outcome", "Expected outcome must be empty or a valid local identifier.")
		}
		if expected.ActiveTaskNode != "" {
			node, found := ctx.Nodes[expected.ActiveTaskNode]
			if !found || node.Kind != "task" {
				add("expected.activeTaskNode", "Expected active task must be empty or name a declared task node.")
			}
		}
		for _, count := range []struct {
			name  string
			value int
		}{{"tasks", expected.Tasks}, {"completedTasks", expected.CompletedTasks}, {"cancelledTasks", expected.CancelledTasks}, {"eventReceipts", expected.EventReceipts}} {
			if count.value < 0 || int64(count.value) > MaxBehaviorInteger {
				add("expected."+count.name, "Expected count must be a nonnegative browser-safe integer.")
			}
		}
		if checkValues("expected.variables", expected.Variables, 48, MaxWorkflowVariablesBytes) {
			appendAt("expected.variables", workflowMatchValues(expected.Variables, ctx.Variables, false))
		}
		seen := map[string]bool{}
		for i, owner := range expected.Owners {
			at := fmt.Sprintf("expected.owners[%d]", i)
			machine, found := ctx.Owners[owner.InstanceID]
			if !found {
				add(at+".instanceId", "Owner expectation must name a declared owner.")
			}
			if seen[owner.InstanceID] {
				add(at+".instanceId", "At most one expectation is allowed per owner.")
			}
			seen[owner.InstanceID] = true
			if owner.Revision < 0 || owner.Revision > MaxBehaviorInteger {
				add(at+".revision", "Expected revision must be a nonnegative browser-safe integer.")
			}
			if owner.Receipts < 0 || int64(owner.Receipts) > MaxBehaviorInteger {
				add(at+".receipts", "Expected receipt count must be a nonnegative browser-safe integer.")
			}
			if owner.Effects < 0 || int64(owner.Effects) > MaxBehaviorInteger {
				add(at+".effects", "Expected effect count must be a nonnegative browser-safe integer.")
			}
			if checkValues(at+".state", owner.State, 8, MaxStateValuesBytes) && found {
				types := map[string]BehaviorValueType{}
				for _, slot := range machine.Machine.State {
					types[slot.Name] = stateSlotType(slot)
				}
				appendAt(at+".state", workflowMatchValues(owner.State, types, true))
			}
			if lowerBytes > MaxWorkflowScenarioBytes {
				return issues
			}
		}
	}
	if len(issues) == 0 {
		encoded, err := json.Marshal(scenario)
		if err != nil || len(encoded) > MaxWorkflowScenarioBytes {
			add("", "Workflow scenario canonical JSON must not exceed 512 KiB.")
		}
	}
	return issues
}
