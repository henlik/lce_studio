package model

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestStateExpectationStrictRequiredJSONIntegers(t *testing.T) {
	for _, raw := range []string{
		`null`,
		`{"instanceId":"first","state":{},"receipts":0,"effects":0}`,
		`{"instanceId":"first","state":{},"revision":0,"effects":0}`,
		`{"instanceId":"first","state":{},"revision":0,"receipts":0}`,
		`{"instanceId":"first","state":{},"revision":null,"receipts":0,"effects":0}`,
		`{"instanceId":"first","state":{},"revision":0,"receipts":null,"effects":0}`,
		`{"instanceId":"first","state":{},"revision":0,"receipts":0,"effects":null}`,
		`{"instanceId":"first","state":{},"revision":0.5,"receipts":0,"effects":0}`,
		`{"instanceId":"first","state":{},"revision":0,"receipts":"0","effects":0}`,
		`{"instanceId":"first","state":{},"revision":0,"receipts":0,"effects":false}`,
		`{"instanceId":"first","state":{},"revision":0,"receipts":0,"effects":0,"unknown":1}`,
	} {
		var value StateExpectation
		if err := json.Unmarshal([]byte(raw), &value); err == nil {
			t.Fatalf("accepted malformed expectation %s", raw)
		}
	}
	for _, raw := range []string{
		`{"instanceId":"first","state":{},"revision":0,"receipts":0,"effects":0}`,
		`{"INSTANCEID":"first","STATE":{},"REVISION":0,"RECEIPTS":0,"EFFECTS":0}`,
	} {
		var value StateExpectation
		if err := json.Unmarshal([]byte(raw), &value); err != nil {
			t.Fatal(err)
		}
		encoded, _ := json.Marshal(value)
		if !bytes.Equal(encoded, []byte(`{"instanceId":"first","state":{},"revision":0,"receipts":0,"effects":0}`)) {
			t.Fatal("canonical zero changed", string(encoded))
		}
	}
	lock := stateTestMachineLock(t, stateTestMachine(t))
	for _, field := range []string{"revision", "receipts", "effects"} {
		scenario := stateTestScenario()
		raw, _ := json.Marshal(scenario.Expected[0])
		var object map[string]json.RawMessage
		_ = json.Unmarshal(raw, &object)
		object[field] = json.RawMessage(`9007199254740992`)
		raw, _ = json.Marshal(object)
		if err := json.Unmarshal(raw, &scenario.Expected[0]); err != nil {
			t.Fatal("Go integer should decode before semantic safe-range check", err)
		}
		if issues := ValidateStateScenario(lock, scenario); !issueAt(issues, "expected[0]."+field) {
			t.Fatal("unsafe expectation integer accepted", issues)
		}
	}
	scenario := stateTestScenario()
	raw := `{"instanceId":"first","state":null,"revision":0,"receipts":0,"effects":0}`
	if err := json.Unmarshal([]byte(raw), &scenario.Expected[0]); err != nil {
		t.Fatal(err)
	}
	if issues := ValidateStateScenario(lock, scenario); !issueAt(issues, "expected[0].state") {
		t.Fatal("null expected state accepted", issues)
	}
}

