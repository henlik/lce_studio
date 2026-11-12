package model

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
)

func structuredTestRecord() (DataContractLock, TypedValue) {
	example := ExampleDataFixtures()[0]
	ref := &DataTypeRef{ContractID: example.Contract.Contract.ID, Version: example.Contract.Contract.Version, Digest: example.Contract.Digest, Record: "Subject"}
	return example.Contract, TypedValue{Type: "record", RecordRef: ref, Record: example.Fixture.Rows["Subjects"][0]}
}

func structuredTestCollection(v TypedValue, items ...TypedValue) TypedValue {
	if items == nil {
		items = []TypedValue{}
	}
	ref := *v.RecordRef
	return TypedValue{Type: "collection", RecordRef: &ref, Collection: &items}
}

func TestStructuredBridgePreservesLegacyDigests(t *testing.T) {
	want := map[string]string{
		"example.health-record-access": "0e0ce2992270fefe6bb87abf994f3a3f02c9ff33386899212c83a0fed79545f9",
		"example.school-record-access": "93d9cd77db39c746cc627caa916b4ff819fa5f51ef9718ce9ef52f45528a0554",
		"lce.access-policy":            "c3ea0e265eda646913c821beb717b099c6cdc4fbc4bc018ef43e3f744fd72ee6",
		"lce.same-tenant":              "122757d94e9c45a2376d28d9d415312e18df91766376a2b9366ba9d23efb9888",
	}
	for _, bundle := range ExampleComponentBundles() {
		for _, lock := range bundle.Packages {
			got, err := ComponentDigest(lock.Package)
			if err != nil || got != want[lock.Package.ID] || got != lock.Digest {
				t.Fatalf("legacy digest changed for %s: %s, %v", lock.Package.ID, got, err)
			}
		}
	}
	if len(PrimitiveSpecs()) != 11 || len(StructuredPrimitiveSpecs()) != 16 {
		t.Fatal("primitive discovery compatibility changed")
	}
}

func TestStructuredExamplesPinUnchangedPolicyAndContract(t *testing.T) {
	examples := ExampleStructuredComponentBundles()
	for i, name := range []string{"structured-health-access", "structured-school-access"} {
		if issues := ValidateComponentBundle(examples[i]); len(issues) != 0 {
			t.Fatal(issues)
		}
		data, err := os.ReadFile("../../examples/composites/" + name + ".json")
		if err != nil {
			t.Fatal(err)
		}
		var imported ComponentBundle
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&imported); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(imported, examples[i]) {
			t.Fatalf("%s fixture differs from helper", name)
		}
		legacy := ExampleComponentBundles()[i]
		if !reflect.DeepEqual(imported.Packages[1:], legacy.Packages[1:]) {
			t.Fatal("scalar dependencies changed")
		}
		if len(imported.Packages[0].Package.Tests) != 5 {
			t.Fatal("missing domain cases")
		}
	}
	if !reflect.DeepEqual(examples[0].Packages[0].Package.DataContracts, examples[1].Packages[0].Package.DataContracts) {
		t.Fatal("domains must share the exact contract")
	}
	examples[0].Packages[0].Package.Tests[0].Inputs["subject"].Record["role"] = behaviorString("edited")
	if got := *examples[1].Packages[0].Package.Tests[0].Inputs["subject"].Record["role"].String; got != "teacher" {
		t.Fatal("examples share mutable field maps")
	}
}

