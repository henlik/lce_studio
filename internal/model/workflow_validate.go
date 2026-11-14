package model

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
)

const MaxWorkflowDefinitionBytes = 1 << 20
const MaxWorkflowScenarioBytes = 512 << 10
const MaxWorkflowVariablesBytes = 256 << 10

// WorkflowContext resolves only exact embedded definitions. It never loads a
// catalog or fills a missing dependency from a different independent closure.
type WorkflowContext struct {
	Contracts                  []DataContractLock
	Inputs, Outputs, Variables map[string]BehaviorValueType
	Nodes                      map[string]WorkflowNode
	Owners                     map[string]StateMachineLock
	Decisions                  map[ComponentRef]ComponentBundle
	Signals                    map[string]WorkflowSignal
}

// ValidateWorkflowValues bounds structure and each value before aggregate JSON
// encoding. It supports the larger workflow variable map without widening the
// existing state/pure-component public maps.
func ValidateWorkflowValues(values map[string]TypedValue, contracts []DataContractLock, maxEntries, maxBytes int) []ValidationIssue {
	issues := []ValidationIssue{}
	if values == nil || len(values) > maxEntries || maxEntries < 0 || maxEntries > 48 || maxBytes < 0 || maxBytes > MaxWorkflowVariablesBytes {
		return []ValidationIssue{{"", fmt.Sprintf("A nonnull map of at most %d values is required.", maxEntries)}}
	}
	lowerBytes := 2
	for _, name := range sortedBehaviorKeys(values) {
		if !behaviorName.MatchString(name) {
			issues = append(issues, ValidationIssue{name, "Value names must be valid local identifiers."})
		}
		local := ValidateStructuredTypedValue(values[name], contracts)
		issues = append(issues, prefixBehaviorIssues(name, local)...)
		if len(local) == 0 {
			encoded, _ := json.Marshal(values[name])
			lowerBytes += len(encoded)
			if lowerBytes > maxBytes {
				return append(issues, ValidationIssue{"", fmt.Sprintf("Value map must not exceed %d bytes of canonical JSON.", maxBytes)})
			}
		}
	}
	if len(issues) == 0 {
		encoded, err := json.Marshal(values)
		if err != nil || len(encoded) > maxBytes {
			issues = append(issues, ValidationIssue{"", fmt.Sprintf("Value map must not exceed %d bytes of canonical JSON.", maxBytes)})
		}
	}
	return issues
}

func workflowBounds(w WorkflowDefinition) []ValidationIssue {
	issues := []ValidationIssue{}
	check := func(path string, nilValue bool, count, limit int) {
		if nilValue || count > limit {
			issues = append(issues, ValidationIssue{path, fmt.Sprintf("A nonnull collection of at most %d entries is required.", limit)})
		}
	}
	check("dataContracts", w.DataContracts == nil, len(w.DataContracts), 8)
	check("decisions", w.Decisions == nil, len(w.Decisions), 32)
	check("machines", w.Machines == nil, len(w.Machines), 8)
	check("owners", w.Owners == nil, len(w.Owners), 8)
	check("inputs", w.Inputs == nil, len(w.Inputs), 16)
	check("outputs", w.Outputs == nil, len(w.Outputs), 16)
	check("variables", w.Variables == nil, len(w.Variables), 48)
	check("nodes", w.Nodes == nil, len(w.Nodes), 96)
	check("signals", w.Signals == nil, len(w.Signals), 8)
	if len(w.Nodes) == 0 {
		issues = append(issues, ValidationIssue{"nodes", "At least one workflow node is required."})
	}
	if len(issues) > 0 {
		return issues
	}
	for i, owner := range w.Owners {
		check(fmt.Sprintf("owners[%d].initial", i), owner.Initial == nil, len(owner.Initial), 8)
	}
	for i, node := range w.Nodes {
		at := fmt.Sprintf("nodes[%d]", i)
		if node.Task != nil {
			check(at+".task.request", node.Task.Request == nil, len(node.Task.Request), 8)
			check(at+".task.response", node.Task.Response == nil, len(node.Task.Response), 8)
			check(at+".task.assign", node.Task.Assign == nil, len(node.Task.Assign), 48)
		}
		if node.Decision != nil {
			check(at+".decision.inputs", node.Decision.Inputs == nil, len(node.Decision.Inputs), 16)
			check(at+".decision.assign", node.Decision.Assign == nil, len(node.Decision.Assign), 48)
		}
		if node.Command != nil {
			check(at+".command.inputs", node.Command.Inputs == nil, len(node.Command.Inputs), 8)
			check(at+".command.assign", node.Command.Assign == nil, len(node.Command.Assign), 48)
		}
		if node.End != nil {
			check(at+".end.outputs", node.End.Outputs == nil, len(node.End.Outputs), 16)
		}
	}
	for i, signal := range w.Signals {
		check(fmt.Sprintf("signals[%d].inputs", i), signal.Inputs == nil, len(signal.Inputs), 8)
		check(fmt.Sprintf("signals[%d].set", i), signal.Set == nil, len(signal.Set), 48)
	}
	return issues
}