func stateTestMachine(t *testing.T) StateMachine {
	t.Helper()
	p := compositeTestPackage("test.state-decision")
	p.Behavior.Inputs = []BehaviorPort{{Name: "value", Type: "integer"}}
	p.Behavior.Outputs = []BehaviorPort{{Name: "accepted", Type: "boolean"}, {Name: "value", Type: "integer"}}
	yes := behaviorBool(true)
	p.Behavior.Bindings = map[string]BehaviorBinding{"accepted": {Literal: &yes}, "value": {Input: "value"}}
	bundle, issues := PrepareComponentBundle(p, []PackageLock{})
	if len(issues) != 0 {
		t.Fatal(issues)
	}
	return StateMachine{SchemaVersion: 1, ID: "test.machine", Version: "1.0.0", Name: "Owned state", Author: "Test", License: "MIT", DataContracts: []DataContractLock{},
		State:   []StateSlot{{Name: "count", Type: "integer", Durability: "durable", Initial: behaviorInt(3)}, {Name: "volatileCount", Type: "integer", Durability: "volatile", Initial: behaviorInt(0)}},
		Effects: []StateEffect{{Name: "changed", Payload: []BehaviorPort{{Name: "value", Type: "integer"}}}},
		Commands: []StateCommand{{Name: "set", Inputs: []BehaviorPort{{Name: "value", Type: "integer"}}, Outputs: []BehaviorPort{{Name: "value", Type: "integer"}}, Decision: bundle,
			Bindings: map[string]StateBinding{"value": {Input: "value"}}, AcceptedOutput: "accepted", Writes: map[string]string{"count": "value"}, Results: map[string]string{"value": "value"},
			Effects: []StateEffectBinding{{Effect: "changed", WhenOutput: "accepted", Payload: map[string]string{"value": "value"}}}}},
	}
}
func stateTestMachineLock(t *testing.T, m StateMachine) StateMachineLock {
	t.Helper()
	digest, err := StateMachineDigest(m)
	if err != nil {
		t.Fatal(err)
	}
	return StateMachineLock{Machine: m, Digest: digest}
}
func stateTestScenario() StateScenario {
	revision := int64(0)
	return StateScenario{SchemaVersion: 1, Name: "Independent expected result", Instances: []StateInstance{{ID: "first", Name: "First instance", Initial: map[string]TypedValue{}}},
		Steps:    []StateStep{{Kind: "command", InstanceID: "first", CommandID: "requestOne", Command: "set", ExpectedRevision: &revision, Inputs: map[string]TypedValue{"value": behaviorInt(2)}}},
		Expected: []StateExpectation{{InstanceID: "first", State: map[string]TypedValue{"count": behaviorInt(2), "volatileCount": behaviorInt(0)}, Revision: 1, Receipts: 1, Effects: 1}},
	}
}

func TestStateMachineTypedDigestAndValidationAreReadOnly(t *testing.T) {
	m := stateTestMachine(t)
	before, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := StateMachineDigest(m)
	sum := sha256.Sum256(before)
	if err != nil || digest != hex.EncodeToString(sum[:]) {
		t.Fatal(digest, err)
	}
	if issues := ValidateStateMachineLock(StateMachineLock{Machine: m, Digest: digest}); issues == nil || len(issues) != 0 {
		t.Fatal(issues)
	}
	after, _ := json.Marshal(m)
	if string(before) != string(after) {
		t.Fatal("validation mutated machine")
	}
	m.Name += " edited"
	if len(ValidateStateMachineLock(StateMachineLock{Machine: m, Digest: digest})) == 0 {
		t.Fatal("tampered machine accepted")
	}
	changed, err := StateMachineDigest(m)
	if err != nil || changed == digest {
		t.Fatal("metadata does not participate in identity")
	}
	m.State = nil
	if _, err := StateMachineDigest(m); err == nil {
		t.Fatal("digest accepted invalid machine")
	}
}

