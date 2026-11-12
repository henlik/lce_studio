package model

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"
)

func deliveryTestModel(t *testing.T) DeliveryModel {
	t.Helper()
	machine := stateTestMachineLock(t, stateTestMachine(t))
	ref := DeliveryMachineRef{MachineID: machine.Machine.ID, Version: machine.Machine.Version, Digest: machine.Digest}
	return DeliveryModel{SchemaVersion: 1, ID: "test.delivery", Version: "1.0.0", Name: "Explicit typed delivery", Author: "Test", License: "MIT", Machines: []StateMachineLock{machine},
		Instances: []DeliveryInstance{{ID: "source", Name: "Source owner", Machine: ref, Position: Position{X: 0, Y: 0}}, {ID: "target", Name: "Target owner", Machine: ref, Position: Position{X: 340, Y: 0}}},
		Routes:    []DeliveryRoute{{ID: "transfer", Name: "Changed value", SourceInstanceID: "source", Effect: "changed", TargetInstanceID: "target", Command: "set", Inputs: map[string]string{"value": "value"}}},
	}
}
func deliveryTestLock(t *testing.T, m DeliveryModel) DeliveryModelLock {
	t.Helper()
	digest, err := DeliveryModelDigest(m)
	if err != nil {
		t.Fatal(err)
	}
	return DeliveryModelLock{Model: m, Digest: digest}
}
func deliveryTestScenario() DeliveryScenario {
	zero := int64(0)
	yes := true
	return DeliveryScenario{SchemaVersion: 1, Name: "Explicit sender and recipient", Initial: []DeliveryInitial{{InstanceID: "source", Initial: map[string]TypedValue{}}, {InstanceID: "target", Initial: map[string]TypedValue{}}},
		Steps:            []DeliveryStep{{Kind: "command", InstanceID: "source", CommandID: "requestOne", Command: "set", ExpectedRevision: &zero, Inputs: map[string]TypedValue{"value": behaviorInt(2)}}, {Kind: "dispatch", RouteID: "transfer", SourceCommandID: "requestOne", Message: "firstMessage", ExpectedRevision: &zero}, {Kind: "deliver", Message: "firstMessage"}},
		Expected:         []StateExpectation{{InstanceID: "source", State: map[string]TypedValue{"count": behaviorInt(2), "volatileCount": behaviorInt(0)}, Revision: 1, Receipts: 1, Effects: 1}, {InstanceID: "target", State: map[string]TypedValue{"count": behaviorInt(2), "volatileCount": behaviorInt(0)}, Revision: 1, Receipts: 1, Effects: 1}},
		ExpectedMessages: []DeliveryMessageExpectation{{Name: "firstMessage", Status: "acknowledged", Attempts: 1, Accepted: &yes}},
	}
}

func TestDeliveryModelDigestIsTypedAndReadOnly(t *testing.T) {
	m := deliveryTestModel(t)
	before, _ := json.Marshal(m)
	sum := sha256.Sum256(before)
	digest, err := DeliveryModelDigest(m)
	if err != nil || digest != hex.EncodeToString(sum[:]) {
		t.Fatal(digest, err)
	}
	if issues := ValidateDeliveryModelLock(DeliveryModelLock{Model: m, Digest: digest}); issues == nil || len(issues) != 0 {
		t.Fatal(issues)
	}
	after, _ := json.Marshal(m)
	if string(before) != string(after) {
		t.Fatal("validation mutated delivery model")
	}
	m.Instances[0].Position.X++
	if len(ValidateDeliveryModelLock(DeliveryModelLock{Model: m, Digest: digest})) == 0 {
		t.Fatal("layout mutation retained old digest")
	}
	m.Machines = nil
	if _, err := DeliveryModelDigest(m); err == nil {
		t.Fatal("invalid model hashed")
	}
}