func TestStructuredValuesAreClosedFlatAndNominal(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*TypedValue)
	}{
		{"missing field", func(v *TypedValue) { delete(v.Record, "role") }},
		{"extra field", func(v *TypedValue) { v.Record["extra"] = behaviorBool(true) }},
		{"wrong scalar", func(v *TypedValue) { v.Record["active"] = behaviorString("true") }},
		{"null field", func(v *TypedValue) { v.Record["active"] = TypedValue{Type: "boolean"} }},
		{"null record", func(v *TypedValue) { v.Record = nil }},
		{"scalar member", func(v *TypedValue) { b := true; v.Boolean = &b }},
		{"collection member", func(v *TypedValue) { items := []TypedValue{}; v.Collection = &items }},
		{"missing reference", func(v *TypedValue) { v.RecordRef = nil }},
		{"unknown record", func(v *TypedValue) { v.RecordRef.Record = "Missing" }},
		{"wrong digest", func(v *TypedValue) { v.RecordRef.Digest = strings.Repeat("a", 64) }},
		{"nested record", func(v *TypedValue) {
			v.Record["active"] = TypedValue{Type: "record", RecordRef: v.RecordRef, Record: map[string]TypedValue{"x": behaviorBool(true)}}
		}},
		{"nested collection", func(v *TypedValue) { v.Record["active"] = structuredTestCollection(*v) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lock, v := structuredTestRecord()
			tc.mutate(&v)
			if issues := ValidateStructuredTypedValue(v, []DataContractLock{lock}); len(issues) == 0 {
				t.Fatal("accepted malformed record")
			}
		})
	}
	lock, v := structuredTestRecord()
	if issues := ValidateStructuredTypedValue(v, []DataContractLock{lock}); len(issues) != 0 {
		t.Fatal(issues)
	}
	if len(ValidateTypedValue(v)) == 0 {
		t.Fatal("scalar validator accepts records")
	}
	if issues := ValidateStructuredTypedValue(structuredTestCollection(v), []DataContractLock{lock}); len(issues) != 0 {
		t.Fatal("empty [] collection rejected", issues)
	}
	for _, tc := range []struct {
		name   string
		mutate func(*TypedValue)
	}{
		{"null array", func(v *TypedValue) { var items []TypedValue; v.Collection = &items }},
		{"missing array", func(v *TypedValue) { v.Collection = nil }},
		{"scalar item", func(v *TypedValue) { (*v.Collection)[0] = behaviorBool(false) }},
		{"nested list", func(v *TypedValue) { (*v.Collection)[0] = structuredTestCollection((*v.Collection)[0]) }},
		{"wrong item ref", func(v *TypedValue) {
			ref := *(*v.Collection)[0].RecordRef
			ref.Record = "Resource"
			(*v.Collection)[0].RecordRef = &ref
		}},
		{"too many", func(v *TypedValue) {
			items := make([]TypedValue, 129)
			for i := range items {
				items[i] = (*v.Collection)[0]
			}
			v.Collection = &items
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lock, record := structuredTestRecord()
			collection := structuredTestCollection(record, record)
			tc.mutate(&collection)
			if len(ValidateStructuredTypedValue(collection, []DataContractLock{lock})) == 0 {
				t.Fatal("accepted malformed collection")
			}
		})
	}
	other := *v.RecordRef
	other.Version = "2.0.0"
	if SameBehaviorType(ValueBehaviorType(v), BehaviorValueType{Type: "record", RecordRef: &other}) {
		t.Fatal("nominal version erased")
	}
	if len(ValidateStructuredBehaviorValues(map[string]TypedValue{"value": v}, []BehaviorPort{{Name: "value", Type: "record", RecordRef: &other}}, []DataContractLock{lock})) == 0 {
		t.Fatal("nominal port mismatch accepted")
	}
}