func TestStateMachineRejectsMalformedDeclarationsAndMappings(t *testing.T) {
	for _, tc := range []struct {
		name, path string
		mutate     func(*StateMachine)
	}{
		{"schema", "schemaVersion", func(m *StateMachine) { m.SchemaVersion = 2 }},
		{"metadata", "id", func(m *StateMachine) { m.ID = "notNamespaced" }},
		{"null contracts", "dataContracts", func(m *StateMachine) { m.DataContracts = nil }},
		{"empty state", "state", func(m *StateMachine) { m.State = []StateSlot{} }},
		{"too much state", "state", func(m *StateMachine) { m.State = make([]StateSlot, 9) }},
		{"null effects", "effects", func(m *StateMachine) { m.Effects = nil }},
		{"null commands", "commands", func(m *StateMachine) { m.Commands = nil }},
		{"bad slot", "state[0].name", func(m *StateMachine) { m.State[0].Name = "9bad" }},
		{"duplicate slot", "state[1].name", func(m *StateMachine) { m.State[1].Name = m.State[0].Name }},
		{"bad durability", "state[0].durability", func(m *StateMachine) { m.State[0].Durability = "persistent" }},
		{"wrong initial", "state[0].initial", func(m *StateMachine) { m.State[0].Initial = behaviorBool(false) }},
		{"null initial", "state[0].initial", func(m *StateMachine) { m.State[0].Initial = TypedValue{Type: "integer"} }},
		{"duplicate effect", "effects[1].name", func(m *StateMachine) { m.Effects = append(m.Effects, m.Effects[0]) }},
		{"null payload ports", "effects[0].payload", func(m *StateMachine) { m.Effects[0].Payload = nil }},
		{"duplicate ports", "effects[0].payload[1].name", func(m *StateMachine) { m.Effects[0].Payload = append(m.Effects[0].Payload, m.Effects[0].Payload[0]) }},
		{"duplicate command", "commands[1].name", func(m *StateMachine) { m.Commands = append(m.Commands, m.Commands[0]) }},
		{"null inputs", "commands[0].inputs", func(m *StateMachine) { m.Commands[0].Inputs = nil }},
		{"null outputs", "commands[0].outputs", func(m *StateMachine) { m.Commands[0].Outputs = nil }},
		{"null bindings", "commands[0].bindings", func(m *StateMachine) { m.Commands[0].Bindings = nil }},
		{"missing binding", "commands[0].bindings.value", func(m *StateMachine) { m.Commands[0].Bindings = map[string]StateBinding{} }},
		{"extra binding", "commands[0].bindings.extra", func(m *StateMachine) { m.Commands[0].Bindings["extra"] = StateBinding{State: "count"} }},
		{"ambiguous source", "commands[0].bindings.value", func(m *StateMachine) { m.Commands[0].Bindings["value"] = StateBinding{Input: "value", State: "count"} }},
		{"unknown state", "commands[0].bindings.value.state", func(m *StateMachine) { m.Commands[0].Bindings["value"] = StateBinding{State: "otherInstance"} }},
		{"unknown input", "commands[0].bindings.value.input", func(m *StateMachine) { m.Commands[0].Bindings["value"] = StateBinding{Input: "unknown"} }},
		{"wrong literal type", "commands[0].bindings.value", func(m *StateMachine) {
			v := behaviorBool(true)
			m.Commands[0].Bindings["value"] = StateBinding{Literal: &v}
		}},
		{"nonboolean acceptance", "commands[0].acceptedOutput", func(m *StateMachine) { m.Commands[0].AcceptedOutput = "value" }},
		{"unknown write target", "commands[0].writes.foreign", func(m *StateMachine) { m.Commands[0].Writes["foreign"] = "value" }},
		{"unknown write output", "commands[0].writes.count", func(m *StateMachine) { m.Commands[0].Writes["count"] = "unknown" }},
		{"wrong write type", "commands[0].writes.count", func(m *StateMachine) { m.Commands[0].Writes["count"] = "accepted" }},
		{"missing result", "commands[0].results.value", func(m *StateMachine) { m.Commands[0].Results = map[string]string{} }},
		{"unknown effect", "commands[0].effects[0].effect", func(m *StateMachine) { m.Commands[0].Effects[0].Effect = "unknown" }},
		{"repeated effect", "commands[0].effects[1].effect", func(m *StateMachine) { m.Commands[0].Effects = append(m.Commands[0].Effects, m.Commands[0].Effects[0]) }},
		{"nonboolean effect guard", "commands[0].effects[0].whenOutput", func(m *StateMachine) { m.Commands[0].Effects[0].WhenOutput = "value" }},
		{"missing effect field", "commands[0].effects[0].payload.value", func(m *StateMachine) { m.Commands[0].Effects[0].Payload = map[string]string{} }},
		{"wrong effect type", "commands[0].effects[0].payload.value", func(m *StateMachine) { m.Commands[0].Effects[0].Payload["value"] = "accepted" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := stateTestMachine(t)
			tc.mutate(&m)
			issues := ValidateStateMachine(m)
			if !issueAt(issues, tc.path) {
				t.Fatalf("want %s, got %+v", tc.path, issues)
			}
		})
	}
	// Empty signatures/mappings and valid unused state/input/lock declarations are supported.
	m := stateTestMachine(t)
	m.Commands[0].Outputs = []BehaviorPort{}
	m.Commands[0].Results = map[string]string{}
	m.Commands[0].Writes = map[string]string{}
	m.Commands[0].Effects = []StateEffectBinding{}
	m.Commands[0].Bindings["value"] = StateBinding{State: "count"}
	if issues := ValidateStateMachine(m); len(issues) != 0 {
		t.Fatal(issues)
	}
}

