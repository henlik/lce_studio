package model

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const MaxStateValuesBytes = 64 * 1024
const MaxStateMachineBytes = 512 * 1024
const MaxStateScenarioBytes = 512 * 1024

func stateSlotType(slot StateSlot) BehaviorValueType {
	return BehaviorValueType{Type: slot.Type, RecordRef: slot.RecordRef}
}

// ValidateStateValues bounds shape and individual values before encoding the
// complete state/public-input/public-result/effect-payload map.
func ValidateStateValues(values map[string]TypedValue, contracts []DataContractLock) []ValidationIssue {
	issues := []ValidationIssue{}
	if values == nil || len(values) > 8 {
		return []ValidationIssue{{"", "A nonnull map of at most 8 state values is required."}}
	}
	bytes := 2
	for _, name := range sortedBehaviorKeys(values) {
		if !behaviorName.MatchString(name) {
			issues = append(issues, ValidationIssue{name, "Value names must be valid local identifiers."})
		}
		local := ValidateStructuredTypedValue(values[name], contracts)
		issues = append(issues, prefixBehaviorIssues(name, local)...)
		if len(local) == 0 {
			encoded, _ := json.Marshal(values[name])
			bytes += len(encoded)
			if bytes > MaxStateValuesBytes {
				return append(issues, ValidationIssue{"", "State value maps must not exceed 64 KiB of canonical JSON."})
			}
		}
	}
	if len(issues) == 0 {
		encoded, err := json.Marshal(values)
		if err != nil || len(encoded) > MaxStateValuesBytes {
			issues = append(issues, ValidationIssue{"", "State value maps must not exceed 64 KiB of canonical JSON."})
		}
	}
	return issues
}

func stateMachineBounds(m StateMachine) []ValidationIssue {
	issues := []ValidationIssue{}
	if m.DataContracts == nil || len(m.DataContracts) > 8 {
		issues = append(issues, ValidationIssue{"dataContracts", "A nonnull array of at most 8 local data locks is required."})
	}
	if m.State == nil || len(m.State) < 1 || len(m.State) > 8 {
		issues = append(issues, ValidationIssue{"state", "State requires 1–8 owned slots."})
	}
	if m.Effects == nil || len(m.Effects) > 8 {
		issues = append(issues, ValidationIssue{"effects", "Effects must be a nonnull array of at most 8 declarations."})
	}
	if m.Commands == nil || len(m.Commands) < 1 || len(m.Commands) > 8 {
		issues = append(issues, ValidationIssue{"commands", "Commands require 1–8 definitions."})
	}
	if len(issues) != 0 {
		return issues
	}
	for i, effect := range m.Effects {
		if effect.Payload == nil || len(effect.Payload) > 8 {
			issues = append(issues, ValidationIssue{fmt.Sprintf("effects[%d].payload", i), "Effect payload must be a nonnull array of at most 8 ports."})
		}
	}
	for i, c := range m.Commands {
		at := fmt.Sprintf("commands[%d]", i)
		for _, x := range []struct {
			name          string
			missing       bool
			length, limit int
		}{
			{"inputs", c.Inputs == nil, len(c.Inputs), 8}, {"outputs", c.Outputs == nil, len(c.Outputs), 8}, {"bindings", c.Bindings == nil, len(c.Bindings), 16},
			{"writes", c.Writes == nil, len(c.Writes), 8}, {"results", c.Results == nil, len(c.Results), 8}, {"effects", c.Effects == nil, len(c.Effects), 8},
		} {
			if x.missing || x.length > x.limit {
				issues = append(issues, ValidationIssue{at + "." + x.name, fmt.Sprintf("A nonnull collection of at most %d entries is required.", x.limit)})
			}
		}
		if len(c.Effects) <= 8 {
			for j, e := range c.Effects {
				if e.Payload == nil || len(e.Payload) > 8 {
					issues = append(issues, ValidationIssue{fmt.Sprintf("%s.effects[%d].payload", at, j), "Effect payload mapping requires a nonnull object of at most 8 entries."})
				}
			}
		}
	}
	return issues
}

