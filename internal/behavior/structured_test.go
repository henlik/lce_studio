package behavior

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"software-studio/internal/model"
)

func structuredLock(t testing.TB) model.DataContractLock {
	t.Helper()
	contract := model.DataContract{SchemaVersion: 1, ID: "test.inventory-data", Version: "1.0.0", Name: "Inventory data", Author: "Test author", License: "MIT",
		Records:  []model.DataRecord{{Name: "Item", Fields: []model.DataField{{Name: "id", Type: "string"}, {Name: "quantity", Type: "integer"}, {Name: "active", Type: "boolean"}}}},
		Entities: []model.DataEntity{}, Relationships: []model.DataRelationship{}}
	digest, err := model.DataContractDigest(contract)
	if err != nil {
		t.Fatal(err)
	}
	lock := model.DataContractLock{Contract: contract, Digest: digest}
	if issues := model.ValidateDataContractLock(lock); len(issues) > 0 {
		t.Fatal(issues)
	}
	return lock
}
func dataReference(lock model.DataContractLock) *model.DataTypeRef {
	return &model.DataTypeRef{ContractID: lock.Contract.ID, Version: lock.Contract.Version, Digest: lock.Digest, Record: "Item"}
}
func item(lock model.DataContractLock, id string, quantity int64, active bool) model.TypedValue {
	return model.TypedValue{Type: "record", RecordRef: dataReference(lock), Record: map[string]model.TypedValue{"id": textValue(id), "quantity": integer(quantity), "active": boolean(active)}}
}
func collection(lock model.DataContractLock, values ...model.TypedValue) model.TypedValue {
	if values == nil {
		values = []model.TypedValue{}
	}
	return model.TypedValue{Type: "collection", RecordRef: dataReference(lock), Collection: &values}
}
func structuredDefinition(node model.BehaviorNode, inputs map[string]model.TypedValue, resultType string, lock model.DataContractLock) model.ComponentPackage {
	ports := make([]model.BehaviorPort, 0, len(inputs))
	node.ID = "operation"
	node.Inputs = map[string]model.BehaviorBinding{}
	for _, name := range sortedKeys(inputs) {
		value := inputs[name]
		ports = append(ports, model.BehaviorPort{Name: name, Type: value.Type, RecordRef: value.RecordRef})
		node.Inputs[name] = input(name)
	}
	out := model.BehaviorPort{Name: "result", Type: resultType}
	if resultType == "record" || resultType == "collection" {
		out.RecordRef = dataReference(lock)
	}
	p := definition("test.structured", model.Behavior{Inputs: ports, Outputs: []model.BehaviorPort{out}, Nodes: []model.BehaviorNode{node}, Bindings: map[string]model.BehaviorBinding{"result": output(node.ID)}})
	p.SchemaVersion = 3
	p.DataContracts = []model.DataContractLock{lock}
	return p
}

