package model

import (
	"encoding/json"
	"fmt"
)

// ExampleDeliveryScenarios returns independently owned examples with the same
// exact delivery model. Their numeric expectations are specified, never run.
func ExampleDeliveryScenarios() []DeliveryExample {
	source := ExampleStateScenarios()[0].Machine
	ledger := reservationLedgerMachine(source.Machine.DataContracts[0])
	machineRef := func(lock StateMachineLock) DeliveryMachineRef {
		return DeliveryMachineRef{MachineID: lock.Machine.ID, Version: lock.Machine.Version, Digest: lock.Digest}
	}
	model := DeliveryModel{SchemaVersion: 1, ID: "lce.reservation-delivery", Version: "1.0.0", Name: "Reservation notice delivery",
		Description: "A resource owner commits reservation intentions; explicit dispatch and delivery submit typed notices to an independently owned reservation ledger. Scripted faults and retries demonstrate receipt deduplication, not external delivery or a distributed transaction.",
		Author:      "LCE Studio contributors", License: "MIT", Machines: []StateMachineLock{source, ledger},
		Instances: []DeliveryInstance{
			{ID: "stock", Name: "Resource owner", Machine: machineRef(source), Position: Position{X: 70, Y: 100}},
			{ID: "ledger", Name: "Reservation register", Machine: machineRef(ledger), Position: Position{X: 600, Y: 100}},
		},
		Routes: []DeliveryRoute{{ID: "reservationToLedger", Name: "Record a reservation notice", SourceInstanceID: "stock", Effect: "reservation", TargetInstanceID: "ledger", Command: "recordReservation", Inputs: map[string]string{"notice": "notice"}}},
	}
	digest, err := DeliveryModelDigest(model)
	if err != nil {
		panic(fmt.Sprintf("invalid delivery example model: %v", err))
	}
	lock := DeliveryModelLock{Model: model, Digest: digest}
	commandRef := source.Machine.Commands[0].Inputs[0].RecordRef
	examples := make([]DeliveryExample, 0, 2)
	for _, domain := range []struct {
		name, tenant, resource string
		initial, firstQuantity int64
	}{
		{"Pharmacy reservation delivery", "pharmacyAlpha", "supplyA", 5, 2},
		{"Library reservation delivery", "schoolAlpha", "titleA", 4, 1},
	} {
		state := func(quantityName string, quantity, counter int64) map[string]TypedValue {
			return map[string]TypedValue{"tenantId": reservationText(domain.tenant), "resourceId": reservationText(domain.resource), quantityName: reservationInteger(quantity), "acceptedSinceRestart": reservationInteger(counter)}
		}
		command := func(id string, revision, quantity int64) DeliveryStep {
			ref := *commandRef
			return DeliveryStep{Kind: "command", InstanceID: "stock", CommandID: id, Command: "reserve", ExpectedRevision: &revision,
				Inputs: map[string]TypedValue{"request": {Type: "record", RecordRef: &ref, Record: map[string]TypedValue{
					"tenantId": reservationText(domain.tenant), "resourceId": reservationText(domain.resource), "quantity": reservationInteger(quantity),
				}}}}
		}
		dispatch := func(id, alias string, revision int64) DeliveryStep {
			return DeliveryStep{Kind: "dispatch", RouteID: "reservationToLedger", SourceCommandID: id, Message: alias, ExpectedRevision: &revision}
		}
		deliver := func(alias, fault string) DeliveryStep {
			return DeliveryStep{Kind: "deliver", Message: alias, Fault: fault}
		}
		accepted := true
		scenario := DeliveryScenario{SchemaVersion: 1, Name: domain.name,
			Initial: []DeliveryInitial{{InstanceID: "stock", Initial: state("available", domain.initial, 0)}, {InstanceID: "ledger", Initial: state("totalReserved", 0, 10)}},
			Steps: []DeliveryStep{
				command("reserveA", 0, domain.firstQuantity),
				dispatch("reserveA", "firstReservation", 0),
				deliver("firstReservation", "drop_before_receive"),
				deliver("firstReservation", "before_recipient_commit"),
				deliver("firstReservation", "after_recipient_commit_before_ack"),
				{Kind: "restart", InstanceID: "ledger"},
				{Kind: "restart", InstanceID: "stock"},
				deliver("firstReservation", ""),
				command("reserveA", 0, domain.firstQuantity),
				dispatch("reserveA", "firstReservation", 0),
				command("reserveB", 1, 1),
				dispatch("reserveB", "secondReservation", 1),
				deliver("secondReservation", ""),
				command("reserveC", 2, 2),
				dispatch("reserveC", "thirdReservation", 2),
				deliver("thirdReservation", ""),
				command("reserveD", 3, 1),
				command("reserveD", 3, 1),
				{Kind: "restart", InstanceID: "ledger"},
			},
			Expected: []StateExpectation{
				{InstanceID: "stock", State: state("available", 0, 2), Revision: 3, Receipts: 4, Effects: 3},
				{InstanceID: "ledger", State: state("totalReserved", domain.initial, 10), Revision: 3, Receipts: 3, Effects: 0},
			},
			ExpectedMessages: []DeliveryMessageExpectation{
				{Name: "firstReservation", Status: "acknowledged", Attempts: 4, Accepted: &accepted},
				{Name: "secondReservation", Status: "acknowledged", Attempts: 1, Accepted: &accepted},
				{Name: "thirdReservation", Status: "acknowledged", Attempts: 1, Accepted: &accepted},
			},
		}
		// Decode the complete artifact to give every returned definition, input,
		// assertion and domain its own maps, slices and scalar pointers.
		encoded, err := json.Marshal(DeliveryExample{Model: lock, Scenario: scenario})
		if err != nil {
			panic(err)
		}
		var example DeliveryExample
		if err := json.Unmarshal(encoded, &example); err != nil {
			panic(err)
		}
		examples = append(examples, example)
	}
	return examples
}

