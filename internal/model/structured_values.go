package model

import (
	"encoding/json"
	"fmt"
	"sort"
)

const (
	MaxCollectionItems       = 128
	MaxStructuredValueBytes  = 64 * 1024
	MaxStructuredValuesBytes = 256 * 1024
)

// BehaviorValueType is a nominal value type. RecordRef participates in identity;
// an equal record shape in a different exact contract is not interchangeable.
type BehaviorValueType struct {
	Type      string       `json:"type"`
	RecordRef *DataTypeRef `json:"recordRef,omitempty"`
}

func ValueBehaviorType(v TypedValue) BehaviorValueType { return BehaviorValueType{v.Type, v.RecordRef} }
func PortBehaviorType(p BehaviorPort) BehaviorValueType {
	return BehaviorValueType{p.Type, p.RecordRef}
}
func SameBehaviorType(a, b BehaviorValueType) bool {
	if a.Type != b.Type {
		return false
	}
	if a.RecordRef == nil || b.RecordRef == nil {
		return a.RecordRef == nil && b.RecordRef == nil
	}
	return *a.RecordRef == *b.RecordRef
}

// A standalone manifest may use contracts embedded in a dependency. Resolve
// locally available identities now; defer other identities to the bundle union.
func validateLocalStructuredReferences(p ComponentPackage) []ValidationIssue {
	issues := []ValidationIssue{}
	local := func(ref *DataTypeRef) bool {
		if ref == nil {
			return false
		}
		for _, lock := range p.DataContracts {
			if lock.Contract.ID == ref.ContractID && lock.Contract.Version == ref.Version {
				return true
			}
		}
		return false
	}
	checkRef := func(ref *DataTypeRef, path string) {
		if local(ref) {
			if _, err := ResolveDataRecord(*ref, p.DataContracts); err != nil {
				issues = append(issues, ValidationIssue{path, err.Error()})
			}
		}
	}
	checkValue := func(value TypedValue, path string) {
		if local(value.RecordRef) {
			issues = append(issues, prefixBehaviorIssues(path, ValidateStructuredTypedValue(value, p.DataContracts))...)
		}
	}
	for _, group := range []struct {
		name  string
		ports []BehaviorPort
	}{{"inputs", p.Behavior.Inputs}, {"outputs", p.Behavior.Outputs}} {
		for i, port := range group.ports {
			checkRef(port.RecordRef, fmt.Sprintf("behavior.%s[%d].recordRef", group.name, i))
		}
	}
	for i, node := range p.Behavior.Nodes {
		path := fmt.Sprintf("behavior.nodes[%d]", i)
		checkRef(node.RecordRef, path+".recordRef")
		for _, name := range sortedBehaviorKeys(node.Inputs) {
			if v := node.Inputs[name].Literal; v != nil {
				checkValue(*v, path+".inputs."+name+".literal")
			}
		}
	}
	for _, name := range sortedBehaviorKeys(p.Behavior.Bindings) {
		if v := p.Behavior.Bindings[name].Literal; v != nil {
			checkValue(*v, "behavior.bindings."+name+".literal")
		}
	}
	for i, test := range p.Tests {
		for _, group := range []struct {
			name   string
			values map[string]TypedValue
		}{{"inputs", test.Inputs}, {"expected", test.Expected}} {
			for _, name := range sortedBehaviorKeys(group.values) {
				checkValue(group.values[name], fmt.Sprintf("tests[%d].%s.%s", i, group.name, name))
			}
		}
	}
	return issues
}

func validateBehaviorValueType(v BehaviorValueType) []ValidationIssue {
	if scalarType(v.Type) {
		if v.RecordRef != nil {
			return []ValidationIssue{{"recordRef", "Scalar types cannot carry a record reference."}}
		}
		return nil
	}
	if v.Type != "record" && v.Type != "collection" {
		return []ValidationIssue{{"type", "Type must be boolean, integer, string, record or collection."}}
	}
	if v.RecordRef == nil {
		return []ValidationIssue{{"recordRef", "Structured types require an exact record reference."}}
	}
	return prefixBehaviorIssues("recordRef", ValidateDataTypeRef(*v.RecordRef))
}

// ResolveDataRecord uses exact embedded locks only. Callers validate the locks
// before resolution; this helper never consults a mutable catalog.
func ResolveDataRecord(ref DataTypeRef, locks []DataContractLock) (DataRecord, error) {
	if issues := ValidateDataTypeRef(ref); len(issues) > 0 {
		return DataRecord{}, fmt.Errorf("invalid record reference: %s", issues[0].Message)
	}
	if len(locks) > 32 {
		return DataRecord{}, fmt.Errorf("at most 32 unique data contracts are supported")
	}
	for _, lock := range locks {
		if lock.Contract.ID != ref.ContractID || lock.Contract.Version != ref.Version {
			continue
		}
		if lock.Digest != ref.Digest {
			return DataRecord{}, fmt.Errorf("record reference digest does not match embedded contract")
		}
		for _, record := range lock.Contract.Records {
			if record.Name == ref.Record {
				return record, nil
			}
		}
		return DataRecord{}, fmt.Errorf("record %q is not declared by the exact contract", ref.Record)
	}
	return DataRecord{}, fmt.Errorf("record reference is missing its exact embedded data contract")
}