// workflowClosure validates independent closures before inspecting or encoding
// them and rejects conflicting identities across every workflow dependency.
func workflowClosure(w WorkflowDefinition) ([]DataContractLock, int, []ValidationIssue) {
	issues := workflowBounds(w)
	if len(issues) > 0 {
		return []DataContractLock{}, 0, issues
	}
	data := map[compositionIdentity]DataContractLock{}
	packages := map[compositionIdentity]string{}
	lowerBytes := 0
	addData := func(lock DataContractLock, path string) {
		key := compositionIdentity{lock.Contract.ID, lock.Contract.Version}
		if old, found := data[key]; found && old.Digest != lock.Digest {
			issues = append(issues, ValidationIssue{path, "Conflicting data contract digests for the same ID/version."})
		} else {
			data[key] = lock
		}
	}
	addBundle := func(bundle ComponentBundle, path string) {
		for i, lock := range bundle.Packages {
			key := packageIdentity(lock.Package)
			if old, found := packages[key]; found && old != lock.Digest {
				issues = append(issues, ValidationIssue{fmt.Sprintf("%s.packages[%d]", path, i), "Conflicting component digests for the same ID/version."})
			} else {
				packages[key] = lock.Digest
			}
			for _, dataLock := range lock.Package.DataContracts {
				addData(dataLock, path)
			}
		}
	}
	capacity := func() bool {
		if len(data) > 32 {
			issues = append(issues, ValidationIssue{"dataContracts", "At most 32 distinct data locks are supported."})
		}
		if len(packages) > 64 {
			issues = append(issues, ValidationIssue{"decisions", "At most 64 distinct component locks are supported."})
		}
		if lowerBytes > MaxWorkflowDefinitionBytes {
			issues = append(issues, ValidationIssue{"", "Workflow definition canonical JSON must not exceed 1 MiB."})
		}
		return len(data) > 32 || len(packages) > 64 || lowerBytes > MaxWorkflowDefinitionBytes
	}
	localData := map[compositionIdentity]bool{}
	for i, lock := range w.DataContracts {
		at := fmt.Sprintf("dataContracts[%d]", i)
		local := ValidateDataContractLock(lock)
		issues = append(issues, prefixBehaviorIssues(at, local)...)
		key := compositionIdentity{lock.Contract.ID, lock.Contract.Version}
		if localData[key] {
			issues = append(issues, ValidationIssue{at, "Local data lock ID/version pairs must be unique."})
		}
		localData[key] = true
		if len(local) == 0 {
			addData(lock, at)
			encoded, _ := json.Marshal(lock)
			lowerBytes += len(encoded)
		}
		if capacity() {
			return []DataContractLock{}, lowerBytes, issues
		}
	}
	roots := map[compositionIdentity]bool{}
	for i, bundle := range w.Decisions {
		at := fmt.Sprintf("decisions[%d]", i)
		local := ValidateComponentBundle(bundle)
		issues = append(issues, prefixBehaviorIssues(at, local)...)
		key := referenceIdentity(bundle.Root)
		if roots[key] {
			issues = append(issues, ValidationIssue{at + ".root", "Decision root ID/version pairs must be unique."})
		}
		roots[key] = true
		if len(local) == 0 {
			addBundle(bundle, at)
			// A valid pure bundle may contain fifty individually bounded
			// packages. Count one lock at a time so workflow admission never
			// materializes an oversized aggregate bundle just to reject it.
			for _, lock := range bundle.Packages {
				encoded, _ := json.Marshal(lock)
				lowerBytes += len(encoded)
				if capacity() {
					return []DataContractLock{}, lowerBytes, issues
				}
			}
		}
		if capacity() {
			return []DataContractLock{}, lowerBytes, issues
		}
	}
	machines := map[compositionIdentity]bool{}
	for i, lock := range w.Machines {
		at := fmt.Sprintf("machines[%d]", i)
		local := ValidateStateMachineLock(lock)
		issues = append(issues, prefixBehaviorIssues(at, local)...)
		key := compositionIdentity{lock.Machine.ID, lock.Machine.Version}
		if machines[key] {
			issues = append(issues, ValidationIssue{at, "Machine ID/version pairs must be unique."})
		}
		machines[key] = true
		if len(local) == 0 {
			for _, dataLock := range lock.Machine.DataContracts {
				addData(dataLock, at)
			}
			for j, command := range lock.Machine.Commands {
				addBundle(command.Decision, fmt.Sprintf("%s.machine.commands[%d].decision", at, j))
			}
			encoded, _ := json.Marshal(lock)
			lowerBytes += len(encoded)
		}
		if capacity() {
			return []DataContractLock{}, lowerBytes, issues
		}
	}
	keys := make([]compositionIdentity, 0, len(data))
	for key := range data {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].id != keys[j].id {
			return keys[i].id < keys[j].id
		}
		return keys[i].version < keys[j].version
	})
	contracts := make([]DataContractLock, 0, len(data))
	for _, key := range keys {
		contracts = append(contracts, data[key])
	}
	return contracts, lowerBytes, issues
}