func reservationLedgerMachine(data DataContractLock) StateMachineLock {
	ref := func(record string) *DataTypeRef {
		return &DataTypeRef{ContractID: data.Contract.ID, Version: data.Contract.Version, Digest: data.Digest, Record: record}
	}
	input := func(name string) BehaviorBinding { return BehaviorBinding{Input: name} }
	output := func(node string) BehaviorBinding { return BehaviorBinding{Node: node, Port: "result"} }
	literal := func(value TypedValue) BehaviorBinding { return BehaviorBinding{Literal: &value} }
	number := func(value int64) BehaviorBinding { return literal(reservationInteger(value)) }
	text := func(value string) BehaviorBinding { return literal(reservationText(value)) }
	nodes := []BehaviorNode{}
	node := func(id, op string, bindings map[string]BehaviorBinding) {
		i := len(nodes)
		nodes = append(nodes, BehaviorNode{ID: id, Op: op, Inputs: bindings, Position: Position{X: float64(i%4) * 320, Y: float64(i/4) * 160}})
	}
	binary := func(id, op string, left, right BehaviorBinding) {
		node(id, op, map[string]BehaviorBinding{"left": left, "right": right})
	}
	selectValue := func(id string, condition, yes, no BehaviorBinding) {
		node(id, "select", map[string]BehaviorBinding{"condition": condition, "trueValue": yes, "falseValue": no})
	}
	for _, field := range []string{"tenantId", "resourceId", "quantity"} {
		node("notice"+field, "record_get", map[string]BehaviorBinding{"value": input("notice")})
		nodes[len(nodes)-1].Field = field
	}
	binary("sameTenant", "equal", input("tenantId"), output("noticetenantId"))
	binary("sameResource", "equal", input("resourceId"), output("noticeresourceId"))
	binary("sameTarget", "and", output("sameTenant"), output("sameResource"))
	binary("positive", "less_than", number(0), output("noticequantity"))
	binary("validTotal", "less_or_equal", number(0), input("totalReserved"))
	// Select safe operands before eager arithmetic: neither a negative initial
	// total nor an extreme rejected quantity may overflow an unused branch.
	selectValue("safeTotal", output("validTotal"), input("totalReserved"), number(0))
	binary("headroom", "subtract", number(MaxBehaviorInteger), output("safeTotal"))
	binary("capacity", "less_or_equal", output("noticequantity"), output("headroom"))
	binary("validQuantity", "and", output("positive"), output("capacity"))
	binary("validTotalQuantity", "and", output("validTotal"), output("validQuantity"))
	binary("accepted", "and", output("sameTarget"), output("validTotalQuantity"))
	selectValue("delta", output("accepted"), output("noticequantity"), number(0))
	binary("nextTotal", "add", input("totalReserved"), output("delta"))
	selectValue("counterDelta", output("accepted"), number(1), number(0))
	binary("nextCounter", "add", input("acceptedSinceRestart"), output("counterDelta"))
	selectValue("quantityReason", output("positive"), text("capacity"), text("invalid_quantity"))
	selectValue("stateReason", output("validTotal"), output("quantityReason"), text("invalid_state"))
	selectValue("acceptedReason", output("accepted"), text("recorded"), output("stateReason"))
	selectValue("reason", output("sameTarget"), output("acceptedReason"), text("wrong_target"))
	ports := []BehaviorPort{{Name: "accepted", Type: "boolean"}, {Name: "totalReserved", Type: "integer"}, {Name: "acceptedSinceRestart", Type: "integer"}, {Name: "reason", Type: "string"}}
	decision := ComponentPackage{SchemaVersion: 3, ID: "lce.record-reservation", Version: "1.0.0", Name: "Record a reservation notice",
		Description: "Pure proposal to add the quantity in an exact reservation notice to a bounded owned total. Checks target identifiers and arithmetic bounds; command receipts in the state host supply deduplication.",
		Author:      "LCE Studio contributors", License: "MIT", Kernel: "composite", Capabilities: []string{"design", "evaluate"}, DataContracts: []DataContractLock{data},
		Behavior: &Behavior{
			Inputs:  []BehaviorPort{{Name: "tenantId", Type: "string"}, {Name: "resourceId", Type: "string"}, {Name: "totalReserved", Type: "integer"}, {Name: "acceptedSinceRestart", Type: "integer"}, {Name: "notice", Type: "record", RecordRef: ref("ReservationNotice")}},
			Outputs: ports, Nodes: nodes, Bindings: map[string]BehaviorBinding{"accepted": output("accepted"), "totalReserved": output("nextTotal"), "acceptedSinceRestart": output("nextCounter"), "reason": output("reason")},
		},
	}
	for _, test := range []struct {
		name, tenant, resource         string
		total, quantity, next, counter int64
		accepted                       bool
		reason                         string
	}{
		{"Record two units", "tenantA", "resourceA", 0, 2, 2, 11, true, "recorded"},
		{"Wrong tenant", "tenantB", "resourceA", 3, 1, 3, 10, false, "wrong_target"},
		{"Wrong resource", "tenantA", "resourceB", 3, 1, 3, 10, false, "wrong_target"},
		{"Zero quantity", "tenantA", "resourceA", 3, 0, 3, 10, false, "invalid_quantity"},
		{"Extreme negative quantity", "tenantA", "resourceA", MaxBehaviorInteger, -MaxBehaviorInteger, MaxBehaviorInteger, 10, false, "invalid_quantity"},
		{"Total already at boundary", "tenantA", "resourceA", MaxBehaviorInteger, 1, MaxBehaviorInteger, 10, false, "capacity"},
		{"Reach exact boundary", "tenantA", "resourceA", MaxBehaviorInteger - 1, 1, MaxBehaviorInteger, 11, true, "recorded"},
		{"Extreme negative initial total", "tenantA", "resourceA", -MaxBehaviorInteger, 1, -MaxBehaviorInteger, 10, false, "invalid_state"},
	} {
		decision.Tests = append(decision.Tests, BehaviorCase{Name: test.name,
			Inputs: map[string]TypedValue{"tenantId": reservationText("tenantA"), "resourceId": reservationText("resourceA"), "totalReserved": reservationInteger(test.total), "acceptedSinceRestart": reservationInteger(10),
				"notice": {Type: "record", RecordRef: ref("ReservationNotice"), Record: map[string]TypedValue{"tenantId": reservationText(test.tenant), "resourceId": reservationText(test.resource), "quantity": reservationInteger(test.quantity), "remaining": reservationInteger(0)}}},
			Expected: map[string]TypedValue{"accepted": reservationBoolean(test.accepted), "totalReserved": reservationInteger(test.next), "acceptedSinceRestart": reservationInteger(test.counter), "reason": reservationText(test.reason)},
		})
	}
	bundle, issues := PrepareComponentBundle(decision, []PackageLock{})
	if len(issues) != 0 {
		panic(fmt.Sprintf("invalid reservation ledger decision: %+v", issues))
	}
	machine := StateMachine{SchemaVersion: 1, ID: "lce.reservation-ledger", Version: "1.0.0", Name: "Owned reservation ledger",
		Description: "Owns an independently committed total of typed reservation notices. Durable command receipts deduplicate the frozen generated delivery command; volatile acceptance count resets to the effective per-instance baseline. No external database or authorization enforcement is provided.",
		Author:      "LCE Studio contributors", License: "MIT", DataContracts: []DataContractLock{data},
		State: []StateSlot{
			{Name: "tenantId", Type: "string", Durability: "durable", Initial: reservationText("tenantDefault")},
			{Name: "resourceId", Type: "string", Durability: "durable", Initial: reservationText("resourceDefault")},
			{Name: "totalReserved", Type: "integer", Durability: "durable", Initial: reservationInteger(0)},
			{Name: "acceptedSinceRestart", Type: "integer", Durability: "volatile", Initial: reservationInteger(0)},
		},
		Effects: []StateEffect{},
		Commands: []StateCommand{{Name: "recordReservation", Inputs: []BehaviorPort{{Name: "notice", Type: "record", RecordRef: ref("ReservationNotice")}}, Outputs: ports, Decision: bundle,
			Bindings:       map[string]StateBinding{"tenantId": {State: "tenantId"}, "resourceId": {State: "resourceId"}, "totalReserved": {State: "totalReserved"}, "acceptedSinceRestart": {State: "acceptedSinceRestart"}, "notice": {Input: "notice"}},
			AcceptedOutput: "accepted", Writes: map[string]string{"totalReserved": "totalReserved", "acceptedSinceRestart": "acceptedSinceRestart"}, Results: map[string]string{"accepted": "accepted", "totalReserved": "totalReserved", "acceptedSinceRestart": "acceptedSinceRestart", "reason": "reason"}, Effects: []StateEffectBinding{},
		}},
	}
	digest, err := StateMachineDigest(machine)
	if err != nil {
		panic(fmt.Sprintf("invalid reservation ledger machine: %v", err))
	}
	return StateMachineLock{Machine: machine, Digest: digest}
}
