package model

import (
	"encoding/json"
	"fmt"
)

func reservationText(value string) TypedValue   { return TypedValue{Type: "string", String: &value} }
func reservationInteger(value int64) TypedValue { return TypedValue{Type: "integer", Integer: &value} }
func reservationBoolean(value bool) TypedValue  { return TypedValue{Type: "boolean", Boolean: &value} }

// ExampleStateScenarios demonstrates one exact owned-state machine in two
// domains. Expectations are handwritten; example construction never runs it.
func ExampleStateScenarios() []StateExample {
	machine := reservationMachine()
	digest, err := StateMachineDigest(machine)
	if err != nil {
		panic(fmt.Sprintf("invalid reservation machine: %v", err))
	}
	lock := StateMachineLock{Machine: machine, Digest: digest}
	commandRef := machine.Commands[0].Inputs[0].RecordRef
	request := func(tenant, resource string, quantity int64) TypedValue {
		ref := *commandRef
		return TypedValue{Type: "record", RecordRef: &ref, Record: map[string]TypedValue{
			"tenantId": reservationText(tenant), "resourceId": reservationText(resource), "quantity": reservationInteger(quantity),
		}}
	}
	examples := make([]StateExample, 0, 2)
	for _, domain := range []struct {
		name, tenant, resource, partnerTenant, partnerResource string
		initial, partnerInitial, firstQuantity, partnerFinal   int64
	}{
		{"Pharmacy stock reservation", "hospitalAlpha", "supplyA", "pharmacyBeta", "supplyB", 5, 3, 2, 1},
		{"School library reservation", "schoolAlpha", "titleA", "libraryBeta", "titleB", 4, 2, 1, 0},
	} {
		values := func(tenant, resource string, available, counter int64) map[string]TypedValue {
			return map[string]TypedValue{"tenantId": reservationText(tenant), "resourceId": reservationText(resource), "available": reservationInteger(available), "acceptedSinceRestart": reservationInteger(counter)}
		}
		command := func(instance, id string, revision, quantity int64, fault string) StateStep {
			tenant, resource := domain.tenant, domain.resource
			if instance == "partner" {
				tenant, resource = domain.partnerTenant, domain.partnerResource
			}
			return StateStep{Kind: "command", InstanceID: instance, CommandID: id, Command: "reserve", ExpectedRevision: &revision,
				Inputs: map[string]TypedValue{"request": request(tenant, resource, quantity)}, Fault: fault}
		}
		wrongTarget := command("partner", "reserveB", 1, 1, "")
		wrongTarget.Inputs["request"].Record["tenantId"] = reservationText(domain.tenant)
		scenario := StateScenario{
			SchemaVersion: 1, Name: domain.name,
			Instances: []StateInstance{
				{ID: "primary", Name: "Primary resource", Initial: values(domain.tenant, domain.resource, domain.initial, 0)},
				{ID: "partner", Name: "Independent partner resource", Initial: values(domain.partnerTenant, domain.partnerResource, domain.partnerInitial, 10)},
			},
			Steps: []StateStep{
				command("primary", "reserveA", 0, domain.firstQuantity, ""),
				command("partner", "reserveA", 0, 1, ""),                                 // IDs are scoped to their instance.
				command("primary", "reserveA", 0, domain.firstQuantity, "before_commit"), // Replay bypasses fault injection.
				command("primary", "reserveB", 1, 4, ""),                                 // Denial commits a receipt, not a revision.
				command("primary", "reserveC", 1, 1, "before_commit"),
				{Kind: "restart", InstanceID: "primary"},
				command("primary", "reserveC", 1, 1, "after_commit_before_reply"),
				{Kind: "restart", InstanceID: "primary"},
				command("primary", "reserveC", 1, 1, "after_commit_before_reply"),
				command("primary", "reserveD", 2, 2, ""),
				command("primary", "reserveE", 3, 1, ""),
				command("primary", "reserveB", 1, 4, ""), // Historical remaining=3; current available=0.
				command("primary", "reserveA", 0, domain.firstQuantity+1, ""),
				command("primary", "reserveF", 2, 1, ""),
				{Kind: "restart", InstanceID: "partner"},
				command("partner", "reserveA", 0, 1, ""), // Historical counter=11; current counter=10.
				wrongTarget,
				command("partner", "reserveC", 1, 1, ""),
			},
			Expected: []StateExpectation{
				{InstanceID: "primary", State: values(domain.tenant, domain.resource, 0, 1), Revision: 3, Receipts: 5, Effects: 3},
				{InstanceID: "partner", State: values(domain.partnerTenant, domain.partnerResource, domain.partnerFinal, 11), Revision: 2, Receipts: 3, Effects: 2},
			},
		}
		// Every returned lock, map and pointer is independently owned, including
		// the exact shared definitions in the two examples.
		example := StateExample{Machine: lock, Scenario: scenario}
		encoded, err := json.Marshal(example)
		if err != nil {
			panic(err)
		}
		var owned StateExample
		if err := json.Unmarshal(encoded, &owned); err != nil {
			panic(err)
		}
		examples = append(examples, owned)
	}
	return examples
}