func TestStructuredPrimitivesAndExactValues(t *testing.T) {
	lock := structuredLock(t)
	zero := item(lock, "", 0, false)
	first := item(lock, "a", 12, true)
	second := item(lock, "b", -3, false)
	// Build an independent map with a different insertion order.
	reordered := model.TypedValue{Type: "record", RecordRef: dataReference(lock), Record: map[string]model.TypedValue{}}
	reordered.Record["quantity"] = integer(12)
	reordered.Record["active"] = boolean(true)
	reordered.Record["id"] = textValue("a")
	for _, tc := range []struct {
		name   string
		node   model.BehaviorNode
		inputs map[string]model.TypedValue
		want   model.TypedValue
	}{
		{"record_make_false_zero_empty", model.BehaviorNode{Op: "record_make", RecordRef: dataReference(lock)}, map[string]model.TypedValue{"id": textValue(""), "quantity": integer(0), "active": boolean(false)}, zero},
		{"record_get_integer", model.BehaviorNode{Op: "record_get", Field: "quantity"}, map[string]model.TypedValue{"value": zero}, integer(0)},
		{"record_get_boolean", model.BehaviorNode{Op: "record_get", Field: "active"}, map[string]model.TypedValue{"value": zero}, boolean(false)},
		{"record_get_empty_string", model.BehaviorNode{Op: "record_get", Field: "id"}, map[string]model.TypedValue{"value": zero}, textValue("")},
		{"collection_count_empty", model.BehaviorNode{Op: "collection_count"}, map[string]model.TypedValue{"values": collection(lock)}, integer(0)},
		{"collection_count_two", model.BehaviorNode{Op: "collection_count"}, map[string]model.TypedValue{"values": collection(lock, first, second)}, integer(2)},
		{"collection_at_second", model.BehaviorNode{Op: "collection_at"}, map[string]model.TypedValue{"values": collection(lock, first, second), "index": integer(1)}, second},
		{"collection_append_empty", model.BehaviorNode{Op: "collection_append"}, map[string]model.TypedValue{"values": collection(lock), "value": first}, collection(lock, first)},
		{"collection_append_order", model.BehaviorNode{Op: "collection_append"}, map[string]model.TypedValue{"values": collection(lock, first), "value": second}, collection(lock, first, second)},
		{"identity_record", model.BehaviorNode{Op: "identity"}, map[string]model.TypedValue{"value": zero}, zero},
		{"identity_collection", model.BehaviorNode{Op: "identity"}, map[string]model.TypedValue{"value": collection(lock, first, second)}, collection(lock, first, second)},
		{"equal_map_order", model.BehaviorNode{Op: "equal"}, map[string]model.TypedValue{"left": first, "right": reordered}, boolean(true)},
		{"equal_collection_order", model.BehaviorNode{Op: "equal"}, map[string]model.TypedValue{"left": collection(lock, first, second), "right": collection(lock, second, first)}, boolean(false)},
		{"select_record", model.BehaviorNode{Op: "select"}, map[string]model.TypedValue{"condition": boolean(false), "trueValue": first, "falseValue": second}, second},
		{"select_collection", model.BehaviorNode{Op: "select"}, map[string]model.TypedValue{"condition": boolean(true), "trueValue": collection(lock, first), "falseValue": collection(lock)}, collection(lock, first)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := structuredDefinition(tc.node, tc.inputs, tc.want.Type, lock)
			got := evaluateOK(t, bundleDefinition(t, p), tc.inputs)
			if !reflect.DeepEqual(got.Outputs["result"], tc.want) || got.Operations != 1 || got.Invocations != 1 {
				t.Fatalf("got %+v; want %+v", got.Outputs, tc.want)
			}
		})
	}
}

func TestStructuredEvidenceHasNoMutableAliases(t *testing.T) {
	lock := structuredLock(t)
	values := map[string]model.TypedValue{"values": collection(lock, item(lock, "a", 1, true)), "value": item(lock, "b", 2, false)}
	p := structuredDefinition(model.BehaviorNode{Op: "collection_append"}, values, "collection", lock)
	b := bundleDefinition(t, p)
	beforeBundle, _ := json.Marshal(b)
	beforeInputs, _ := json.Marshal(values)
	result := evaluateOK(t, b, values)
	if len(*values["values"].Collection) != 1 {
		t.Fatal("append mutated input collection")
	}
	result.Outputs["result"].RecordRef.Record = "Changed"
	*(*result.Outputs["result"].Collection)[0].Record["quantity"].Integer = 99
	*(*result.Events[1].Inputs["values"].Collection)[0].Record["id"].String = "event mutation"
	(*result.Events[1].Inputs["values"].Collection)[0].RecordRef.Digest = "changed"
	afterBundle, _ := json.Marshal(b)
	afterInputs, _ := json.Marshal(values)
	if string(beforeBundle) != string(afterBundle) || string(beforeInputs) != string(afterInputs) {
		t.Fatal("result mutation reached supplied definition or data")
	}
	exit := result.Events[len(result.Events)-1].Outputs["result"]
	if *(*exit.Collection)[0].Record["quantity"].Integer != 1 || exit.RecordRef.Record != "Item" {
		t.Fatal("outputs and event snapshots share mutable structures")
	}
	repeated := evaluateOK(t, b, values)
	if *(*repeated.Outputs["result"].Collection)[0].Record["quantity"].Integer != 1 {
		t.Fatal("later evaluation was affected by earlier evidence edits")
	}
}