func TestDeliveryModelRejectsMalformedOwnersAndRoutes(t *testing.T) {
	for _, tc := range []struct {
		name, path string
		mutate     func(*DeliveryModel)
	}{
		{"schema", "schemaVersion", func(m *DeliveryModel) { m.SchemaVersion = 2 }},
		{"identity", "id", func(m *DeliveryModel) { m.ID = "unnamespaced" }},
		{"null machines", "machines", func(m *DeliveryModel) { m.Machines = nil }},
		{"too many machines", "machines", func(m *DeliveryModel) { m.Machines = make([]StateMachineLock, 5) }},
		{"duplicate machine identity", "machines[1]", func(m *DeliveryModel) { m.Machines = append(m.Machines, m.Machines[0]) }},
		{"tampered machine", "machines[0].digest", func(m *DeliveryModel) { m.Machines[0].Machine.Name += " tampered" }},
		{"no instances", "instances", func(m *DeliveryModel) { m.Instances = []DeliveryInstance{} }},
		{"too many instances", "instances", func(m *DeliveryModel) { m.Instances = make([]DeliveryInstance, 9) }},
		{"bad instance ID", "instances[0].id", func(m *DeliveryModel) { m.Instances[0].ID = "bad-id" }},
		{"duplicate instance", "instances[1].id", func(m *DeliveryModel) { m.Instances[1].ID = m.Instances[0].ID }},
		{"blank display name", "instances[0].name", func(m *DeliveryModel) { m.Instances[0].Name = "" }},
		{"nonfinite position", "instances[0].position.x", func(m *DeliveryModel) { m.Instances[0].Position.X = math.Inf(1) }},
		{"position bound", "instances[0].position.y", func(m *DeliveryModel) { m.Instances[0].Position.Y = 1e6 + 1 }},
		{"unresolved machine", "instances[0].machine", func(m *DeliveryModel) { m.Instances[0].Machine.MachineID = "test.missing" }},
		{"mismatched machine digest", "instances[0].machine.digest", func(m *DeliveryModel) { m.Instances[0].Machine.Digest = strings.Repeat("a", 64) }},
		{"null routes", "routes", func(m *DeliveryModel) { m.Routes = nil }},
		{"too many routes", "routes", func(m *DeliveryModel) { m.Routes = make([]DeliveryRoute, 17) }},
		{"duplicate route ID", "routes[1].id", func(m *DeliveryModel) { m.Routes = append(m.Routes, m.Routes[0]) }},
		{"duplicate route tuple", "routes[1]", func(m *DeliveryModel) { r := m.Routes[0]; r.ID = "second"; m.Routes = append(m.Routes, r) }},
		{"unknown source", "routes[0].sourceInstanceId", func(m *DeliveryModel) { m.Routes[0].SourceInstanceID = "missing" }},
		{"unknown target", "routes[0].targetInstanceId", func(m *DeliveryModel) { m.Routes[0].TargetInstanceID = "missing" }},
		{"unknown effect", "routes[0].effect", func(m *DeliveryModel) { m.Routes[0].Effect = "missing" }},
		{"unknown command", "routes[0].command", func(m *DeliveryModel) { m.Routes[0].Command = "missing" }},
		{"null inputs", "routes[0].inputs", func(m *DeliveryModel) { m.Routes[0].Inputs = nil }},
		{"missing mapped input", "routes[0].inputs.value", func(m *DeliveryModel) { m.Routes[0].Inputs = map[string]string{} }},
		{"extra mapped input", "routes[0].inputs.extra", func(m *DeliveryModel) { m.Routes[0].Inputs["extra"] = "value" }},
		{"unknown payload source", "routes[0].inputs.value", func(m *DeliveryModel) { m.Routes[0].Inputs["value"] = "missing" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := deliveryTestModel(t)
			tc.mutate(&m)
			if issues := ValidateDeliveryModel(m); !issueAt(issues, tc.path) {
				t.Fatalf("want %s, got %+v", tc.path, issues)
			}
		})
	}
	m := deliveryTestModel(t)
	m.Routes = append(m.Routes, DeliveryRoute{ID: "back", Name: "Return", SourceInstanceID: "target", Effect: "changed", TargetInstanceID: "source", Command: "set", Inputs: map[string]string{"value": "value"}}, DeliveryRoute{ID: "self", Name: "Self", SourceInstanceID: "source", Effect: "changed", TargetInstanceID: "source", Command: "set", Inputs: map[string]string{"value": "value"}})
	if issues := ValidateDeliveryModel(m); len(issues) != 0 {
		t.Fatal("explicit cycle/self/fanout routes rejected", issues)
	}
	unused := stateTestMachine(t)
	unused.ID = "test.unused"
	m.Machines = append(m.Machines, stateTestMachineLock(t, unused))
	if issues := ValidateDeliveryModel(m); len(issues) != 0 {
		t.Fatal("valid unused machine rejected", issues)
	}
}