// StateMachineDataContracts collects exact locks without consulting catalogs.
// It also rejects conflicting pure package identities across command closures.
func StateMachineDataContracts(m StateMachine) ([]DataContractLock, []ValidationIssue) {
	issues := stateMachineBounds(m)
	if len(issues) != 0 {
		return []DataContractLock{}, issues
	}
	data := map[compositionIdentity]DataContractLock{}
	packages := map[compositionIdentity]string{}
	add := func(lock DataContractLock, path string) {
		issues = append(issues, prefixBehaviorIssues(path, ValidateDataContractLock(lock))...)
		key := compositionIdentity{lock.Contract.ID, lock.Contract.Version}
		if existing, ok := data[key]; ok && existing.Digest != lock.Digest {
			issues = append(issues, ValidationIssue{path, "Conflicting data contract digests for the same ID/version."})
		} else {
			data[key] = lock
		}
	}
	local := map[compositionIdentity]bool{}
	for i, lock := range m.DataContracts {
		at := fmt.Sprintf("dataContracts[%d]", i)
		key := compositionIdentity{lock.Contract.ID, lock.Contract.Version}
		if local[key] {
			issues = append(issues, ValidationIssue{at, "Local data contract ID/version pairs must be unique."})
		}
		local[key] = true
		add(lock, at)
	}
	for i, c := range m.Commands {
		if len(c.Decision.Packages) > 50 {
			issues = append(issues, ValidationIssue{fmt.Sprintf("commands[%d].decision.packages", i), "At most 50 packages are supported per decision bundle."})
			continue
		}
		for j, p := range c.Decision.Packages {
			at := fmt.Sprintf("commands[%d].decision.packages[%d]", i, j)
			key := packageIdentity(p.Package)
			if previous, ok := packages[key]; ok && previous != p.Digest {
				issues = append(issues, ValidationIssue{at, "Conflicting component digests for the same ID/version across decisions."})
			} else {
				packages[key] = p.Digest
			}
			if len(packages) > 64 {
				return []DataContractLock{}, append(issues, ValidationIssue{"commands", "At most 64 distinct component locks are supported across the machine."})
			}
			if len(p.Package.DataContracts) > 8 {
				issues = append(issues, ValidationIssue{at + ".package.dataContracts", "At most 8 data locks are supported per component."})
				continue
			}
			for k, lock := range p.Package.DataContracts {
				add(lock, fmt.Sprintf("%s.package.dataContracts[%d]", at, k))
				if len(data) > 32 {
					return []DataContractLock{}, append(issues, ValidationIssue{"dataContracts", "At most 32 distinct data contracts are supported across the machine."})
				}
			}
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
	locks := make([]DataContractLock, 0, len(keys))
	for _, key := range keys {
		locks = append(locks, data[key])
	}
	return locks, issues
}

func statePorts(ports []BehaviorPort, contracts []DataContractLock, path string) (map[string]BehaviorValueType, []ValidationIssue) {
	types := map[string]BehaviorValueType{}
	issues := []ValidationIssue{}
	for i, port := range ports {
		at := fmt.Sprintf("%s[%d]", path, i)
		if !behaviorName.MatchString(port.Name) {
			issues = append(issues, ValidationIssue{at + ".name", "Port names must be valid local identifiers."})
		}
		if _, ok := types[port.Name]; ok {
			issues = append(issues, ValidationIssue{at + ".name", "Port names must be unique within their direction."})
		}
		typ := PortBehaviorType(port)
		types[port.Name] = typ
		local := validateBehaviorValueType(typ)
		issues = append(issues, prefixBehaviorIssues(at, local)...)
		if len(local) == 0 && typ.RecordRef != nil {
			if _, err := ResolveDataRecord(*typ.RecordRef, contracts); err != nil {
				issues = append(issues, ValidationIssue{at + ".recordRef", err.Error()})
			}
		}
	}
	return types, issues
}

func ValidateStateMachine(m StateMachine) []ValidationIssue {
	issues := stateMachineBounds(m)
	if len(issues) != 0 {
		return issues
	}
	add := func(path, message string) { issues = append(issues, ValidationIssue{path, message}) }
	if m.SchemaVersion != 1 {
		add("schemaVersion", "Only state machine schema version 1 is supported.")
	}
	if !safeID.MatchString(m.ID) || !strings.Contains(m.ID, ".") {
		add("id", "Use a namespaced safe identifier containing a dot.")
	}
	if len(m.Version) > 32 || !componentVersion.MatchString(m.Version) {
		add("version", "Use an exact three-part decimal version of at most 32 bytes.")
	}
	for _, f := range []struct {
		name, value string
		limit       int
		required    bool
	}{{"name", m.Name, 200, true}, {"description", m.Description, 4000, false}, {"author", m.Author, 200, true}, {"license", m.License, 200, true}} {
		if !behaviorTextValid(f.value, f.limit, f.required) {
			add(f.name, fmt.Sprintf("Must be valid text of at most %d bytes.", f.limit))
		}
	}
	for i, c := range m.Commands {
		issues = append(issues, prefixBehaviorIssues(fmt.Sprintf("commands[%d].decision", i), ValidateComponentBundle(c.Decision))...)
	}
	if len(issues) != 0 {
		return issues
	}
	contracts, local := StateMachineDataContracts(m)
	issues = append(issues, local...)
	if len(issues) != 0 {
		return issues
	}
	slots := map[string]BehaviorValueType{}
	defaults := map[string]TypedValue{}
	for i, slot := range m.State {
		at := fmt.Sprintf("state[%d]", i)
		if !behaviorName.MatchString(slot.Name) {
			add(at+".name", "Slot names must be valid local identifiers.")
		}
		if _, ok := slots[slot.Name]; ok {
			add(at+".name", "Owned slot names must be unique.")
		}
		slots[slot.Name] = stateSlotType(slot)
		defaults[slot.Name] = slot.Initial
		issues = append(issues, prefixBehaviorIssues(at, validateBehaviorValueType(stateSlotType(slot)))...)
		issues = append(issues, prefixBehaviorIssues(at+".initial", ValidateStructuredTypedValue(slot.Initial, contracts))...)
		if !SameBehaviorType(stateSlotType(slot), ValueBehaviorType(slot.Initial)) {
			add(at+".initial", "Initial value must match the slot's exact nominal type.")
		}
		if slot.Durability != "durable" && slot.Durability != "volatile" {
			add(at+".durability", "Durability must be durable or volatile.")
		}
	}
	issues = append(issues, prefixBehaviorIssues("state", ValidateStateValues(defaults, contracts))...)
	effects := map[string]map[string]BehaviorValueType{}
	for i, effect := range m.Effects {
		at := fmt.Sprintf("effects[%d]", i)
		if !behaviorName.MatchString(effect.Name) {
			add(at+".name", "Effect names must be valid local identifiers.")
		}
		if _, ok := effects[effect.Name]; ok {
			add(at+".name", "Effect names must be unique.")
		}
		types, local := statePorts(effect.Payload, contracts, at+".payload")
		issues = append(issues, local...)
		effects[effect.Name] = types
	}
	commands := map[string]bool{}
	for i, c := range m.Commands {
		at := fmt.Sprintf("commands[%d]", i)
		if !behaviorName.MatchString(c.Name) {
			add(at+".name", "Command names must be valid local identifiers.")
		}
		if commands[c.Name] {
			add(at+".name", "Command names must be unique.")
		}
		commands[c.Name] = true
		inputs, local := statePorts(c.Inputs, contracts, at+".inputs")
		issues = append(issues, local...)
		outputs, local := statePorts(c.Outputs, contracts, at+".outputs")
		issues = append(issues, local...)
		var decision *Behavior
		for _, lock := range c.Decision.Packages {
			if lock.Package.ID == c.Decision.Root.PackageID && lock.Package.Version == c.Decision.Root.Version && lock.Digest == c.Decision.Root.Digest {
				decision = lock.Package.Behavior
				break
			}
		}
		if decision == nil {
			add(at+".decision.root", "Decision root must resolve to an embedded pure component.")
			continue
		}
		rootInputs := map[string]BehaviorValueType{}
		rootOutputs := map[string]BehaviorValueType{}
		for _, p := range decision.Inputs {
			rootInputs[p.Name] = PortBehaviorType(p)
		}
		for _, p := range decision.Outputs {
			rootOutputs[p.Name] = PortBehaviorType(p)
		}
		for _, name := range sortedBehaviorKeys(rootInputs) {
			if _, ok := c.Bindings[name]; !ok {
				add(at+".bindings."+name, "Every decision input requires one binding.")
			}
		}
		for _, name := range sortedBehaviorKeys(c.Bindings) {
			binding := c.Bindings[name]
			path := at + ".bindings." + name
			count := 0
			var typ BehaviorValueType
			found := true
			if binding.State != "" {
				count++
				typ, found = slots[binding.State]
				if !found {
					add(path+".state", "Source must name an owned slot.")
				}
			}
			if binding.Input != "" {
				count++
				typ, found = inputs[binding.Input]
				if !found {
					add(path+".input", "Source must name a declared command input.")
				}
			}
			if binding.Literal != nil {
				count++
				typ = ValueBehaviorType(*binding.Literal)
				issues = append(issues, prefixBehaviorIssues(path+".literal", ValidateStructuredTypedValue(*binding.Literal, contracts))...)
			}
			if count != 1 {
				add(path, "Specify exactly one owned state slot, command input, or typed literal.")
			}
			want, ok := rootInputs[name]
			if !ok {
				add(path, "Binding must name a declared decision input.")
			} else if count == 1 && found && !SameBehaviorType(typ, want) {
				add(path, "Source must exactly match the decision input's nominal type.")
			}
		}
		booleanOutput := func(name, path string) {
			if typ, ok := rootOutputs[name]; !ok || !SameBehaviorType(typ, BehaviorValueType{Type: "boolean"}) {
				add(path, "Must name a Boolean decision output.")
			}
		}
		booleanOutput(c.AcceptedOutput, at+".acceptedOutput")
		mapping := func(bindings map[string]string, targets map[string]BehaviorValueType, path string, complete bool) {
			if complete {
				for _, name := range sortedBehaviorKeys(targets) {
					if _, ok := bindings[name]; !ok {
						add(path+"."+name, "Every destination requires one decision-output mapping.")
					}
				}
			}
			for _, name := range sortedBehaviorKeys(bindings) {
				want, ok := targets[name]
				source, exists := rootOutputs[bindings[name]]
				if !ok {
					add(path+"."+name, "Mapping target is not declared.")
				} else if !exists {
					add(path+"."+name, "Source must name a decision output.")
				} else if !SameBehaviorType(want, source) {
					add(path+"."+name, "Decision output must exactly match the destination's nominal type.")
				}
			}
		}
		mapping(c.Writes, slots, at+".writes", false)
		mapping(c.Results, outputs, at+".results", true)
		seen := map[string]bool{}
		for j, effect := range c.Effects {
			path := fmt.Sprintf("%s.effects[%d]", at, j)
			ports, ok := effects[effect.Effect]
			if !ok {
				add(path+".effect", "Effect must name a declaration owned by this machine.")
			}
			if seen[effect.Effect] {
				add(path+".effect", "Each effect may be emitted at most once per command.")
			}
			seen[effect.Effect] = true
			if effect.WhenOutput != "" {
				booleanOutput(effect.WhenOutput, path+".whenOutput")
			}
			mapping(effect.Payload, ports, path+".payload", true)
		}
	}
	if len(issues) == 0 {
		issues = append(issues, stateMachineSize(m)...)
	}
	return issues
}

// Bound all potentially large children before serializing the enclosing machine.
func stateMachineSize(m StateMachine) []ValidationIssue {
	lower := 0
	add := func(value any) bool {
		encoded, err := json.Marshal(value)
		if err != nil {
			return false
		}
		lower += len(encoded)
		return lower <= MaxStateMachineBytes
	}
	for _, lock := range m.DataContracts {
		if !add(lock) {
			return []ValidationIssue{{"", "Machine canonical JSON must not exceed 512 KiB."}}
		}
	}
	for _, slot := range m.State {
		if !add(slot.Initial) {
			return []ValidationIssue{{"", "Machine canonical JSON must not exceed 512 KiB."}}
		}
	}
	for _, c := range m.Commands {
		for _, lock := range c.Decision.Packages {
			if !add(lock) {
				return []ValidationIssue{{"", "Machine canonical JSON must not exceed 512 KiB."}}
			}
		}
		for _, name := range sortedBehaviorKeys(c.Bindings) {
			if v := c.Bindings[name].Literal; v != nil {
				if !add(v) {
					return []ValidationIssue{{"", "Machine canonical JSON must not exceed 512 KiB."}}
				}
			}
		}
	}
	encoded, err := json.Marshal(m)
	if err != nil || len(encoded) > MaxStateMachineBytes {
		return []ValidationIssue{{"", "Machine canonical JSON must not exceed 512 KiB."}}
	}
	return []ValidationIssue{}
}

func StateMachineDigest(m StateMachine) (string, error) {
	if issues := ValidateStateMachine(m); len(issues) != 0 {
		return "", fmt.Errorf("invalid state machine at %s: %s", issues[0].Path, issues[0].Message)
	}
	encoded, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}
func ValidateStateMachineLock(lock StateMachineLock) []ValidationIssue {
	issues := prefixBehaviorIssues("machine", ValidateStateMachine(lock.Machine))
	if !componentDigest.MatchString(lock.Digest) {
		issues = append(issues, ValidationIssue{"digest", "A machine lock requires a lowercase SHA-256 digest."})
	} else if len(issues) == 0 {
		encoded, err := json.Marshal(lock.Machine)
		digest := sha256.Sum256(encoded)
		if err != nil || hex.EncodeToString(digest[:]) != lock.Digest {
			issues = append(issues, ValidationIssue{"digest", "Digest does not match the exact embedded machine."})
		}
	}
	return issues
}