func TestStructuredValueBudgets(t *testing.T) {
	lock, record := structuredTestRecord()
	lock.Contract.Records = []DataRecord{{Name: "Large", Fields: []DataField{}}}
	lock.Contract.Entities = []DataEntity{}
	lock.Contract.Relationships = []DataRelationship{}
	record.Record = map[string]TypedValue{}
	for i := 0; i < 16; i++ {
		name := fmt.Sprintf("field%d", i)
		lock.Contract.Records[0].Fields = append(lock.Contract.Records[0].Fields, DataField{Name: name, Type: "string"})
		record.Record[name] = behaviorString(strings.Repeat("x", 4096))
	}
	lock.Digest, _ = DataContractDigest(lock.Contract)
	record.RecordRef = &DataTypeRef{ContractID: lock.Contract.ID, Version: lock.Contract.Version, Digest: lock.Digest, Record: "Large"}
	if len(ValidateStructuredTypedValue(record, []DataContractLock{lock})) == 0 {
		t.Fatal("accepted >64KiB record")
	}
	for name := range record.Record {
		record.Record[name] = behaviorString(strings.Repeat("x", 3000))
	}
	if issues := ValidateStructuredTypedValue(record, []DataContractLock{lock}); len(issues) != 0 {
		t.Fatal(issues)
	}
	items := make([]TypedValue, 128)
	for i := range items {
		items[i] = record
	}
	if len(ValidateStructuredTypedValue(structuredTestCollection(record, items...), []DataContractLock{lock})) == 0 {
		t.Fatal("accepted oversized collection")
	}
	values := map[string]TypedValue{}
	for i := 0; i < 6; i++ {
		values[fmt.Sprintf("value%d", i)] = record
	}
	if len(ValidateStructuredValueMap(values, []DataContractLock{lock})) == 0 {
		t.Fatal("accepted >256KiB map")
	}
	values = map[string]TypedValue{}
	for i := 0; i < 17; i++ {
		values[fmt.Sprintf("value%d", i)] = behaviorBool(true)
	}
	if len(ValidateStructuredValueMap(values, nil)) == 0 {
		t.Fatal("accepted >16 values")
	}
	// Reject nested/cyclic values by shape before attempting recursive encoding.
	cyclic := TypedValue{Type: "record", RecordRef: record.RecordRef, Record: map[string]TypedValue{}}
	cyclic.Record["field0"] = cyclic
	if len(ValidateStructuredTypedValue(cyclic, []DataContractLock{lock})) == 0 {
		t.Fatal("accepted cyclic Go value")
	}
}

func TestStructuredPrimitiveSignatures(t *testing.T) {
	lock, v := structuredTestRecord()
	locks := []DataContractLock{lock}
	record := ValueBehaviorType(v)
	collection := BehaviorValueType{Type: "collection", RecordRef: v.RecordRef}
	boolean := BehaviorValueType{Type: "boolean"}
	integer := BehaviorValueType{Type: "integer"}
	str := BehaviorValueType{Type: "string"}
	for _, tc := range []struct {
		name   string
		node   BehaviorNode
		inputs map[string]BehaviorValueType
		want   BehaviorValueType
	}{
		{"make", BehaviorNode{Op: "record_make", RecordRef: v.RecordRef}, map[string]BehaviorValueType{"id": str, "tenantId": str, "role": str, "active": boolean}, record},
		{"get", BehaviorNode{Op: "record_get", Field: "active"}, map[string]BehaviorValueType{"value": record}, boolean},
		{"count", BehaviorNode{Op: "collection_count"}, map[string]BehaviorValueType{"values": collection}, integer},
		{"at", BehaviorNode{Op: "collection_at"}, map[string]BehaviorValueType{"values": collection, "index": integer}, record},
		{"append", BehaviorNode{Op: "collection_append"}, map[string]BehaviorValueType{"values": collection, "value": record}, collection},
		{"identity", BehaviorNode{Op: "identity"}, map[string]BehaviorValueType{"value": collection}, collection},
		{"equal", BehaviorNode{Op: "equal"}, map[string]BehaviorValueType{"left": record, "right": record}, boolean},
		{"select", BehaviorNode{Op: "select"}, map[string]BehaviorValueType{"condition": boolean, "trueValue": record, "falseValue": record}, record},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := StructuredPrimitiveOutputType(tc.node, tc.inputs, locks)
			if err != nil || !SameBehaviorType(got, tc.want) {
				t.Fatalf("%+v %v", got, err)
			}
		})
	}
	other := *v.RecordRef
	other.Record = "Resource"
	otherRecord := BehaviorValueType{Type: "record", RecordRef: &other}
	for _, tc := range []struct {
		node   BehaviorNode
		inputs map[string]BehaviorValueType
	}{
		{BehaviorNode{Op: "record_make"}, map[string]BehaviorValueType{}},
		{BehaviorNode{Op: "record_make", RecordRef: v.RecordRef}, map[string]BehaviorValueType{"id": str}},
		{BehaviorNode{Op: "record_get", Field: "missing"}, map[string]BehaviorValueType{"value": record}},
		{BehaviorNode{Op: "record_get", Field: "active", RecordRef: v.RecordRef}, map[string]BehaviorValueType{"value": record}},
		{BehaviorNode{Op: "identity", Field: "active"}, map[string]BehaviorValueType{"value": record}},
		{BehaviorNode{Op: "collection_at"}, map[string]BehaviorValueType{"values": collection, "index": str}},
		{BehaviorNode{Op: "collection_append"}, map[string]BehaviorValueType{"values": collection, "value": otherRecord}},
		{BehaviorNode{Op: "equal"}, map[string]BehaviorValueType{"left": record, "right": otherRecord}},
		{BehaviorNode{Op: "select"}, map[string]BehaviorValueType{"condition": boolean, "trueValue": record, "falseValue": otherRecord}},
	} {
		if _, err := StructuredPrimitiveOutputType(tc.node, tc.inputs, locks); err == nil {
			t.Fatalf("accepted incompatible signature %+v", tc.node)
		}
	}
}