func TestStructuredErrorsNeverReturnSuccessfulPartialEvidence(t *testing.T) {
	lock := structuredLock(t)
	v := item(lock, "a", 1, true)
	atInputs := map[string]model.TypedValue{"values": collection(lock, v), "index": integer(0)}
	atBundle := bundleDefinition(t, structuredDefinition(model.BehaviorNode{Op: "collection_at"}, atInputs, "record", lock))
	for _, index := range []int64{-1, 1, model.MaxBehaviorInteger} {
		atInputs["index"] = integer(index)
		r, err := Evaluate(context.Background(), atBundle, atInputs)
		if err == nil || r.EngineVersion != "" || !strings.Contains(err.Error(), "$/operation") {
			t.Fatalf("out-of-range index: %+v %v", r, err)
		}
	}
	records := make([]model.TypedValue, 128)
	for i := range records {
		records[i] = v
	}
	appendInputs := map[string]model.TypedValue{"values": collection(lock, records...), "value": v}
	appendBundle := bundleDefinition(t, structuredDefinition(model.BehaviorNode{Op: "collection_append"}, appendInputs, "collection", lock))
	if r, err := Evaluate(context.Background(), appendBundle, appendInputs); err == nil || r.EngineVersion != "" || !strings.Contains(err.Error(), "128") {
		t.Fatalf("collection amplification: %+v %v", r, err)
	}
	identityInputs := map[string]model.TypedValue{"value": v}
	identityBundle := bundleDefinition(t, structuredDefinition(model.BehaviorNode{Op: "identity"}, identityInputs, "record", lock))
	for _, mutate := range []func(*model.TypedValue){
		func(v *model.TypedValue) { delete(v.Record, "active") },
		func(v *model.TypedValue) { v.Record["extra"] = integer(1) },
		func(v *model.TypedValue) { v.Record["quantity"] = integer(model.MaxBehaviorInteger + 1) },
		func(v *model.TypedValue) { v.Record["quantity"] = textValue("1") },
		func(v *model.TypedValue) { v.Record = nil },
		func(v *model.TypedValue) { v.RecordRef.Version = "1.0.1" },
	} {
		invalid := cloneValue(v)
		mutate(&invalid)
		r, err := Evaluate(context.Background(), identityBundle, map[string]model.TypedValue{"value": invalid})
		if err == nil || r.EngineVersion != "" {
			t.Fatalf("invalid structured input: %+v %v", r, err)
		}
	}
	empty := collection(lock)
	emptyBundle := bundleDefinition(t, structuredDefinition(model.BehaviorNode{Op: "identity"}, map[string]model.TypedValue{"value": empty}, "collection", lock))
	invalid := empty
	invalid.Collection = nil
	if r, err := Evaluate(context.Background(), emptyBundle, map[string]model.TypedValue{"value": invalid}); err == nil || r.EngineVersion != "" {
		t.Fatal("null collection accepted")
	}
	evaluateOK(t, emptyBundle, map[string]model.TypedValue{"value": empty})
}

func TestStructuredCollectionByteLimitAndTraceAmplification(t *testing.T) {
	lock := structuredLock(t)
	v := item(lock, strings.Repeat("x", 4096), 1, true)
	values := []model.TypedValue{}
	for {
		next := append(append([]model.TypedValue{}, values...), v)
		encoded, _ := json.Marshal(collection(lock, next...))
		if len(encoded) > 64*1024 {
			break
		}
		values = next
	}
	if len(values) == 0 || len(values) >= 128 {
		t.Fatal("bad byte-bound fixture")
	}
	large := collection(lock, values...)
	args := map[string]model.TypedValue{"values": large, "value": v}
	b := bundleDefinition(t, structuredDefinition(model.BehaviorNode{Op: "collection_append"}, args, "collection", lock))
	if r, err := Evaluate(context.Background(), b, args); err == nil || r.EngineVersion != "" {
		t.Fatalf("oversized append result: %+v %v", r, err)
	}
	p := structuredDefinition(model.BehaviorNode{Op: "identity"}, map[string]model.TypedValue{"value": large}, "collection", lock)
	p.Behavior.Nodes = []model.BehaviorNode{}
	source := input("value")
	for _, id := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"} {
		p.Behavior.Nodes = append(p.Behavior.Nodes, model.BehaviorNode{ID: id, Op: "identity", Inputs: map[string]model.BehaviorBinding{"value": source}})
		source = output(id)
	}
	p.Behavior.Bindings["result"] = source
	if r, err := Evaluate(context.Background(), bundleDefinition(t, p), map[string]model.TypedValue{"value": large}); !errors.Is(err, ErrLimit) || r.EngineVersion != "" {
		t.Fatalf("trace amplification: %+v %v", r, err)
	}
	// Each collection is valid independently, but five copies exceed the public
	// map bound. Exercise both externally supplied inputs and computed outputs.
	ports := []model.BehaviorPort{}
	bindings := map[string]model.BehaviorBinding{}
	inputs := map[string]model.TypedValue{}
	for _, name := range []string{"a", "b", "c", "d", "e"} {
		ports = append(ports, model.BehaviorPort{Name: name, Type: "collection", RecordRef: dataReference(lock)})
		bindings[name] = input("a")
		inputs[name] = large
	}
	p.Behavior = &model.Behavior{Inputs: ports, Outputs: ports[:1], Nodes: []model.BehaviorNode{}, Bindings: map[string]model.BehaviorBinding{"a": input("a")}}
	if r, err := Evaluate(context.Background(), bundleDefinition(t, p), inputs); err == nil || r.EngineVersion != "" || !strings.Contains(err.Error(), "256 KiB") {
		t.Fatalf("aggregate input map: %+v %v", r, err)
	}
	p.Behavior = &model.Behavior{Inputs: ports[:1], Outputs: ports, Nodes: []model.BehaviorNode{}, Bindings: bindings}
	if r, err := Evaluate(context.Background(), bundleDefinition(t, p), map[string]model.TypedValue{"a": large}); err == nil || r.EngineVersion != "" || !strings.Contains(err.Error(), "256 KiB") {
		t.Fatalf("aggregate output map: %+v %v", r, err)
	}
}