func TestStateMachineDecisionClosuresStayIndependentAndGloballyConsistent(t *testing.T) {
	m := stateTestMachine(t)
	second := m.Commands[0]
	second.Name = "again"
	m.Commands = append(m.Commands, second)
	if issues := ValidateStateMachine(m); len(issues) != 0 {
		t.Fatal("same exact dependency reuse rejected", issues)
	}
	m = stateTestMachine(t)
	second = m.Commands[0]
	second.Name = "again"
	second.Decision.Packages = append([]PackageLock{}, second.Decision.Packages...)
	second.Decision.Packages[0].Package.Name += " different"
	compositeRelockRoot(t, &second.Decision)
	m.Commands = append(m.Commands, second)
	if issues := ValidateStateMachine(m); len(issues) == 0 {
		t.Fatal("different definitions under one package ID/version accepted")
	}
	m = stateTestMachine(t)
	data, record := structuredTestRecord()
	m.DataContracts = []DataContractLock{data, data}
	if len(ValidateStateMachine(m)) == 0 {
		t.Fatal("duplicate machine-local data locks accepted")
	}
	m = stateTestMachine(t)
	structured := compositeTestPackage("test.structured-decision")
	structured.SchemaVersion = 3
	structured.Behavior.Inputs = []BehaviorPort{{Name: "record", Type: "record", RecordRef: record.RecordRef}}
	structured.Behavior.Outputs = []BehaviorPort{{Name: "accepted", Type: "boolean"}}
	structured.Behavior.Nodes = []BehaviorNode{{ID: "active", Op: "record_get", Field: "active", Inputs: map[string]BehaviorBinding{"value": {Input: "record"}}}}
	structured.Behavior.Bindings = map[string]BehaviorBinding{"accepted": {Node: "active", Port: "result"}}
	structured.DataContracts = []DataContractLock{data}
	bundle, issues := PrepareComponentBundle(structured, []PackageLock{})
	if len(issues) != 0 {
		t.Fatal(issues)
	}
	c := &m.Commands[0]
	c.Decision = bundle
	c.Inputs = []BehaviorPort{{Name: "record", Type: "record", RecordRef: record.RecordRef}}
	c.Bindings = map[string]StateBinding{"record": {Input: "record"}}
	c.Outputs = []BehaviorPort{}
	c.Results = map[string]string{}
	c.Writes = map[string]string{}
	c.Effects = []StateEffectBinding{}
	if issues := ValidateStateMachine(m); len(issues) != 0 {
		t.Fatal(issues)
	}
	m.DataContracts = []DataContractLock{data}
	m.Commands[0].Decision.Packages[0].Package.DataContracts = nil
	compositeRelockRoot(t, &m.Commands[0].Decision)
	if len(ValidateStateMachine(m)) == 0 {
		t.Fatal("machine-only lock silently supplied a pure decision dependency")
	}
}