func TestSchemaTwoRejectsStructuredExtensions(t *testing.T) {
	lock, v := structuredTestRecord()
	for _, mutate := range []func(*ComponentPackage){
		func(p *ComponentPackage) { p.DataContracts = []DataContractLock{} },
		func(p *ComponentPackage) { p.DataContracts = []DataContractLock{lock} },
		func(p *ComponentPackage) { p.Behavior.Inputs[0].RecordRef = v.RecordRef },
		func(p *ComponentPackage) {
			p.Behavior.Inputs[0].Type = "record"
			p.Behavior.Inputs[0].RecordRef = v.RecordRef
		},
		func(p *ComponentPackage) { p.Behavior.Bindings["result"] = BehaviorBinding{Literal: &v} },
		func(p *ComponentPackage) {
			p.Behavior.Nodes = []BehaviorNode{{ID: "newOp", Op: "record_get", Field: "active", Inputs: map[string]BehaviorBinding{"value": {Input: "value"}}}}
		},
		func(p *ComponentPackage) {
			p.Behavior.Nodes = []BehaviorNode{{ID: "metadata", Op: "identity", RecordRef: v.RecordRef, Inputs: map[string]BehaviorBinding{"value": {Input: "value"}}}}
		},
	} {
		p := compositeTestPackage("test.legacy")
		mutate(&p)
		if len(ValidateComponentPackage(p)) == 0 {
			t.Fatal("schema2 accepted structured extension")
		}
	}
}