func reservationMachine() StateMachine {
	data := DataContract{SchemaVersion: 1, ID: "lce.reservation-data", Version: "1.0.0", Name: "Resource reservation values",
		Description: "Required scalar command and notice records shared by resource-reservation examples. Command identity belongs to the state protocol envelope.",
		Author:      "LCE Studio contributors", License: "MIT",
		Records: []DataRecord{
			{Name: "ReservationCommand", Fields: []DataField{{Name: "tenantId", Type: "string"}, {Name: "resourceId", Type: "string"}, {Name: "quantity", Type: "integer"}}},
			{Name: "ReservationNotice", Fields: []DataField{{Name: "tenantId", Type: "string"}, {Name: "resourceId", Type: "string"}, {Name: "quantity", Type: "integer"}, {Name: "remaining", Type: "integer"}}},
		}, Entities: []DataEntity{}, Relationships: []DataRelationship{},
	}
	digest, err := DataContractDigest(data)
	if err != nil {
		panic(err)
	}
	dataLock := DataContractLock{Contract: data, Digest: digest}
	ref := func(record string) *DataTypeRef {
		return &DataTypeRef{ContractID: data.ID, Version: data.Version, Digest: digest, Record: record}
	}
	input := func(name string) BehaviorBinding { return BehaviorBinding{Input: name} }
	output := func(node string) BehaviorBinding { return BehaviorBinding{Node: node, Port: "result"} }
	literal := func(value TypedValue) BehaviorBinding { return BehaviorBinding{Literal: &value} }
	number := func(value int64) BehaviorBinding { return literal(reservationInteger(value)) }
	text := func(value string) BehaviorBinding { return literal(reservationText(value)) }
	nodes := []BehaviorNode{}
	node := func(id, op string, inputs map[string]BehaviorBinding) {
		i := len(nodes)
		nodes = append(nodes, BehaviorNode{ID: id, Op: op, Inputs: inputs, Position: Position{X: float64(i%4) * 320, Y: float64(i/4) * 160}})
	}
	binary := func(id, op string, left, right BehaviorBinding) {
		node(id, op, map[string]BehaviorBinding{"left": left, "right": right})
	}
	selectValue := func(id string, condition, yes, no BehaviorBinding) {
		node(id, "select", map[string]BehaviorBinding{"condition": condition, "trueValue": yes, "falseValue": no})
	}
	for _, field := range []string{"tenantId", "resourceId", "quantity"} {
		node("request"+field, "record_get", map[string]BehaviorBinding{"value": input("request")})
		nodes[len(nodes)-1].Field = field
	}
	binary("sameTenant", "equal", input("tenantId"), output("requesttenantId"))
	binary("sameResource", "equal", input("resourceId"), output("requestresourceId"))
	binary("sameTarget", "and", output("sameTenant"), output("sameResource"))
	binary("positive", "less_than", number(0), output("requestquantity"))
	binary("validAvailable", "less_or_equal", number(0), input("available"))
	binary("enough", "less_or_equal", output("requestquantity"), input("available"))
	binary("validQuantity", "and", output("positive"), output("enough"))
	binary("validStock", "and", output("validAvailable"), output("validQuantity"))
	binary("accepted", "and", output("sameTarget"), output("validStock"))
	// Selection is eager. Sanitize the operand before subtracting so a denied
	// extreme negative quantity cannot overflow an otherwise unused branch.
	selectValue("delta", output("accepted"), output("requestquantity"), number(0))
	binary("remaining", "subtract", input("available"), output("delta"))
	selectValue("counterDelta", output("accepted"), number(1), number(0))
	binary("nextCounter", "add", input("acceptedSinceRestart"), output("counterDelta"))
	selectValue("quantityReason", output("positive"), text("insufficient"), text("invalid_quantity"))
	selectValue("stockReason", output("validAvailable"), output("quantityReason"), text("invalid_state"))
	selectValue("acceptedReason", output("accepted"), text("reserved"), output("stockReason"))
	selectValue("reason", output("sameTarget"), output("acceptedReason"), text("wrong_target"))
	node("notice", "record_make", map[string]BehaviorBinding{"tenantId": input("tenantId"), "resourceId": input("resourceId"), "quantity": output("delta"), "remaining": output("remaining")})
	nodes[len(nodes)-1].RecordRef = ref("ReservationNotice")
	publicOutputs := []BehaviorPort{{Name: "accepted", Type: "boolean"}, {Name: "remaining", Type: "integer"}, {Name: "acceptedSinceRestart", Type: "integer"}, {Name: "reason", Type: "string"}}
	decision := ComponentPackage{SchemaVersion: 3, ID: "lce.reserve-available", Version: "1.0.0", Name: "Reserve available resource",
		Description: "Pure bounded decision for one owned resource. Matching target, positive quantity and sufficient nonnegative availability permit a reservation. Returns proposed values and a notice; the state host controls commits and effect intents.",
		Author:      "LCE Studio contributors", License: "MIT", Kernel: "composite", Capabilities: []string{"design", "evaluate"}, DataContracts: []DataContractLock{dataLock},
		Behavior: &Behavior{
			Inputs:  []BehaviorPort{{Name: "tenantId", Type: "string"}, {Name: "resourceId", Type: "string"}, {Name: "available", Type: "integer"}, {Name: "acceptedSinceRestart", Type: "integer"}, {Name: "request", Type: "record", RecordRef: ref("ReservationCommand")}},
			Outputs: append(append([]BehaviorPort{}, publicOutputs...), BehaviorPort{Name: "notice", Type: "record", RecordRef: ref("ReservationNotice")}),
			Nodes:   nodes, Bindings: map[string]BehaviorBinding{"accepted": output("accepted"), "remaining": output("remaining"), "acceptedSinceRestart": output("nextCounter"), "reason": output("reason"), "notice": output("notice")},
		},
	}
	for _, test := range []struct {
		name, tenant, resource                         string
		available, quantity, remaining, counter, delta int64
		accepted                                       bool
		reason                                         string
	}{
		{"Reserve two units", "tenantA", "resourceA", 5, 2, 3, 1, 2, true, "reserved"},
		{"Insufficient quantity", "tenantA", "resourceA", 3, 4, 3, 0, 0, false, "insufficient"},
		{"Wrong tenant", "tenantB", "resourceA", 5, 1, 5, 0, 0, false, "wrong_target"},
		{"Wrong resource", "tenantA", "resourceB", 5, 1, 5, 0, 0, false, "wrong_target"},
		{"Zero quantity", "tenantA", "resourceA", 5, 0, 5, 0, 0, false, "invalid_quantity"},
		{"Negative quantity at integer boundary", "tenantA", "resourceA", MaxBehaviorInteger, -MaxBehaviorInteger, MaxBehaviorInteger, 0, 0, false, "invalid_quantity"},
		{"Invalid initial stock", "tenantA", "resourceA", -1, 1, -1, 0, 0, false, "invalid_state"},
	} {
		decision.Tests = append(decision.Tests, BehaviorCase{Name: test.name,
			Inputs: map[string]TypedValue{"tenantId": reservationText("tenantA"), "resourceId": reservationText("resourceA"), "available": reservationInteger(test.available), "acceptedSinceRestart": reservationInteger(0),
				"request": {Type: "record", RecordRef: ref("ReservationCommand"), Record: map[string]TypedValue{"tenantId": reservationText(test.tenant), "resourceId": reservationText(test.resource), "quantity": reservationInteger(test.quantity)}}},
			Expected: map[string]TypedValue{"accepted": reservationBoolean(test.accepted), "remaining": reservationInteger(test.remaining), "acceptedSinceRestart": reservationInteger(test.counter), "reason": reservationText(test.reason),
				"notice": {Type: "record", RecordRef: ref("ReservationNotice"), Record: map[string]TypedValue{"tenantId": reservationText("tenantA"), "resourceId": reservationText("resourceA"), "quantity": reservationInteger(test.delta), "remaining": reservationInteger(test.remaining)}}},
		})
	}
	bundle, issues := PrepareComponentBundle(decision, []PackageLock{})
	if len(issues) != 0 {
		panic(fmt.Sprintf("invalid reservation decision: %+v", issues))
	}
	return StateMachine{SchemaVersion: 1, ID: "lce.resource-reservation", Version: "1.0.0", Name: "Owned resource reservation",
		Description: "One serialized resource owner with durable availability and a volatile accepted-command counter. Accepted commands commit a reservation notice intent; no external delivery, pharmacy-specific rules or authorization enforcement is implied.",
		Author:      "LCE Studio contributors", License: "MIT", DataContracts: []DataContractLock{dataLock},
		State: []StateSlot{
			{Name: "tenantId", Type: "string", Durability: "durable", Initial: reservationText("tenantDefault")},
			{Name: "resourceId", Type: "string", Durability: "durable", Initial: reservationText("resourceDefault")},
			{Name: "available", Type: "integer", Durability: "durable", Initial: reservationInteger(0)},
			{Name: "acceptedSinceRestart", Type: "integer", Durability: "volatile", Initial: reservationInteger(0)},
		},
		Effects: []StateEffect{{Name: "reservation", Payload: []BehaviorPort{{Name: "notice", Type: "record", RecordRef: ref("ReservationNotice")}}}},
		Commands: []StateCommand{{Name: "reserve", Inputs: []BehaviorPort{{Name: "request", Type: "record", RecordRef: ref("ReservationCommand")}}, Outputs: publicOutputs, Decision: bundle,
			Bindings:       map[string]StateBinding{"tenantId": {State: "tenantId"}, "resourceId": {State: "resourceId"}, "available": {State: "available"}, "acceptedSinceRestart": {State: "acceptedSinceRestart"}, "request": {Input: "request"}},
			AcceptedOutput: "accepted", Writes: map[string]string{"available": "remaining", "acceptedSinceRestart": "acceptedSinceRestart"},
			Results: map[string]string{"accepted": "accepted", "remaining": "remaining", "acceptedSinceRestart": "acceptedSinceRestart", "reason": "reason"},
			Effects: []StateEffectBinding{{Effect: "reservation", WhenOutput: "accepted", Payload: map[string]string{"notice": "notice"}}},
		}},
	}
}