func TestDeliveryGlobalLockConflictAndNominalRouteTypes(t *testing.T) {
	m := deliveryTestModel(t)
	other := stateTestMachine(t)
	other.ID = "test.other"
	other.Commands[0].Decision.Packages[0].Package.Name += " changed"
	compositeRelockRoot(t, &other.Commands[0].Decision)
	m.Machines = append(m.Machines, stateTestMachineLock(t, other))
	if len(ValidateDeliveryModel(m)) == 0 {
		t.Fatal("conflicting component identity across machines accepted")
	}
	m = deliveryTestModel(t)
	data, _ := structuredTestRecord()
	m.Machines[0].Machine.DataContracts = []DataContractLock{data}
	m.Machines[0] = stateTestMachineLock(t, m.Machines[0].Machine)
	for i := range m.Instances {
		m.Instances[i].Machine.Digest = m.Machines[0].Digest
	}
	other = stateTestMachine(t)
	other.ID = "test.other"
	changed := data
	changed.Contract.Name += " changed"
	changed.Digest, _ = DataContractDigest(changed.Contract)
	other.DataContracts = []DataContractLock{changed}
	m.Machines = append(m.Machines, stateTestMachineLock(t, other))
	if len(ValidateDeliveryModel(m)) == 0 {
		t.Fatal("conflicting data identity across machines accepted")
	}
	m = deliveryTestModel(t)
	other = stateTestMachine(t)
	other.ID = "test.recipient"
	other.Commands[0].Inputs[0].Type = "string"
	value := behaviorInt(1)
	other.Commands[0].Bindings = map[string]StateBinding{"value": {Literal: &value}}
	recipient := stateTestMachineLock(t, other)
	m.Machines = append(m.Machines, recipient)
	m.Instances[1].Machine = DeliveryMachineRef{MachineID: other.ID, Version: other.Version, Digest: recipient.Digest}
	if issues := ValidateDeliveryModel(m); !issueAt(issues, "routes[0].inputs.value") {
		t.Fatal("incompatible route types accepted", issues)
	}
}