func TestStructuredBundleResolvesEmbeddedUnion(t *testing.T) {
	lock, v := structuredTestRecord()
	child := compositeTestPackage("test.record-child")
	child.SchemaVersion = 3
	child.DataContracts = []DataContractLock{lock}
	child.Behavior.Inputs = []BehaviorPort{}
	child.Behavior.Outputs = []BehaviorPort{{Name: "result", Type: "record", RecordRef: v.RecordRef}}
	child.Behavior.Bindings = map[string]BehaviorBinding{"result": {Literal: &v}}
	childLock := compositeTestLock(t, child)
	childRef := compositeTestRef(childLock)
	parent := compositeTestPackage("test.record-parent")
	parent.SchemaVersion = 3
	parent.Behavior.Inputs = []BehaviorPort{}
	parent.Behavior.Nodes = []BehaviorNode{
		{ID: "child", Component: &childRef, Inputs: map[string]BehaviorBinding{}},
		{ID: "read", Op: "record_get", Field: "active", Inputs: map[string]BehaviorBinding{"value": {Node: "child", Port: "result"}}},
	}
	parent.Behavior.Bindings = map[string]BehaviorBinding{"result": {Node: "read", Port: "result"}}
	bundle, issues := PrepareComponentBundle(parent, []PackageLock{childLock})
	if len(issues) != 0 {
		t.Fatal("callee contract was not available to parent", issues)
	}
	contracts, issues := BundleDataContracts(bundle)
	if len(issues) != 0 || len(contracts) != 1 {
		t.Fatal(contracts, issues)
	}
	parent.DataContracts = []DataContractLock{lock}
	if _, issues = PrepareComponentBundle(parent, []PackageLock{childLock}); len(issues) != 0 {
		t.Fatal("identical locks across manifests rejected", issues)
	}
	conflict := lock
	conflict.Contract.Name += " changed"
	conflict.Digest, _ = DataContractDigest(conflict.Contract)
	parent.DataContracts = []DataContractLock{conflict}
	if _, issues = PrepareComponentBundle(parent, []PackageLock{childLock}); len(issues) == 0 {
		t.Fatal("conflicting data contract identity accepted")
	}
	child.DataContracts = nil
	if issues = ValidateComponentPackage(child); len(issues) != 0 {
		t.Fatal("local manifest must defer absent contract refs", issues)
	}
	if _, issues = PrepareComponentBundle(child, nil); len(issues) == 0 {
		t.Fatal("unresolved union reference accepted")
	}
	child.DataContracts = []DataContractLock{lock}
	wrong := *v.RecordRef
	wrong.Record = "Missing"
	child.Behavior.Outputs[0].RecordRef = &wrong
	if issues = ValidateComponentPackage(child); len(issues) == 0 {
		t.Fatal("locally resolvable invalid record accepted")
	}
}

func TestStructuredBundleUnionBound(t *testing.T) {
	base, _ := structuredTestRecord()
	bundle := ComponentBundle{Packages: []PackageLock{}}
	for i := 0; i < 5; i++ {
		p := compositeTestPackage(fmt.Sprintf("test.package%d", i))
		p.SchemaVersion = 3
		for j := 0; j < 7; j++ {
			lock := base
			lock.Contract.ID = fmt.Sprintf("test.contract%d", i*7+j)
			lock.Digest, _ = DataContractDigest(lock.Contract)
			p.DataContracts = append(p.DataContracts, lock)
		}
		bundle.Packages = append(bundle.Packages, PackageLock{Package: p})
	}
	if _, issues := BundleDataContracts(bundle); len(issues) == 0 {
		t.Fatal("accepted >32 unique embedded contracts")
	}
}