func TestStateScenarioValidatesInputsAndIndependentExpectations(t *testing.T) {
	lock := stateTestMachineLock(t, stateTestMachine(t))
	scenario := stateTestScenario()
	before, _ := json.Marshal(scenario)
	if issues := ValidateStateScenario(lock, scenario); issues == nil || len(issues) != 0 {
		t.Fatal(issues)
	}
	after, _ := json.Marshal(scenario)
	if string(before) != string(after) {
		t.Fatal("validation mutated scenario")
	}
	for _, tc := range []struct {
		name, path string
		mutate     func(*StateScenario)
	}{
		{"schema", "schemaVersion", func(s *StateScenario) { s.SchemaVersion = 2 }},
		{"missing name", "name", func(s *StateScenario) { s.Name = "" }},
		{"no instances", "instances", func(s *StateScenario) { s.Instances = []StateInstance{} }},
		{"too many steps", "steps", func(s *StateScenario) { s.Steps = make([]StateStep, 65) }},
		{"null expectations", "expected", func(s *StateScenario) { s.Expected = nil }},
		{"duplicate instance", "instances[1].id", func(s *StateScenario) { s.Instances = append(s.Instances, s.Instances[0]) }},
		{"bad instance ID", "instances[0].id", func(s *StateScenario) { s.Instances[0].ID = "not-safe" }},
		{"null overrides", "instances[0].initial", func(s *StateScenario) { s.Instances[0].Initial = nil }},
		{"foreign override", "instances[0].initial.foreign", func(s *StateScenario) { s.Instances[0].Initial["foreign"] = behaviorInt(2) }},
		{"wrong override type", "instances[0].initial.count", func(s *StateScenario) { s.Instances[0].Initial["count"] = behaviorString("2") }},
		{"unknown instance", "steps[0].instanceId", func(s *StateScenario) { s.Steps[0].InstanceID = "unknown" }},
		{"unknown kind", "steps[0].kind", func(s *StateScenario) { s.Steps[0].Kind = "deliver" }},
		{"unknown command", "steps[0].command", func(s *StateScenario) { s.Steps[0].Command = "unknown" }},
		{"missing command ID", "steps[0].commandId", func(s *StateScenario) { s.Steps[0].CommandID = "" }},
		{"missing revision", "steps[0].expectedRevision", func(s *StateScenario) { s.Steps[0].ExpectedRevision = nil }},
		{"negative revision", "steps[0].expectedRevision", func(s *StateScenario) { n := int64(-1); s.Steps[0].ExpectedRevision = &n }},
		{"unsafe revision", "steps[0].expectedRevision", func(s *StateScenario) { n := MaxBehaviorInteger + 1; s.Steps[0].ExpectedRevision = &n }},
		{"unknown fault", "steps[0].fault", func(s *StateScenario) { s.Steps[0].Fault = "after_reply" }},
		{"null command inputs", "steps[0].inputs", func(s *StateScenario) { s.Steps[0].Inputs = nil }},
		{"missing command input", "steps[0].inputs.value", func(s *StateScenario) { s.Steps[0].Inputs = map[string]TypedValue{} }},
		{"wrong command input", "steps[0].inputs.value", func(s *StateScenario) { s.Steps[0].Inputs["value"] = behaviorBool(true) }},
		{"restart command fields", "steps[0]", func(s *StateScenario) { s.Steps[0].Kind = "restart" }},
		{"unknown expected instance", "expected[0].instanceId", func(s *StateScenario) { s.Expected[0].InstanceID = "unknown" }},
		{"duplicate expectation", "expected[1].instanceId", func(s *StateScenario) { s.Expected = append(s.Expected, s.Expected[0]) }},
		{"missing expected slot", "expected[0].state.count", func(s *StateScenario) { delete(s.Expected[0].State, "count") }},
		{"wrong expected type", "expected[0].state.count", func(s *StateScenario) { s.Expected[0].State["count"] = behaviorBool(false) }},
		{"negative expectation", "expected[0].receipts", func(s *StateScenario) { s.Expected[0].Receipts = -1 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := stateTestScenario()
			tc.mutate(&s)
			if issues := ValidateStateScenario(lock, s); !issueAt(issues, tc.path) {
				t.Fatalf("want %s; got %+v", tc.path, issues)
			}
		})
	}
	scenario = stateTestScenario()
	scenario.Expected = []StateExpectation{}
	scenario.Steps = []StateStep{{Kind: "restart", InstanceID: "first"}}
	if issues := ValidateStateScenario(lock, scenario); len(issues) != 0 {
		t.Fatal("unverified restart scenario rejected", issues)
	}
	raw, _ := json.Marshal(scenario.Steps[0])
	if string(raw) != `{"kind":"restart","instanceId":"first","inputs":null}` {
		t.Fatal("restart canonical representation changed", string(raw))
	}
	scenario = stateTestScenario()
	scenario.Expected[0].State["count"] = behaviorInt(999)
	scenario.Expected[0].Receipts = 1000
	if issues := ValidateStateScenario(lock, scenario); len(issues) != 0 {
		t.Fatal("model validation executed or inferred expected outcomes", issues)
	}
}