func TestDeliveryScenarioFieldMatrixAndActualValues(t *testing.T) {
	lock := deliveryTestLock(t, deliveryTestModel(t))
	scenario := deliveryTestScenario()
	before, _ := json.Marshal(scenario)
	if issues := ValidateDeliveryScenario(lock, scenario); issues == nil || len(issues) != 0 {
		t.Fatal(issues)
	}
	after, _ := json.Marshal(scenario)
	if string(before) != string(after) {
		t.Fatal("validation mutated scenario")
	}
	for _, tc := range []struct {
		name, path string
		mutate     func(*DeliveryScenario)
	}{
		{"schema", "schemaVersion", func(s *DeliveryScenario) { s.SchemaVersion = 2 }},
		{"no steps", "steps", func(s *DeliveryScenario) { s.Steps = []DeliveryStep{} }},
		{"too many steps", "steps", func(s *DeliveryScenario) { s.Steps = make([]DeliveryStep, 65) }},
		{"null initial", "initial", func(s *DeliveryScenario) { s.Initial = nil }},
		{"missing initial", "initial", func(s *DeliveryScenario) { s.Initial = s.Initial[:1] }},
		{"duplicate initial", "initial[1].instanceId", func(s *DeliveryScenario) { s.Initial[1].InstanceID = s.Initial[0].InstanceID }},
		{"unknown initial", "initial[0].instanceId", func(s *DeliveryScenario) { s.Initial[0].InstanceID = "missing" }},
		{"null override map", "initial[0].initial", func(s *DeliveryScenario) { s.Initial[0].Initial = nil }},
		{"unknown override", "initial[0].initial.missing", func(s *DeliveryScenario) { s.Initial[0].Initial["missing"] = behaviorInt(1) }},
		{"wrong override", "initial[0].initial.count", func(s *DeliveryScenario) { s.Initial[0].Initial["count"] = behaviorBool(true) }},
		{"command transport field", "steps[0]", func(s *DeliveryScenario) { s.Steps[0].Message = "unrelated" }},
		{"unknown command owner", "steps[0].instanceId", func(s *DeliveryScenario) { s.Steps[0].InstanceID = "missing" }},
		{"reserved command ID", "steps[0].commandId", func(s *DeliveryScenario) { s.Steps[0].CommandID = "msg_reserved" }},
		{"missing expected revision", "steps[0].expectedRevision", func(s *DeliveryScenario) { s.Steps[0].ExpectedRevision = nil }},
		{"wrong command input", "steps[0].inputs.value", func(s *DeliveryScenario) { s.Steps[0].Inputs["value"] = behaviorBool(true) }},
		{"null command inputs", "steps[0].inputs", func(s *DeliveryScenario) { s.Steps[0].Inputs = nil }},
		{"wrong command fault", "steps[0].fault", func(s *DeliveryScenario) { s.Steps[0].Fault = "drop_before_receive" }},
		{"restart extra field", "steps[0]", func(s *DeliveryScenario) { s.Steps[0].Kind = "restart" }},
		{"dispatch inputs object", "steps[1]", func(s *DeliveryScenario) { s.Steps[1].Inputs = map[string]TypedValue{} }},
		{"dispatch extra owner", "steps[1]", func(s *DeliveryScenario) { s.Steps[1].InstanceID = "source" }},
		{"unknown route", "steps[1].routeId", func(s *DeliveryScenario) { s.Steps[1].RouteID = "missing" }},
		{"missing dispatch revision", "steps[1].expectedRevision", func(s *DeliveryScenario) { s.Steps[1].ExpectedRevision = nil }},
		{"negative dispatch revision", "steps[1].expectedRevision", func(s *DeliveryScenario) { negative := int64(-1); s.Steps[1].ExpectedRevision = &negative }},
		{"unsafe dispatch revision", "steps[1].expectedRevision", func(s *DeliveryScenario) { unsafe := MaxBehaviorInteger + 1; s.Steps[1].ExpectedRevision = &unsafe }},
		{"missing source selector", "steps[1]", func(s *DeliveryScenario) { s.Steps[1].SourceCommandID = "" }},
		{"ambiguous source", "steps[1]", func(s *DeliveryScenario) { s.Steps[1].SourceMessage = "firstMessage" }},
		{"reserved source command", "steps[1].sourceCommandId", func(s *DeliveryScenario) { s.Steps[1].SourceCommandID = "msg_reserved" }},
		{"future source message", "steps[1].sourceMessage", func(s *DeliveryScenario) { s.Steps[1].SourceCommandID = ""; s.Steps[1].SourceMessage = "future" }},
		{"deliver extra revision", "steps[2]", func(s *DeliveryScenario) { zero := int64(0); s.Steps[2].ExpectedRevision = &zero }},
		{"deliver undeclared alias", "steps[2].message", func(s *DeliveryScenario) { s.Steps[2].Message = "missing" }},
		{"deliver unknown fault", "steps[2].fault", func(s *DeliveryScenario) { s.Steps[2].Fault = "before_commit" }},
		{"null state expectations", "expected", func(s *DeliveryScenario) { s.Expected = nil }},
		{"missing expected slot", "expected[0].state.count", func(s *DeliveryScenario) { delete(s.Expected[0].State, "count") }},
		{"null message expectations", "expectedMessages", func(s *DeliveryScenario) { s.ExpectedMessages = nil }},
		{"duplicate message expectation", "expectedMessages[1].name", func(s *DeliveryScenario) { s.ExpectedMessages = append(s.ExpectedMessages, s.ExpectedMessages[0]) }},
		{"unknown message status", "expectedMessages[0].status", func(s *DeliveryScenario) { s.ExpectedMessages[0].Status = "delivered" }},
		{"negative attempts", "expectedMessages[0].attempts", func(s *DeliveryScenario) { s.ExpectedMessages[0].Attempts = -1 }},
		{"unsafe attempts", "expectedMessages[0].attempts", func(s *DeliveryScenario) { s.ExpectedMessages[0].Attempts = int(MaxBehaviorInteger + 1) }},
		{"acknowledged zero attempts", "expectedMessages[0].attempts", func(s *DeliveryScenario) { s.ExpectedMessages[0].Attempts = 0 }},
		{"pending acceptance", "expectedMessages[0].accepted", func(s *DeliveryScenario) { s.ExpectedMessages[0].Status = "pending" }},
		{"absent attempts", "expectedMessages[0]", func(s *DeliveryScenario) {
			s.ExpectedMessages[0].Status = "absent"
			s.ExpectedMessages[0].Accepted = nil
		}},
		{"undeclared acknowledged message", "expectedMessages[0].name", func(s *DeliveryScenario) { s.ExpectedMessages[0].Name = "missing" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := deliveryTestScenario()
			tc.mutate(&s)
			if issues := ValidateDeliveryScenario(lock, s); !issueAt(issues, tc.path) {
				t.Fatalf("want %s, got %+v", tc.path, issues)
			}
		})
	}
	scenario = deliveryTestScenario()
	scenario.Expected = []StateExpectation{}
	scenario.ExpectedMessages = []DeliveryMessageExpectation{{Name: "notDeclared", Status: "absent", Attempts: 0}}
	scenario.Steps = []DeliveryStep{{Kind: "restart", InstanceID: "source"}}
	if issues := ValidateDeliveryScenario(lock, scenario); len(issues) != 0 {
		t.Fatal("independent absent assertion rejected", issues)
	}
	scenario.ExpectedMessages = []DeliveryMessageExpectation{}
	if issues := ValidateDeliveryScenario(lock, scenario); len(issues) != 0 {
		t.Fatal("unverified scenario rejected", issues)
	}
	raw, _ := json.Marshal(scenario.Steps[0])
	if string(raw) != `{"kind":"restart","instanceId":"source","inputs":null}` {
		t.Fatal("canonical restart changed", string(raw))
	}
	stateScenario := stateTestScenario()
	stateScenario.Steps[0].CommandID = "msg_stillValidInState"
	if issues := ValidateStateScenario(lock.Model.Machines[0], stateScenario); len(issues) != 0 {
		t.Fatal("delivery ID reservation changed state scenarios", issues)
	}
}