// BundleDataContracts returns the deterministic union of exact embedded locks.
// Repeated equal locks are permitted across packages, conflicting identities are not.
func BundleDataContracts(bundle ComponentBundle) ([]DataContractLock, []ValidationIssue) {
	if len(bundle.Packages) > 50 {
		return nil, []ValidationIssue{{"packages", "At most 50 component packages are supported."}}
	}
	values := map[compositionIdentity]DataContractLock{}
	issues := []ValidationIssue{}
	for i, p := range bundle.Packages {
		if len(p.Package.DataContracts) > 8 {
			return nil, []ValidationIssue{{fmt.Sprintf("packages[%d].package.dataContracts", i), "At most 8 data contract locks are supported per component."}}
		}
		for j, lock := range p.Package.DataContracts {
			path := fmt.Sprintf("packages[%d].package.dataContracts[%d]", i, j)
			issues = append(issues, prefixBehaviorIssues(path, ValidateDataContractLock(lock))...)
			key := compositionIdentity{lock.Contract.ID, lock.Contract.Version}
			if existing, ok := values[key]; ok && existing.Digest != lock.Digest {
				issues = append(issues, ValidationIssue{path, "Conflicting data contract digests for the same ID/version."})
			} else {
				values[key] = lock
			}
			if len(values) > 32 {
				return nil, append(issues, ValidationIssue{"packages", "At most 32 unique data contract ID/version pairs are supported per bundle."})
			}
		}
	}
	keys := make([]compositionIdentity, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].id != keys[j].id {
			return keys[i].id < keys[j].id
		}
		return keys[i].version < keys[j].version
	})
	result := make([]DataContractLock, 0, len(keys))
	for _, key := range keys {
		result = append(result, values[key])
	}
	return result, issues
}

// validateStructuredShape validates bounded structure before any recursive
// encoding, cloning, or schema resolution. Records are flat scalar products.
func validateStructuredShape(v TypedValue) []ValidationIssue {
	if scalarType(v.Type) {
		return ValidateTypedValue(v)
	}
	issues := validateBehaviorValueType(ValueBehaviorType(v))
	if v.Boolean != nil || v.Integer != nil || v.String != nil {
		issues = append(issues, ValidationIssue{"", "A structured value cannot contain scalar members."})
	}
	if v.Type == "record" {
		if v.Record == nil || v.Collection != nil {
			issues = append(issues, ValidationIssue{"record", "A record requires a nonnull field object and no collection member."})
		}
		if len(v.Record) < 1 || len(v.Record) > 16 {
			return append(issues, ValidationIssue{"record", "A record requires between 1 and 16 scalar fields."})
		}
		for _, name := range sortedBehaviorKeys(v.Record) {
			if !behaviorName.MatchString(name) {
				issues = append(issues, ValidationIssue{"record." + name, "Field names must be valid local identifiers."})
			}
			issues = append(issues, prefixBehaviorIssues("record."+name, ValidateTypedValue(v.Record[name]))...)
		}
	} else if v.Type == "collection" {
		if v.Record != nil || v.Collection == nil || (v.Collection != nil && *v.Collection == nil) {
			return append(issues, ValidationIssue{"collection", "A collection requires a nonnull array (use [] when empty) and no record member."})
		}
		if len(*v.Collection) > MaxCollectionItems {
			return append(issues, ValidationIssue{"collection", "A collection supports at most 128 records."})
		}
		encodedItems := 0
		for i, item := range *v.Collection {
			path := fmt.Sprintf("collection[%d]", i)
			if item.Type != "record" {
				issues = append(issues, ValidationIssue{path + ".type", "Collection elements must be records."})
				continue
			}
			if !SameBehaviorType(ValueBehaviorType(item), BehaviorValueType{"record", v.RecordRef}) {
				issues = append(issues, ValidationIssue{path + ".recordRef", "Every record must have the collection's exact nominal reference."})
			}
			issues = append(issues, prefixBehaviorIssues(path, validateStructuredShape(item))...)
			if len(issues) >= 200 {
				return issues[:200]
			}
			if len(issues) == 0 {
				// A valid record is already bounded to 64 KiB. Accumulate a
				// lower bound before encoding the enclosing collection so an
				// oversized collection never allocates its entire JSON form.
				encoded, _ := json.Marshal(item)
				encodedItems += len(encoded) + 1
				if encodedItems > MaxStructuredValueBytes {
					return []ValidationIssue{{"", "Structured typed value must not exceed 64 KiB of canonical JSON."}}
				}
			}
		}
	}
	if len(issues) == 0 {
		encoded, err := json.Marshal(v)
		if err != nil || len(encoded) > MaxStructuredValueBytes {
			issues = append(issues, ValidationIssue{"", "Structured typed value must not exceed 64 KiB of canonical JSON."})
		}
	}
	return issues
}