func WorkflowDataContracts(w WorkflowDefinition) ([]DataContractLock, []ValidationIssue) {
	contracts, _, issues := workflowClosure(w)
	return contracts, issues
}

func workflowContext(w WorkflowDefinition, contracts []DataContractLock) WorkflowContext {
	ctx := WorkflowContext{Contracts: contracts, Inputs: map[string]BehaviorValueType{}, Outputs: map[string]BehaviorValueType{}, Variables: map[string]BehaviorValueType{}, Nodes: map[string]WorkflowNode{}, Owners: map[string]StateMachineLock{}, Decisions: map[ComponentRef]ComponentBundle{}, Signals: map[string]WorkflowSignal{}}
	for _, port := range w.Inputs {
		ctx.Inputs[port.Name] = PortBehaviorType(port)
	}
	for _, port := range w.Outputs {
		ctx.Outputs[port.Name] = PortBehaviorType(port)
	}
	for _, variable := range w.Variables {
		ctx.Variables[variable.Name] = BehaviorValueType{Type: variable.Type, RecordRef: variable.RecordRef}
	}
	for _, node := range w.Nodes {
		ctx.Nodes[node.ID] = node
	}
	for _, signal := range w.Signals {
		ctx.Signals[signal.Name] = signal
	}
	for _, decision := range w.Decisions {
		ctx.Decisions[decision.Root] = decision
	}
	for _, owner := range w.Owners {
		for _, lock := range w.Machines {
			if owner.Machine.MachineID == lock.Machine.ID && owner.Machine.Version == lock.Machine.Version && owner.Machine.Digest == lock.Digest {
				ctx.Owners[owner.ID] = lock
			}
		}
	}
	return ctx
}