func TestDeliveryStaticAliasesAndExplicitSourceMessageCycles(t *testing.T) {
	m := deliveryTestModel(t)
	m.Routes = append(m.Routes, DeliveryRoute{ID: "back", Name: "Return", SourceInstanceID: "target", Effect: "changed", TargetInstanceID: "source", Command: "set", Inputs: map[string]string{"value": "value"}})
	lock := deliveryTestLock(t, m)
	s := deliveryTestScenario()
	one := int64(1)
	s.Steps = append(s.Steps, DeliveryStep{Kind: "dispatch", RouteID: "back", SourceMessage: "firstMessage", Message: "secondMessage", ExpectedRevision: &one}, DeliveryStep{Kind: "deliver", Message: "secondMessage"})
	if issues := ValidateDeliveryScenario(lock, s); len(issues) != 0 {
		t.Fatal("explicit downstream cycle rejected", issues)
	}
	s.Steps[3].RouteID = "transfer"
	if issues := ValidateDeliveryScenario(lock, s); !issueAt(issues, "steps[3].sourceMessage") {
		t.Fatal("unrelated sourceMessage owner accepted", issues)
	}
	s = deliveryTestScenario()
	repeat := s.Steps[1]
	repeat.ExpectedRevision = &one
	s.Steps = append(s.Steps, repeat)
	if issues := ValidateDeliveryScenario(lock, s); len(issues) != 0 {
		t.Fatal("same alias/source with changed revision must be modeled at runtime", issues)
	}
	s.Steps[3].Message = "otherAlias"
	if issues := ValidateDeliveryScenario(lock, s); !issueAt(issues, "steps[3].message") {
		t.Fatal("one selector had multiple aliases", issues)
	}
	s = deliveryTestScenario()
	repeat = s.Steps[1]
	repeat.SourceCommandID = "different"
	s.Steps = append(s.Steps, repeat)
	if issues := ValidateDeliveryScenario(lock, s); !issueAt(issues, "steps[3].message") {
		t.Fatal("same alias changed selector", issues)
	}
	s = deliveryTestScenario()
	s.Steps[1].SourceCommandID = "notYetExecuted"
	if issues := ValidateDeliveryScenario(lock, s); len(issues) != 0 {
		t.Fatal("missing intent is a runtime outcome", issues)
	}
	s = deliveryTestScenario()
	s.Steps[0], s.Steps[2] = s.Steps[2], s.Steps[0]
	if issues := ValidateDeliveryScenario(lock, s); !issueAt(issues, "steps[0].message") {
		t.Fatal("future deliver alias accepted", issues)
	}
}