func validateRecordFields(v TypedValue, record DataRecord) []ValidationIssue {
	issues := []ValidationIssue{}
	fields := make(map[string]string, len(record.Fields))
	for _, f := range record.Fields {
		fields[f.Name] = f.Type
	}
	for _, name := range sortedBehaviorKeys(fields) {
		field, ok := v.Record[name]
		if !ok {
			issues = append(issues, ValidationIssue{"record." + name, "A value is required for this record field."})
			continue
		}
		if field.Type != fields[name] {
			issues = append(issues, ValidationIssue{"record." + name + ".type", "Field type must match the exact contract record."})
		}
	}
	for _, name := range sortedBehaviorKeys(v.Record) {
		if _, ok := fields[name]; !ok {
			issues = append(issues, ValidationIssue{"record." + name, "Unknown field is not declared by the closed record."})
		}
	}
	return issues
}

func ValidateStructuredTypedValue(v TypedValue, locks []DataContractLock) []ValidationIssue {
	issues := validateStructuredShape(v)
	if len(issues) > 0 || scalarType(v.Type) {
		return issues
	}
	record, err := ResolveDataRecord(*v.RecordRef, locks)
	if err != nil {
		return []ValidationIssue{{"recordRef", err.Error()}}
	}
	if v.Type == "record" {
		return validateRecordFields(v, record)
	}
	for i, item := range *v.Collection {
		issues = append(issues, prefixBehaviorIssues(fmt.Sprintf("collection[%d]", i), validateRecordFields(item, record))...)
		if len(issues) >= 200 {
			return issues[:200]
		}
	}
	return issues
}

// ValidateStructuredValueMap checks an independent map without imposing ports.
func ValidateStructuredValueMap(values map[string]TypedValue, locks []DataContractLock) []ValidationIssue {
	return validateStructuredValueMap(values, locks, true)
}
func validateStructuredValueMap(values map[string]TypedValue, locks []DataContractLock, resolve bool) []ValidationIssue {
	if len(values) > 16 {
		return []ValidationIssue{{"", "At most 16 typed values are supported."}}
	}
	issues := []ValidationIssue{}
	for _, name := range sortedBehaviorKeys(values) {
		var local []ValidationIssue
		if resolve {
			local = ValidateStructuredTypedValue(values[name], locks)
		} else {
			local = validateStructuredShape(values[name])
		}
		issues = append(issues, prefixBehaviorIssues(name, local)...)
		if len(issues) >= 200 {
			return issues[:200]
		}
	}
	if len(issues) == 0 {
		encoded, err := json.Marshal(values)
		if err != nil || len(encoded) > MaxStructuredValuesBytes {
			issues = append(issues, ValidationIssue{"", "Typed value map must not exceed 256 KiB of canonical JSON."})
		}
	}
	return issues
}
func ValidateStructuredBehaviorValues(values map[string]TypedValue, ports []BehaviorPort, locks []DataContractLock) []ValidationIssue {
	return validateStructuredBehaviorValues(values, ports, locks, true)
}
func validateStructuredBehaviorValues(values map[string]TypedValue, ports []BehaviorPort, locks []DataContractLock, resolve bool) []ValidationIssue {
	if len(ports) > 16 || len(values) > 16 {
		return []ValidationIssue{{"", "At most 16 typed values are supported."}}
	}
	issues := validateStructuredValueMap(values, locks, resolve)
	expected := make(map[string]BehaviorValueType, len(ports))
	for _, port := range ports {
		expected[port.Name] = PortBehaviorType(port)
	}
	for _, name := range sortedBehaviorKeys(expected) {
		v, ok := values[name]
		if !ok {
			issues = append(issues, ValidationIssue{name, "A value is required for this public port."})
			continue
		}
		if !SameBehaviorType(ValueBehaviorType(v), expected[name]) {
			issues = append(issues, ValidationIssue{name + ".type", "Value type and exact record reference must match the public port."})
		}
	}
	for _, name := range sortedBehaviorKeys(values) {
		if _, ok := expected[name]; !ok {
			issues = append(issues, ValidationIssue{name, "Value does not correspond to a public port."})
		}
	}
	return issues
}