func TestStructuredNestedCallsUseExactEmbeddedUnion(t *testing.T) {
	lock := structuredLock(t)
	v := item(lock, "nested", 42, true)
	args := map[string]model.TypedValue{"value": v}
	child := structuredDefinition(model.BehaviorNode{Op: "identity"}, args, "record", lock)
	child.ID = "test.structured-child"
	// The parent alone supplies the lock; child types resolve against the bundle.
	child.DataContracts = nil
	childLock := lockDefinition(t, child)
	parent := structuredDefinition(model.BehaviorNode{}, args, "record", lock)
	parent.Behavior.Nodes[0].Component = refCopy(reference(childLock))
	b := bundleDefinition(t, parent, childLock)
	first := evaluateOK(t, b, args)
	if first.Operations != 1 || first.Invocations != 2 || !reflect.DeepEqual(first.Outputs["result"], v) {
		t.Fatalf("nested structured result: %+v", first)
	}
	for i := 0; i < 4; i++ {
		if next := evaluateOK(t, b, args); !reflect.DeepEqual(first, next) {
			t.Fatal("nested structured values or traces were nondeterministic")
		}
	}
	for _, after := range []int32{1, 7, 11} {
		if r, err := Evaluate(&stepContext{after: after}, b, args); !errors.Is(err, context.Canceled) || r.EngineVersion != "" {
			t.Fatalf("structured cancellation: %+v %v", r, err)
		}
	}
	// Shape equality is insufficient: every nominal-reference member matters.
	for _, mutate := range []func(*model.DataTypeRef){
		func(ref *model.DataTypeRef) { ref.ContractID = "test.same-shape" },
		func(ref *model.DataTypeRef) { ref.Version = "1.0.1" },
		func(ref *model.DataTypeRef) { ref.Digest = strings.Repeat("a", 64) },
		func(ref *model.DataTypeRef) { ref.Record = "Other" },
	} {
		other := cloneValue(v)
		mutate(other.RecordRef)
		if equalValue(v, other) {
			t.Fatal("record equality omitted a nominal-reference member")
		}
		if r, err := Evaluate(context.Background(), b, map[string]model.TypedValue{"value": other}); err == nil || r.EngineVersion != "" {
			t.Fatalf("different nominal input accepted: %+v %v", r, err)
		}
	}
}