func TestDeliveryMessageExpectationStrictJSON(t *testing.T) {
	for _, raw := range []string{`null`, `{"name":"missing","status":"absent"}`, `{"name":"missing","status":"absent","attempts":null}`, `{"name":"missing","status":"absent","attempts":0.5}`, `{"name":"missing","status":"absent","attempts":"0"}`, `{"name":"missing","status":"absent","attempts":false}`, `{"name":"missing","status":"absent","attempts":0,"unknown":1}`} {
		var value DeliveryMessageExpectation
		if err := json.Unmarshal([]byte(raw), &value); err == nil {
			t.Fatal("invalid expected attempts accepted", raw)
		}
	}
	for _, raw := range []string{`{"name":"missing","status":"absent","attempts":0}`, `{"NAME":"missing","STATUS":"absent","ATTEMPTS":0}`} {
		var value DeliveryMessageExpectation
		if err := json.Unmarshal([]byte(raw), &value); err != nil {
			t.Fatal(err)
		}
		encoded, _ := json.Marshal(value)
		if string(encoded) != `{"name":"missing","status":"absent","attempts":0}` {
			t.Fatal("canonical zero changed", string(encoded))
		}
	}
	lock := deliveryTestLock(t, deliveryTestModel(t))
	s := deliveryTestScenario()
	raw := `{"name":"firstMessage","status":"acknowledged","attempts":9007199254740992}`
	if err := json.Unmarshal([]byte(raw), &s.ExpectedMessages[0]); err != nil {
		t.Fatal(err)
	}
	if issues := ValidateDeliveryScenario(lock, s); !issueAt(issues, "expectedMessages[0].attempts") {
		t.Fatal("unsafe raw expectation accepted", issues)
	}
}

func TestDeliveryScenarioBoundsBeforeEncoding(t *testing.T) {
	data, large, small := stateTestLargeRecord(t)
	machine := stateTestMachine(t)
	machine.DataContracts = []DataContractLock{data}
	machine.Commands[0].Inputs = []BehaviorPort{{Name: "payload", Type: "record", RecordRef: large.RecordRef}}
	value := behaviorInt(1)
	machine.Commands[0].Bindings = map[string]StateBinding{"value": {Literal: &value}}
	m := deliveryTestModel(t)
	m.Machines = []StateMachineLock{stateTestMachineLock(t, machine)}
	for i := range m.Instances {
		m.Instances[i].Machine.Digest = m.Machines[0].Digest
	}
	m.Routes = []DeliveryRoute{}
	lock := deliveryTestLock(t, m)
	s := deliveryTestScenario()
	s.Expected = []StateExpectation{}
	s.ExpectedMessages = []DeliveryMessageExpectation{}
	s.Steps = []DeliveryStep{}
	for i := 0; i < 14; i++ {
		revision := int64(i)
		s.Steps = append(s.Steps, DeliveryStep{Kind: "command", InstanceID: "source", CommandID: fmt.Sprintf("request%d", i), Command: "set", ExpectedRevision: &revision, Inputs: map[string]TypedValue{"payload": large}})
	}
	if len(ValidateDeliveryScenario(lock, s)) == 0 {
		t.Fatal("accepted >512KiB scenario")
	}
	cyclic := TypedValue{Type: "record", RecordRef: large.RecordRef, Record: map[string]TypedValue{}}
	cyclic.Record["self"] = cyclic
	s.Steps = s.Steps[:1]
	s.Steps[0].Inputs = map[string]TypedValue{"payload": cyclic}
	if len(ValidateDeliveryScenario(lock, s)) == 0 {
		t.Fatal("accepted recursive typed value")
	}
	machine.State = []StateSlot{{Name: "first", Type: "record", RecordRef: large.RecordRef, Durability: "durable", Initial: large}, {Name: "second", Type: "record", RecordRef: small.RecordRef, Durability: "volatile", Initial: small}}
	machine.Commands[0].Writes = map[string]string{}
	m.Machines = []StateMachineLock{stateTestMachineLock(t, machine)}
	for i := range m.Instances {
		m.Instances[i].Machine.Digest = m.Machines[0].Digest
	}
	lock = deliveryTestLock(t, m)
	s.Steps = []DeliveryStep{{Kind: "restart", InstanceID: "source"}}
	s.Initial[0].Initial = map[string]TypedValue{"second": large}
	if issues := ValidateDeliveryScenario(lock, s); !issueAt(issues, "initial[0].initial") {
		t.Fatal("oversized merged effective state accepted", issues)
	}
}