func BuildWorkflowContext(w WorkflowDefinition) (WorkflowContext, []ValidationIssue) {
	issues := ValidateWorkflowDefinition(w)
	if len(issues) > 0 {
		return WorkflowContext{}, issues
	}
	contracts, _ := WorkflowDataContracts(w)
	return workflowContext(w, contracts), issues
}

type workflowValidator struct {
	context WorkflowContext
	issues  []ValidationIssue
	bytes   int
}

func (v *workflowValidator) add(path, message string) {
	v.issues = append(v.issues, ValidationIssue{path, message})
}
func (v *workflowValidator) append(path string, issues []ValidationIssue) {
	v.issues = append(v.issues, prefixBehaviorIssues(path, issues)...)
}
func (v *workflowValidator) oversized() bool { return v.bytes > MaxWorkflowDefinitionBytes }
func (v *workflowValidator) count(value any) {
	encoded, _ := json.Marshal(value)
	v.bytes += len(encoded)
	if v.oversized() {
		v.add("", "Workflow definition canonical JSON must not exceed 1 MiB.")
	}
}
func (v *workflowValidator) binding(binding WorkflowBinding, inputs map[string]BehaviorValueType, path string) (BehaviorValueType, bool) {
	count := 0
	if binding.Input != "" {
		count++
	}
	if binding.Variable != "" {
		count++
	}
	if binding.Literal != nil {
		count++
	}
	if count != 1 {
		v.add(path, "A binding requires exactly one input, variable or literal source.")
		return BehaviorValueType{}, false
	}
	if binding.Literal != nil {
		local := ValidateStructuredTypedValue(*binding.Literal, v.context.Contracts)
		v.append(path+".literal", local)
		if len(local) != 0 {
			return BehaviorValueType{}, false
		}
		v.count(*binding.Literal)
		return ValueBehaviorType(*binding.Literal), true
	}
	if binding.Input != "" {
		value, found := inputs[binding.Input]
		if !found {
			v.add(path+".input", "Input source is not declared in this binding context.")
		}
		return value, found
	}
	value, found := v.context.Variables[binding.Variable]
	if !found {
		v.add(path+".variable", "Variable source is not declared.")
	}
	return value, found
}
func (v *workflowValidator) bindings(bindings map[string]WorkflowBinding, expected map[string]BehaviorValueType, inputs map[string]BehaviorValueType, complete bool, path string) {
	if complete {
		for _, name := range sortedBehaviorKeys(expected) {
			if _, found := bindings[name]; !found {
				v.add(path+"."+name, "Every declared destination requires a binding.")
			}
		}
	}
	for _, name := range sortedBehaviorKeys(bindings) {
		if v.oversized() {
			return
		}
		if !behaviorName.MatchString(name) {
			v.add(path+"."+name, "Binding names must be valid local identifiers.")
		}
		typ, valid := v.binding(bindings[name], inputs, path+"."+name)
		if expected != nil {
			dest, found := expected[name]
			if !found {
				v.add(path+"."+name, "Destination is not declared.")
			} else if valid && !SameBehaviorType(typ, dest) {
				v.add(path+"."+name, "Binding must match the destination's exact nominal type.")
			}
		}
	}
}
func (v *workflowValidator) assign(mapping map[string]string, ports map[string]BehaviorValueType, path string) {
	for _, name := range sortedBehaviorKeys(mapping) {
		target, found := v.context.Variables[name]
		if !found {
			v.add(path+"."+name, "Assignment destination must name a declared workflow variable.")
		}
		source, exists := ports[mapping[name]]
		if !exists {
			v.add(path+"."+name, "Assignment source must name a declared output port.")
		} else if found && !SameBehaviorType(target, source) {
			v.add(path+"."+name, "Assignment types must match exactly.")
		}
	}
}
func rootBehavior(bundle ComponentBundle) *Behavior {
	for _, lock := range bundle.Packages {
		if componentRefForWorkflow(lock) == bundle.Root {
			return lock.Package.Behavior
		}
	}
	return nil
}
func componentRefForWorkflow(lock PackageLock) ComponentRef {
	return ComponentRef{PackageID: lock.Package.ID, Version: lock.Package.Version, Digest: lock.Digest}
}