func TestStructuredEagerSelectAndRootCases(t *testing.T) {
	lock := structuredLock(t)
	v := item(lock, "a", 1, true)
	p := definition("test.structured-eager", model.Behavior{Inputs: []model.BehaviorPort{}, Outputs: []model.BehaviorPort{{Name: "result", Type: "record", RecordRef: dataReference(lock)}}, Nodes: []model.BehaviorNode{
		{ID: "bad", Op: "collection_at", Inputs: map[string]model.BehaviorBinding{"values": literal(collection(lock)), "index": literal(integer(0))}},
		{ID: "choose", Op: "select", Inputs: map[string]model.BehaviorBinding{"condition": literal(boolean(false)), "trueValue": output("bad"), "falseValue": literal(v)}},
	}, Bindings: map[string]model.BehaviorBinding{"result": output("choose")}})
	p.SchemaVersion = 3
	p.DataContracts = []model.DataContractLock{lock}
	b := bundleDefinition(t, p)
	if r, err := Evaluate(context.Background(), b, nil); err == nil || r.EngineVersion != "" || !strings.Contains(err.Error(), "$/bad") {
		t.Fatalf("select hid structured error: %+v %v", r, err)
	}
	p = structuredDefinition(model.BehaviorNode{Op: "identity"}, map[string]model.TypedValue{"value": v}, "record", lock)
	p.Tests = []model.BehaviorCase{{Name: "Record", Inputs: map[string]model.TypedValue{"value": v}, Expected: map[string]model.TypedValue{"result": v}}}
	got, err := Test(context.Background(), bundleDefinition(t, p))
	if err != nil || !got.Passed || len(got.Cases) != 1 {
		t.Fatalf("structured expected comparison: %+v %v", got, err)
	}
	*got.Cases[0].Actual["result"].Record["quantity"].Integer = 7
	if *p.Tests[0].Expected["result"].Record["quantity"].Integer != 1 || *got.Cases[0].Expected["result"].Record["quantity"].Integer != 1 {
		t.Fatal("test result aliases expectations")
	}
}

func TestStructuredExamplesMatchIndependentPolicyTruthTable(t *testing.T) {
	bundles := model.ExampleStructuredComponentBundles()
	if len(bundles) != 2 {
		t.Fatal("expected healthcare and education structured examples")
	}
	legacy := model.ExampleComponentBundles()
	sharedDataDigest := ""
	for domain, bundle := range bundles {
		var root model.ComponentPackage
		for _, lock := range bundle.Packages {
			if reference(lock) == bundle.Root {
				root = lock.Package
				continue
			}
			unchanged := false
			for _, old := range legacy[domain].Packages {
				if reference(old) == reference(lock) && reflect.DeepEqual(old.Package, lock.Package) {
					unchanged = true
				}
			}
			if !unchanged || lock.Package.SchemaVersion != 2 {
				t.Fatal("structured wrapper changed a shared scalar definition")
			}
		}
		if len(root.DataContracts) != 1 {
			t.Fatal("wrapper must supply its exact shared data contract")
		}
		data := root.DataContracts[0]
		if domain == 0 {
			sharedDataDigest = data.Digest
		} else if data.Digest != sharedDataDigest {
			t.Fatal("domain wrappers must reuse the same exact data contract")
		}
		ref := func(record string) *model.DataTypeRef {
			return &model.DataTypeRef{ContractID: data.Contract.ID, Version: data.Contract.Version, Digest: data.Digest, Record: record}
		}
		role, scope := "clinician", "record:read"
		if domain == 1 {
			role, scope = "teacher", "student:read"
		}
		for mask := 0; mask < 16; mask++ {
			subject := model.TypedValue{Type: "record", RecordRef: ref("Subject"), Record: map[string]model.TypedValue{"id": textValue("s"), "tenantId": textValue("alpha"), "role": textValue(role), "active": boolean(mask&8 != 0)}}
			resource := model.TypedValue{Type: "record", RecordRef: ref("Resource"), Record: map[string]model.TypedValue{"id": textValue("r"), "tenantId": textValue("alpha"), "scope": textValue(scope)}}
			if mask&1 == 0 {
				resource.Record["tenantId"] = textValue("beta")
			}
			if mask&2 == 0 {
				subject.Record["role"] = textValue("visitor")
			}
			if mask&4 == 0 {
				resource.Record["scope"] = textValue("other:write")
			}
			values := map[string]model.TypedValue{"subject": subject, "resource": resource}
			result := evaluateOK(t, bundle, values)
			// The independent truth table permits only when all four conditions
			// hold; it does not use primitive evaluation to compute expectations.
			if result.Outputs["permitted"].Boolean == nil || *result.Outputs["permitted"].Boolean != (mask == 15) || result.Operations != 11 || result.Invocations != 3 {
				t.Fatalf("domain %d mask %d: %+v", domain, mask, result)
			}
			if mask == 15 {
				encoded, err := json.Marshal(bundle)
				if err != nil {
					t.Fatal(err)
				}
				var portable model.ComponentBundle
				if err := json.Unmarshal(encoded, &portable); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(result, evaluateOK(t, portable, values)) {
					t.Fatal("portable structured bundle changed exact results or traces")
				}
			}
		}
		suite, err := Test(context.Background(), bundle)
		if err != nil || !suite.Passed || len(suite.Cases) != 5 {
			t.Fatalf("structured example root cases: %+v %v", suite, err)
		}
	}
}