func TestDeliveryNominalRoutingRejectsEqualRecordShapes(t *testing.T) {
	data, _ := structuredTestRecord()
	build := func(id string, contract DataContractLock) StateMachineLock {
		ref := &DataTypeRef{ContractID: contract.Contract.ID, Version: contract.Contract.Version, Digest: contract.Digest, Record: "Subject"}
		p := compositeTestPackage(id + "-decision")
		p.SchemaVersion = 3
		p.DataContracts = []DataContractLock{contract}
		p.Behavior.Inputs = []BehaviorPort{{Name: "value", Type: "record", RecordRef: ref}}
		p.Behavior.Outputs = []BehaviorPort{{Name: "accepted", Type: "boolean"}, {Name: "value", Type: "record", RecordRef: ref}}
		yes := behaviorBool(true)
		p.Behavior.Bindings = map[string]BehaviorBinding{"accepted": {Literal: &yes}, "value": {Input: "value"}}
		bundle, issues := PrepareComponentBundle(p, []PackageLock{})
		if len(issues) != 0 {
			t.Fatal(issues)
		}
		machine := stateTestMachine(t)
		machine.ID = id
		machine.Effects[0].Payload = []BehaviorPort{{Name: "value", Type: "record", RecordRef: ref}}
		c := &machine.Commands[0]
		c.Decision = bundle
		c.Inputs = []BehaviorPort{{Name: "value", Type: "record", RecordRef: ref}}
		c.Outputs = []BehaviorPort{{Name: "value", Type: "record", RecordRef: ref}}
		c.Writes = map[string]string{}
		return stateTestMachineLock(t, machine)
	}
	first := build("test.record-source", data)
	other := data
	other.Contract.ID = "test.equal-shaped"
	other.Digest, _ = DataContractDigest(other.Contract)
	second := build("test.record-target", other)
	m := deliveryTestModel(t)
	m.Machines = []StateMachineLock{first, second}
	for i, lock := range m.Machines {
		m.Instances[i].Machine = DeliveryMachineRef{MachineID: lock.Machine.ID, Version: lock.Machine.Version, Digest: lock.Digest}
	}
	if issues := ValidateDeliveryModel(m); !issueAt(issues, "routes[0].inputs.value") {
		t.Fatal("same-shaped foreign nominal record accepted", issues)
	}
	second = build("test.record-target", data)
	m.Machines[1] = second
	m.Instances[1].Machine.Digest = second.Digest
	if issues := ValidateDeliveryModel(m); len(issues) != 0 {
		t.Fatal("same exact shared nominal record rejected", issues)
	}
}

func TestDeliveryAggregateModelBudgetCountsUnusedMachines(t *testing.T) {
	machine := stateTestMachine(t)
	p := machine.Commands[0].Decision.Packages[0].Package
	p.Behavior.Inputs = []BehaviorPort{{Name: "first", Type: "string"}, {Name: "second", Type: "string"}}
	value := behaviorInt(1)
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
	machine.Commands[0].Decision = bundle
	machine.Commands[0].Bindings = map[string]StateBinding{"first": {Literal: &large}, "second": {Literal: &large}}
	base := machine.Commands[0]
	for i := 1; i < 4; i++ {
		c := base
		c.Name = fmt.Sprintf("set%d", i)
		machine.Commands = append(machine.Commands, c)
	}
	m := deliveryTestModel(t)
	m.Machines = []StateMachineLock{}
	for i := 0; i < 4; i++ {
		owner := machine
		owner.ID = fmt.Sprintf("test.large%d", i)
		m.Machines = append(m.Machines, stateTestMachineLock(t, owner))
	}
	for i := range m.Instances {
		m.Instances[i].Machine = DeliveryMachineRef{MachineID: m.Machines[0].Machine.ID, Version: m.Machines[0].Machine.Version, Digest: m.Machines[0].Digest}
	}
	if contracts, issues := DeliveryModelDataContracts(m); len(issues) != 0 || len(contracts) != 0 {
		t.Fatal("repeated exact dependencies should be legal", issues)
	}
	if issues := ValidateDeliveryModel(m); !issueAt(issues, "") {
		t.Fatal("accepted >1MiB including valid unused machines", issues)
	}
}