func ValidateWorkflowDefinition(w WorkflowDefinition) []ValidationIssue {
	issues := workflowBounds(w)
	if len(issues) > 0 {
		return issues
	}
	contracts, lowerBytes, closureIssues := workflowClosure(w)
	if len(closureIssues) > 0 {
		return closureIssues
	}
	v := workflowValidator{context: workflowContext(w, contracts), issues: []ValidationIssue{}, bytes: lowerBytes}
	if w.SchemaVersion != 1 {
		v.add("schemaVersion", "Only workflow definition schema version 1 is supported.")
	}
	if !safeID.MatchString(w.ID) || !strings.Contains(w.ID, ".") {
		v.add("id", "Use a namespaced safe identifier containing a dot.")
	}
	if len(w.Version) > 32 || !componentVersion.MatchString(w.Version) {
		v.add("version", "Use an exact three-part decimal version of at most 32 bytes.")
	}
	for _, field := range []struct {
		name, value string
		limit       int
		required    bool
	}{{"name", w.Name, 200, true}, {"description", w.Description, 4000, false}, {"author", w.Author, 200, true}, {"license", w.License, 200, true}} {
		if !behaviorTextValid(field.value, field.limit, field.required) {
			v.add(field.name, fmt.Sprintf("Must be valid text of at most %d bytes.", field.limit))
		} else {
			v.count(field.value)
		}
	}
	_, local := statePorts(w.Inputs, contracts, "inputs")
	v.issues = append(v.issues, local...)
	_, local = statePorts(w.Outputs, contracts, "outputs")
	v.issues = append(v.issues, local...)
	defaults := map[string]TypedValue{}
	seen := map[string]bool{}
	for i, variable := range w.Variables {
		at := fmt.Sprintf("variables[%d]", i)
		if !behaviorName.MatchString(variable.Name) {
			v.add(at+".name", "Variable names must be valid local identifiers.")
		}
		if seen[variable.Name] {
			v.add(at+".name", "Variable names must be unique.")
		}
		seen[variable.Name] = true
		typ := BehaviorValueType{Type: variable.Type, RecordRef: variable.RecordRef}
		v.append(at, validateBehaviorValueType(typ))
		v.append(at+".initial", ValidateStructuredTypedValue(variable.Initial, contracts))
		if !SameBehaviorType(typ, ValueBehaviorType(variable.Initial)) {
			v.add(at+".initial", "Initial value must match the variable's exact nominal type.")
		}
		defaults[variable.Name] = variable.Initial
	}
	v.append("variables", ValidateWorkflowValues(defaults, contracts, 48, MaxWorkflowVariablesBytes))
	if len(v.issues) > 0 {
		return v.issues
	}
	v.count(defaults)
	seen = map[string]bool{}
	for i, owner := range w.Owners {
		at := fmt.Sprintf("owners[%d]", i)
		if !behaviorName.MatchString(owner.ID) {
			v.add(at+".id", "Owner IDs must be valid local identifiers.")
		}
		if seen[owner.ID] {
			v.add(at+".id", "Owner IDs must be unique.")
		}
		seen[owner.ID] = true
		if !behaviorTextValid(owner.Name, 200, true) {
			v.add(at+".name", "Owner name requires valid nonblank text of at most 200 bytes.")
		} else {
			v.count(owner.Name)
		}
		v.append(at+".machine", ValidateComponentReference(ComponentRef{PackageID: owner.Machine.MachineID, Version: owner.Machine.Version, Digest: owner.Machine.Digest}))
		lock, found := v.context.Owners[owner.ID]
		if !found {
			v.add(at+".machine", "Owner must resolve an exact embedded machine lock.")
			continue
		}
		slots := map[string]BehaviorValueType{}
		for _, slot := range lock.Machine.State {
			slots[slot.Name] = stateSlotType(slot)
		}
		for _, name := range sortedBehaviorKeys(owner.Initial) {
			if owner.Initial[name].Variable != "" {
				v.add(at+".initial."+name+".variable", "Owner initial values must use root inputs or literals.")
			}
		}
		v.bindings(owner.Initial, slots, v.context.Inputs, false, at+".initial")
		if v.oversized() {
			return v.issues
		}
	}
	seen = map[string]bool{}
	for i, node := range w.Nodes {
		at := fmt.Sprintf("nodes[%d]", i)
		if !behaviorName.MatchString(node.ID) {
			v.add(at+".id", "Node IDs must be valid local identifiers.")
		}
		if seen[node.ID] {
			v.add(at+".id", "Node IDs must be unique.")
		}
		seen[node.ID] = true
		if !behaviorTextValid(node.Name, 200, true) {
			v.add(at+".name", "Node name requires valid nonblank text of at most 200 bytes.")
		} else {
			v.count(node.Name)
		}
		if node.Stage != "" {
			if !behaviorTextValid(node.Stage, 240, true) || len(strings.Split(node.Stage, "/")) > 6 {
				v.add(at+".stage", "Stage paths require nonblank text, at most 240 bytes and six levels.")
			}
			for _, part := range strings.Split(node.Stage, "/") {
				if strings.TrimSpace(part) == "" {
					v.add(at+".stage", "Stage path segments cannot be empty.")
				}
			}
			v.count(node.Stage)
		}
		if node.Color != "" && !nodeColor.MatchString(node.Color) {
			v.add(at+".color", "Node color must be a six-digit hexadecimal color.")
		}
		if math.IsNaN(node.Position.X) || math.IsInf(node.Position.X, 0) || math.Abs(node.Position.X) > 1e6 || math.IsNaN(node.Position.Y) || math.IsInf(node.Position.Y, 0) || math.Abs(node.Position.Y) > 1e6 {
			v.add(at+".position", "Position must be finite within ±1000000.")
		}
		count := 0
		if node.Task != nil {
			count++
		}
		if node.Decision != nil {
			count++
		}
		if node.Command != nil {
			count++
		}
		if node.End != nil {
			count++
		}
		if count != 1 {
			v.add(at, "Exactly one node definition must match its kind.")
		}
		if node.Kind == "end" {
			if node.Next != "" || node.Branch != nil {
				v.add(at, "End nodes cannot have continuation edges.")
			}
		} else {
			if (node.Next != "") == (node.Branch != nil) {
				v.add(at, "A nonterminal node requires exactly one next edge or Boolean branch.")
			}
			if node.Branch != nil {
				typ, found := v.context.Variables[node.Branch.Variable]
				if !found || typ.Type != "boolean" {
					v.add(at+".branch.variable", "Branch must name a Boolean workflow variable.")
				}
			}
		}
		switch node.Kind {
		case "task":
			if node.Task == nil || count != 1 {
				v.add(at+".task", "Task kind requires only task metadata.")
				break
			}
			task := node.Task
			if task.Kind != "human" && task.Kind != "adapter" {
				v.add(at+".task.kind", "Task kind must be human or adapter.")
			}
			if !behaviorTextValid(task.Title, 200, true) {
				v.add(at+".task.title", "Task title requires valid nonblank text of at most 200 bytes.")
			} else {
				v.count(task.Title)
			}
			if !behaviorTextValid(task.Instructions, 4000, false) {
				v.add(at+".task.instructions", "Instructions must be valid text of at most 4000 bytes.")
			} else {
				v.count(task.Instructions)
			}
			ports, local := statePorts(task.Response, contracts, at+".task.response")
			v.issues = append(v.issues, local...)
			v.bindings(task.Request, nil, v.context.Inputs, false, at+".task.request")
			v.assign(task.Assign, ports, at+".task.assign")
		case "decision":
			if node.Decision == nil || count != 1 {
				v.add(at+".decision", "Decision kind requires only decision metadata.")
				break
			}
			v.append(at+".decision.component", ValidateComponentReference(node.Decision.Component))
			decision, found := v.context.Decisions[node.Decision.Component]
			if !found {
				v.add(at+".decision.component", "Decision must name an exact embedded bundle root.")
				break
			}
			body := rootBehavior(decision)
			inputs, _ := statePorts(body.Inputs, contracts, "")
			outputs, _ := statePorts(body.Outputs, contracts, "")
			v.bindings(node.Decision.Inputs, inputs, v.context.Inputs, true, at+".decision.inputs")
			v.assign(node.Decision.Assign, outputs, at+".decision.assign")
		case "command":
			if node.Command == nil || count != 1 {
				v.add(at+".command", "Command kind requires only command metadata.")
				break
			}
			command := node.Command
			owner, found := v.context.Owners[command.Owner]
			if !found {
				v.add(at+".command.owner", "Command must name a declared resolved owner.")
				break
			}
			var handler *StateCommand
			for j := range owner.Machine.Commands {
				if owner.Machine.Commands[j].Name == command.Command {
					handler = &owner.Machine.Commands[j]
				}
			}
			if handler == nil {
				v.add(at+".command.command", "Command must name a declared owned command.")
				break
			}
			inputs, _ := statePorts(handler.Inputs, contracts, "")
			outputs, _ := statePorts(handler.Outputs, contracts, "")
			v.bindings(command.Inputs, inputs, v.context.Inputs, true, at+".command.inputs")
			v.assign(command.Assign, outputs, at+".command.assign")
			accepted, found := v.context.Variables[command.AcceptedVariable]
			if !found || accepted.Type != "boolean" {
				v.add(at+".command.acceptedVariable", "Accepted destination must name a Boolean workflow variable.")
			}
			if _, found := command.Assign[command.AcceptedVariable]; found {
				v.add(at+".command.assign", "Accepted variable cannot also receive an output assignment.")
			}
			if command.ErrorNext == "" {
				v.add(at+".command.errorNext", "Command requires an explicit error continuation.")
			}
		case "end":
			if node.End == nil || count != 1 {
				v.add(at+".end", "End kind requires only end metadata.")
				break
			}
			if !behaviorName.MatchString(node.End.Outcome) {
				v.add(at+".end.outcome", "Outcome must be a valid local identifier.")
			}
			v.bindings(node.End.Outputs, v.context.Outputs, v.context.Inputs, true, at+".end.outputs")
		default:
			v.add(at+".kind", "Node kind must be task, decision, command or end.")
		}
		if v.oversized() {
			return v.issues
		}
	}
	seen = map[string]bool{}
	for i, signal := range w.Signals {
		at := fmt.Sprintf("signals[%d]", i)
		if !behaviorName.MatchString(signal.Name) {
			v.add(at+".name", "Signal names must be valid local identifiers.")
		}
		if seen[signal.Name] {
			v.add(at+".name", "Signal names must be unique.")
		}
		seen[signal.Name] = true
		inputs, local := statePorts(signal.Inputs, contracts, at+".inputs")
		v.issues = append(v.issues, local...)
		if signal.When != "" {
			typ, found := v.context.Variables[signal.When]
			if !found || typ.Type != "boolean" {
				v.add(at+".when", "Signal condition must name a Boolean workflow variable.")
			}
		}
		v.bindings(signal.Set, v.context.Variables, inputs, false, at+".set")
		if v.oversized() {
			return v.issues
		}
	}
	v.validateGraph(w)
	if len(v.issues) == 0 {
		encoded, err := json.Marshal(w)
		if err != nil || len(encoded) > MaxWorkflowDefinitionBytes {
			v.add("", "Workflow definition canonical JSON must not exceed 1 MiB.")
		}
	}
	return v.issues
}