func stateTestLargeRecord(t *testing.T) (DataContractLock, TypedValue, TypedValue) {
	t.Helper()
	lock, large := structuredTestRecord()
	lock.Contract.Records = []DataRecord{{Name: "Payload", Fields: []DataField{}}}
	lock.Contract.Entities = []DataEntity{}
	lock.Contract.Relationships = []DataRelationship{}
	large.Record = map[string]TypedValue{}
	small := TypedValue{Type: "record", Record: map[string]TypedValue{}}
	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("field%d", i)
		lock.Contract.Records[0].Fields = append(lock.Contract.Records[0].Fields, DataField{Name: name, Type: "string"})
		large.Record[name] = behaviorString(strings.Repeat("x", 4000))
		small.Record[name] = behaviorString("")
	}
	var err error
	lock.Digest, err = DataContractDigest(lock.Contract)
	if err != nil {
		t.Fatal(err)
	}
	ref := &DataTypeRef{ContractID: lock.Contract.ID, Version: lock.Contract.Version, Digest: lock.Digest, Record: "Payload"}
	large.RecordRef = ref
	small.RecordRef = ref
	return lock, large, small
}

func TestStateValueAndScenarioBudgetsPrecedeEncoding(t *testing.T) {
	data, large, small := stateTestLargeRecord(t)
	contracts := []DataContractLock{data}
	if issues := ValidateStateValues(map[string]TypedValue{"first": large}, contracts); len(issues) != 0 {
		t.Fatal(issues)
	}
	if len(ValidateStateValues(map[string]TypedValue{"first": large, "second": large}, contracts)) == 0 {
		t.Fatal("accepted >64KiB state map")
	}
	cyclic := TypedValue{Type: "record", RecordRef: large.RecordRef, Record: map[string]TypedValue{}}
	cyclic.Record["self"] = cyclic
	if len(ValidateStateValues(map[string]TypedValue{"value": cyclic}, contracts)) == 0 {
		t.Fatal("accepted recursive Go value")
	}
	m := stateTestMachine(t)
	m.DataContracts = contracts
	m.State = []StateSlot{{Name: "first", Type: "record", RecordRef: large.RecordRef, Durability: "durable", Initial: large}, {Name: "second", Type: "record", RecordRef: small.RecordRef, Durability: "volatile", Initial: small}}
	m.Commands[0].Writes = map[string]string{}
	lock := stateTestMachineLock(t, m)
	s := stateTestScenario()
	s.Expected = []StateExpectation{}
	s.Instances[0].Initial = map[string]TypedValue{"second": large}
	if len(ValidateStateScenario(lock, s)) == 0 {
		t.Fatal("accepted oversized merged defaults+overrides")
	}
	m = stateTestMachine(t)
	m.DataContracts = contracts
	m.Commands[0].Inputs = []BehaviorPort{{Name: "payload", Type: "record", RecordRef: large.RecordRef}}
	number := behaviorInt(1)
	m.Commands[0].Bindings = map[string]StateBinding{"value": {Literal: &number}}
	lock = stateTestMachineLock(t, m)
	s = stateTestScenario()
	s.Expected = []StateExpectation{}
	s.Steps = []StateStep{}
	for i := 0; i < 14; i++ {
		revision := int64(i)
		s.Steps = append(s.Steps, StateStep{Kind: "command", InstanceID: "first", CommandID: fmt.Sprintf("request%d", i), Command: "set", ExpectedRevision: &revision, Inputs: map[string]TypedValue{"payload": large}})
	}
	if len(ValidateStateScenario(lock, s)) == 0 {
		t.Fatal("accepted >512KiB scenario")
	}
	// A malformed value is rejected before recursive scenario serialization.
	s = stateTestScenario()
	s.Steps[0].Inputs = map[string]TypedValue{"payload": cyclic}
	if len(ValidateStateScenario(lock, s)) == 0 {
		t.Fatal("accepted recursive scenario value")
	}
}