func TestStructuredRequiredNullAndLegacyOptionalNullNormalization(t *testing.T) {
	lock, record := structuredTestRecord()
	for _, tc := range []struct {
		name   string
		value  TypedValue
		mutate func(map[string]any)
	}{
		{"record reference", record, func(v map[string]any) { v["recordRef"] = nil }},
		{"record object", record, func(v map[string]any) { v["record"] = nil }},
		{"record field", record, func(v map[string]any) { v["record"].(map[string]any)["active"] = nil }},
		{"scalar member", record, func(v map[string]any) { v["record"].(map[string]any)["active"].(map[string]any)["boolean"] = nil }},
		{"collection array", structuredTestCollection(record), func(v map[string]any) { v["collection"] = nil }},
		{"collection reference", structuredTestCollection(record), func(v map[string]any) { v["recordRef"] = nil }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			encoded, _ := json.Marshal(tc.value)
			var raw map[string]any
			if err := json.Unmarshal(encoded, &raw); err != nil {
				t.Fatal(err)
			}
			tc.mutate(raw)
			encoded, _ = json.Marshal(raw)
			var imported TypedValue
			if err := json.Unmarshal(encoded, &imported); err != nil {
				t.Fatal(err)
			}
			if len(ValidateStructuredTypedValue(imported, []DataContractLock{lock})) == 0 {
				t.Fatal("required null accepted")
			}
		})
	}
	var addOptionalNulls func(any)
	addOptionalNulls = func(value any) {
		switch v := value.(type) {
		case []any:
			for _, child := range v {
				addOptionalNulls(child)
			}
		case map[string]any:
			if _, ok := v["schemaVersion"]; ok {
				v["dataContracts"] = nil
			}
			if _, ok := v["type"]; ok {
				v["recordRef"] = nil
				if _, isPort := v["name"]; !isPort {
					v["record"], v["collection"] = nil, nil
				}
			}
			if _, ok := v["position"]; ok {
				v["recordRef"], v["field"] = nil, nil
			}
			for _, child := range v {
				addOptionalNulls(child)
			}
		}
	}
	for _, bundle := range ExampleComponentBundles() {
		for _, legacy := range bundle.Packages {
			encoded, _ := json.Marshal(legacy.Package)
			var raw any
			if err := json.Unmarshal(encoded, &raw); err != nil {
				t.Fatal(err)
			}
			addOptionalNulls(raw)
			encoded, _ = json.Marshal(raw)
			var imported ComponentPackage
			if err := json.Unmarshal(encoded, &imported); err != nil {
				t.Fatal(err)
			}
			if issues := ValidateComponentPackage(imported); len(issues) != 0 {
				t.Fatal(issues)
			}
			got, err := ComponentDigest(imported)
			if err != nil || got != legacy.Digest {
				t.Fatalf("optional null changed legacy identity: %s %v", got, err)
			}
		}
	}
}

func TestSchemaTwoCannotTransportRecordsThroughCalls(t *testing.T) {
	lock, value := structuredTestRecord()
	producer := compositeTestPackage("test.producer")
	producer.SchemaVersion = 3
	producer.DataContracts = []DataContractLock{lock}
	producer.Behavior.Inputs = []BehaviorPort{}
	producer.Behavior.Outputs = []BehaviorPort{{Name: "result", Type: "record", RecordRef: value.RecordRef}}
	producer.Behavior.Bindings = map[string]BehaviorBinding{"result": {Literal: &value}}
	produceLock := compositeTestLock(t, producer)
	produceRef := compositeTestRef(produceLock)
	consumer := compositeTestPackage("test.consumer")
	consumer.SchemaVersion = 3
	consumer.Behavior.Inputs = []BehaviorPort{{Name: "value", Type: "record", RecordRef: value.RecordRef}}
	consumer.Behavior.Nodes = []BehaviorNode{{ID: "get", Op: "record_get", Field: "active", Inputs: map[string]BehaviorBinding{"value": {Input: "value"}}}}
	consumer.Behavior.Bindings = map[string]BehaviorBinding{"result": {Node: "get", Port: "result"}}
	consumeLock := compositeTestLock(t, consumer)
	consumeRef := compositeTestRef(consumeLock)
	parent := compositeTestPackage("test.bridge")
	parent.Behavior.Inputs = []BehaviorPort{}
	parent.Behavior.Nodes = []BehaviorNode{
		{ID: "produce", Component: &produceRef, Inputs: map[string]BehaviorBinding{}},
		{ID: "consume", Component: &consumeRef, Inputs: map[string]BehaviorBinding{"value": {Node: "produce", Port: "result"}}},
	}
	parent.Behavior.Bindings = map[string]BehaviorBinding{"result": {Node: "consume", Port: "result"}}
	dependencies := []PackageLock{produceLock, consumeLock}
	if _, issues := PrepareComponentBundle(parent, dependencies); len(issues) == 0 {
		t.Fatal("schema2 internally transported a structured value")
	}
	parent.SchemaVersion = 3
	if _, issues := PrepareComponentBundle(parent, dependencies); len(issues) != 0 {
		t.Fatal("schema3 cannot transport structured values", issues)
	}
}