func (v *workflowValidator) validateGraph(w WorkflowDefinition) {
	edges := map[string][]string{}
	check := func(target, path string) {
		if _, found := v.context.Nodes[target]; !found {
			v.add(path, "Edge must name a declared workflow node.")
		}
	}
	check(w.Entry, "entry")
	for i, node := range w.Nodes {
		at := fmt.Sprintf("nodes[%d]", i)
		if node.Next != "" {
			check(node.Next, at+".next")
			edges[node.ID] = append(edges[node.ID], node.Next)
		}
		if node.Branch != nil {
			check(node.Branch.Then, at+".branch.then")
			check(node.Branch.Else, at+".branch.else")
			edges[node.ID] = append(edges[node.ID], node.Branch.Then, node.Branch.Else)
		}
		if node.Command != nil {
			check(node.Command.ErrorNext, at+".command.errorNext")
			edges[node.ID] = append(edges[node.ID], node.Command.ErrorNext)
		}
	}
	for i, signal := range w.Signals {
		check(signal.Target, fmt.Sprintf("signals[%d].target", i))
	}
	// Signals are explicit interruption points while a task is waiting. A task
	// loop can therefore terminate through a declared interrupt even when its
	// ordinary continuation loops back to itself.
	for _, node := range w.Nodes {
		if node.Kind == "task" {
			for _, signal := range w.Signals {
				edges[node.ID] = append(edges[node.ID], signal.Target)
			}
		}
	}
	if len(v.issues) > 0 {
		return
	}
	seen := map[string]bool{}
	var visit func(string)
	visit = func(id string) {
		if seen[id] {
			return
		}
		seen[id] = true
		for _, next := range edges[id] {
			visit(next)
		}
	}
	visit(w.Entry)
	for _, signal := range w.Signals {
		visit(signal.Target)
	}
	for i, node := range w.Nodes {
		if !seen[node.ID] {
			v.add(fmt.Sprintf("nodes[%d]", i), "Every node must be reachable from entry or a declared signal target.")
		}
	}
	// Cycles are bounded by the runtime, but every node must have a structural
	// route to an end; an accidental closed loop cannot claim a completion path.
	reverse := map[string][]string{}
	queue := []string{}
	ends := map[string]bool{}
	for _, node := range w.Nodes {
		if node.Kind == "end" {
			queue = append(queue, node.ID)
			ends[node.ID] = true
		}
		for _, next := range edges[node.ID] {
			reverse[next] = append(reverse[next], node.ID)
		}
	}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, previous := range reverse[current] {
			if !ends[previous] {
				ends[previous] = true
				queue = append(queue, previous)
			}
		}
	}
	for i, node := range w.Nodes {
		if !ends[node.ID] {
			v.add(fmt.Sprintf("nodes[%d]", i), "Every node must have a structural path to an end.")
		}
	}
}

func WorkflowDigest(w WorkflowDefinition) (string, error) {
	if issues := ValidateWorkflowDefinition(w); len(issues) > 0 {
		return "", fmt.Errorf("invalid workflow at %s: %s", issues[0].Path, issues[0].Message)
	}
	encoded, err := json.Marshal(w)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}
func ValidateWorkflowLock(lock WorkflowLock) []ValidationIssue {
	issues := prefixBehaviorIssues("workflow", ValidateWorkflowDefinition(lock.Workflow))
	if !componentDigest.MatchString(lock.Digest) {
		issues = append(issues, ValidationIssue{"digest", "A lock requires a lowercase SHA-256 digest."})
	} else if len(issues) == 0 {
		encoded, err := json.Marshal(lock.Workflow)
		if err != nil {
			return append(issues, ValidationIssue{"digest", "Workflow could not be encoded."})
		}
		sum := sha256.Sum256(encoded)
		if hex.EncodeToString(sum[:]) != lock.Digest {
			issues = append(issues, ValidationIssue{"digest", "Digest does not match the exact embedded workflow."})
		}
	}
	return issues
}
