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

const MaxDeliveryModelBytes = 1024 * 1024
const MaxDeliveryScenarioBytes = 512 * 1024

func deliveryModelBounds(m DeliveryModel) []ValidationIssue {
	issues := []ValidationIssue{}
	if m.Machines == nil || len(m.Machines) > 4 {
		issues = append(issues, ValidationIssue{"machines", "Machines must be a nonnull array of at most 4 exact locks."})
	}
	if m.Instances == nil || len(m.Instances) < 1 || len(m.Instances) > 8 {
		issues = append(issues, ValidationIssue{"instances", "A delivery model requires 1–8 instances."})
	}
	if m.Routes == nil || len(m.Routes) > 16 {
		issues = append(issues, ValidationIssue{"routes", "Routes must be a nonnull array of at most 16 definitions."})
	}
	if len(issues) != 0 {
		return issues
	}
	for i, route := range m.Routes {
		if route.Inputs == nil || len(route.Inputs) > 8 {
			issues = append(issues, ValidationIssue{fmt.Sprintf("routes[%d].inputs", i), "Route inputs require a nonnull map of at most 8 mappings."})
		}
	}
	return issues
}

// DeliveryModelDataContracts forms a deterministic exact union and checks both
// component and data identities globally. It never repairs a machine closure.
func DeliveryModelDataContracts(m DeliveryModel) ([]DataContractLock, []ValidationIssue) {
	issues := deliveryModelBounds(m)
	if len(issues) != 0 {
		return []DataContractLock{}, issues
	}
	data := map[compositionIdentity]DataContractLock{}
	components := map[compositionIdentity]string{}
	for i, lock := range m.Machines {
		at := fmt.Sprintf("machines[%d].machine", i)
		contracts, local := StateMachineDataContracts(lock.Machine)
		issues = append(issues, prefixBehaviorIssues(at, local)...)
		if len(local) != 0 {
			continue
		}
		for _, contract := range contracts {
			key := compositionIdentity{contract.Contract.ID, contract.Contract.Version}
			if previous, ok := data[key]; ok && previous.Digest != contract.Digest {
				issues = append(issues, ValidationIssue{at, "Conflicting data contract digests for the same ID/version across machines."})
			} else {
				data[key] = contract
			}
			if len(data) > 32 {
				return []DataContractLock{}, append(issues, ValidationIssue{"machines", "At most 32 distinct data contracts are supported across the delivery model."})
			}
		}
		for j, command := range lock.Machine.Commands {
			for k, p := range command.Decision.Packages {
				key := packageIdentity(p.Package)
				if previous, ok := components[key]; ok && previous != p.Digest {
					issues = append(issues, ValidationIssue{fmt.Sprintf("%s.commands[%d].decision.packages[%d]", at, j, k), "Conflicting component digests for the same ID/version across machines."})
				} else {
					components[key] = p.Digest
				}
				if len(components) > 64 {
					return []DataContractLock{}, append(issues, ValidationIssue{"machines", "At most 64 distinct component locks are supported across the delivery model."})
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
	contracts := make([]DataContractLock, 0, len(keys))
	for _, key := range keys {
		contracts = append(contracts, data[key])
	}
	return contracts, issues
}

func ValidateDeliveryModel(m DeliveryModel) []ValidationIssue {
	issues := deliveryModelBounds(m)
	if len(issues) != 0 {
		return issues
	}
	add := func(path, message string) { issues = append(issues, ValidationIssue{path, message}) }
	if m.SchemaVersion != 1 {
		add("schemaVersion", "Only delivery model schema version 1 is supported.")
	}
	if !safeID.MatchString(m.ID) || !strings.Contains(m.ID, ".") {
		add("id", "Use a namespaced safe identifier containing a dot.")
	}
	if len(m.Version) > 32 || !componentVersion.MatchString(m.Version) {
		add("version", "Use an exact three-part decimal version of at most 32 bytes.")
	}
	for _, field := range []struct {
		name, value string
		limit       int
		required    bool
	}{{"name", m.Name, 200, true}, {"description", m.Description, 4000, false}, {"author", m.Author, 200, true}, {"license", m.License, 200, true}} {
		if !behaviorTextValid(field.value, field.limit, field.required) {
			add(field.name, fmt.Sprintf("Must be valid text of at most %d bytes.", field.limit))
		}
	}
	machines := map[compositionIdentity]StateMachineLock{}
	lowerBytes := 0
	for i, lock := range m.Machines {
		at := fmt.Sprintf("machines[%d]", i)
		local := ValidateStateMachineLock(lock)
		issues = append(issues, prefixBehaviorIssues(at, local)...)
		key := compositionIdentity{lock.Machine.ID, lock.Machine.Version}
		if _, ok := machines[key]; ok {
			add(at, "Embedded machine ID/version pairs must be unique.")
		}
		machines[key] = lock
		if len(local) == 0 {
			encoded, _ := json.Marshal(lock)
			lowerBytes += len(encoded)
			if lowerBytes > MaxDeliveryModelBytes {
				add("", "Delivery model canonical JSON must not exceed 1 MiB.")
				return issues
			}
		}
	}
	if len(issues) != 0 {
		return issues
	}
	_, unionIssues := DeliveryModelDataContracts(m)
	issues = append(issues, unionIssues...)
	if len(issues) != 0 {
		return issues
	}
	instances := map[string]StateMachine{}
	for i, instance := range m.Instances {
		at := fmt.Sprintf("instances[%d]", i)
		if !behaviorName.MatchString(instance.ID) {
			add(at+".id", "Instance IDs must be valid local identifiers.")
		}
		if _, ok := instances[instance.ID]; ok {
			add(at+".id", "Instance IDs must be unique.")
		}
		if !behaviorTextValid(instance.Name, 200, true) {
			add(at+".name", "Instance name requires nonblank valid text of at most 200 bytes.")
		}
		for _, coordinate := range []struct {
			name  string
			value float64
		}{{"x", instance.Position.X}, {"y", instance.Position.Y}} {
			if math.IsNaN(coordinate.value) || math.IsInf(coordinate.value, 0) || math.Abs(coordinate.value) > 1e6 {
				add(at+".position."+coordinate.name, "Position must be finite within -1000000..1000000.")
			}
		}
		ref := instance.Machine
		if !safeID.MatchString(ref.MachineID) || !strings.Contains(ref.MachineID, ".") {
			add(at+".machine.machineId", "Machine reference requires a namespaced safe ID.")
		}
		if len(ref.Version) > 32 || !componentVersion.MatchString(ref.Version) {
			add(at+".machine.version", "Machine reference requires an exact three-part decimal version.")
		}
		if !componentDigest.MatchString(ref.Digest) {
			add(at+".machine.digest", "Machine reference requires a lowercase SHA-256 digest.")
		}
		lock, ok := machines[compositionIdentity{ref.MachineID, ref.Version}]
		if !ok {
			add(at+".machine", "Instance machine must resolve to an exact embedded lock.")
		} else if lock.Digest != ref.Digest {
			add(at+".machine.digest", "Machine reference digest must match the exact embedded lock.")
		}
		instances[instance.ID] = lock.Machine
	}
	if len(issues) != 0 {
		return issues
	}
	routes := map[string]bool{}
	tuples := map[[4]string]bool{}
	for i, route := range m.Routes {
		at := fmt.Sprintf("routes[%d]", i)
		if !behaviorName.MatchString(route.ID) {
			add(at+".id", "Route IDs must be valid local identifiers.")
		}
		if routes[route.ID] {
			add(at+".id", "Route IDs must be unique.")
		}
		routes[route.ID] = true
		if !behaviorTextValid(route.Name, 200, true) {
			add(at+".name", "Route name requires nonblank valid text of at most 200 bytes.")
		}
		tuple := [4]string{route.SourceInstanceID, route.Effect, route.TargetInstanceID, route.Command}
		if tuples[tuple] {
			add(at, "Source/effect/target/command route tuples must be unique.")
		}
		tuples[tuple] = true
		source, sourceExists := instances[route.SourceInstanceID]
		target, targetExists := instances[route.TargetInstanceID]
		if !sourceExists {
			add(at+".sourceInstanceId", "Source must name a declared instance.")
		}
		if !targetExists {
			add(at+".targetInstanceId", "Target must name a declared instance.")
		}
		payload := map[string]BehaviorValueType{}
		effectExists := false
		for _, effect := range source.Effects {
			if effect.Name == route.Effect {
				effectExists = true
				for _, port := range effect.Payload {
					payload[port.Name] = PortBehaviorType(port)
				}
				break
			}
		}
		if !effectExists {
			add(at+".effect", "Route effect must be declared by its source machine.")
		}
		inputs := map[string]BehaviorValueType{}
		commandExists := false
		for _, command := range target.Commands {
			if command.Name == route.Command {
				commandExists = true
				for _, port := range command.Inputs {
					inputs[port.Name] = PortBehaviorType(port)
				}
				break
			}
		}
		if !commandExists {
			add(at+".command", "Route command must be declared by its target machine.")
		}
		for _, name := range sortedBehaviorKeys(inputs) {
			if _, ok := route.Inputs[name]; !ok {
				add(at+".inputs."+name, "Every recipient input requires one source payload mapping.")
			}
		}
		for _, name := range sortedBehaviorKeys(route.Inputs) {
			want, ok := inputs[name]
			actual, exists := payload[route.Inputs[name]]
			if !ok {
				add(at+".inputs."+name, "Mapping target must name a recipient input.")
			} else if !exists {
				add(at+".inputs."+name, "Mapping source must name an effect payload port.")
			} else if !SameBehaviorType(actual, want) {
				add(at+".inputs."+name, "Source payload and recipient input must have the exact same nominal type.")
			}
		}
	}
	if len(issues) == 0 {
		encoded, err := json.Marshal(m)
		if err != nil || len(encoded) > MaxDeliveryModelBytes {
			add("", "Delivery model canonical JSON must not exceed 1 MiB.")
		}
	}
	return issues
}

func DeliveryModelDigest(m DeliveryModel) (string, error) {
	if issues := ValidateDeliveryModel(m); len(issues) != 0 {
		return "", fmt.Errorf("invalid delivery model at %s: %s", issues[0].Path, issues[0].Message)
	}
	encoded, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}
func ValidateDeliveryModelLock(lock DeliveryModelLock) []ValidationIssue {
	issues := prefixBehaviorIssues("model", ValidateDeliveryModel(lock.Model))
	if !componentDigest.MatchString(lock.Digest) {
		issues = append(issues, ValidationIssue{"digest", "A delivery lock requires a lowercase SHA-256 digest."})
	} else if len(issues) == 0 {
		encoded, err := json.Marshal(lock.Model)
		digest := sha256.Sum256(encoded)
		if err != nil || hex.EncodeToString(digest[:]) != lock.Digest {
			issues = append(issues, ValidationIssue{"digest", "Digest does not match the exact embedded delivery model."})
		}
	}
	return issues
}