func TestStateNominalMappingsRejectEqualShapeDifferentContracts(t *testing.T) {
	data, record := structuredTestRecord()
	second := data
	second.Contract.ID = "test.other-registry"
	second.Digest, _ = DataContractDigest(second.Contract)
	otherRef := *record.RecordRef
	otherRef.ContractID = second.Contract.ID
	otherRef.Digest = second.Digest
	m := stateTestMachine(t)
	m.DataContracts = []DataContractLock{data, second}
	m.State[0] = StateSlot{Name: "count", Type: "record", RecordRef: &otherRef, Durability: "durable", Initial: record}
	if len(ValidateStateMachine(m)) == 0 {
		t.Fatal("same-shaped different nominal initial accepted")
	}
	refBefore := *record.RecordRef
	_ = ValidateStateMachine(m)
	if !reflect.DeepEqual(*record.RecordRef, refBefore) {
		t.Fatal("validation mutated nominal reference")
	}
}

func TestStateMachineAggregateByteLimitCountsRepeatedEmbeddedDecisions(t *testing.T) {
	m := stateTestMachine(t)
	p := m.Commands[0].Decision.Packages[0].Package
	p.Behavior.Inputs = []BehaviorPort{{Name: "first", Type: "string"}, {Name: "second", Type: "string"}}
	value := behaviorInt(2)
	p.Behavior.Bindings["value"] = BehaviorBinding{Literal: &value}
	large := behaviorString(strings.Repeat("x", 4000))
	p.Tests = []BehaviorCase{}
	for i := 0; i < 9; i++ {
		p.Tests = append(p.Tests, BehaviorCase{Name: fmt.Sprintf("case%d", i), Inputs: map[string]TypedValue{"first": large, "second": large}, Expected: map[string]TypedValue{"accepted": behaviorBool(true), "value": value}})
	}
	bundle, issues := PrepareComponentBundle(p, []PackageLock{})
	if len(issues) != 0 {
		t.Fatal(issues)
	}
	m.Commands[0].Decision = bundle
	m.Commands[0].Bindings = map[string]StateBinding{"first": {Literal: &large}, "second": {Literal: &large}}
	base := m.Commands[0]
	for i := 1; i < 8; i++ {
		command := base
		command.Name = fmt.Sprintf("set%d", i)
		m.Commands = append(m.Commands, command)
	}
	if contracts, issues := StateMachineDataContracts(m); len(issues) != 0 || len(contracts) != 0 {
		t.Fatal("exact repeated component locks should remain legal", issues)
	}
	if issues := ValidateStateMachine(m); !issueAt(issues, "") {
		t.Fatal("accepted >512KiB machine with repeated valid decision bundles", issues)
	}
}
