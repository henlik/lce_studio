package model

import "fmt"

func receptionMachineLock(machine StateMachine) StateMachineLock {
	digest, err := StateMachineDigest(machine)
	if err != nil {
		panic(fmt.Sprintf("invalid reception owner %s: %v", machine.ID, err))
	}
	return StateMachineLock{Machine: machine, Digest: digest}
}
func receptionOwnerBase(id, name, description string) StateMachine {
	return StateMachine{SchemaVersion: 1, ID: id, Version: "1.0.0", Name: name, Description: description, Author: "LCE Studio contributors", License: "MIT", DataContracts: []DataContractLock{}, State: []StateSlot{}, Effects: []StateEffect{}, Commands: []StateCommand{}}
}
func receptionState(name, kind string, value TypedValue) StateSlot {
	return StateSlot{Name: name, Type: kind, Durability: "durable", Initial: value}
}
func receptionNonempty(d *receptionDecision, id string, value BehaviorBinding) BehaviorBinding {
	return d.node(id, "not", map[string]BehaviorBinding{"value": d.equalText(id+"Empty", value, "")})
}

// The ticket owner is scoped to one case/visit. It reuses a case-derived ticket
// across retries; it is not a global facility queue or printer integration.
func receptionTicketOwner() StateMachineLock {
	m := receptionOwnerBase("lce.case-ticket", "Case-scoped ticket lifecycle", "Owns one case's durable ticket, call and cancellation facts. Ticket IDs are derived from an explicitly unique case ID; A configured starting sequence supplies the numeric display ticket; this one-case experiment does not share a facility queue across independent runs. Printing and human attendance remain explicit tasks.")
	m.State = []StateSlot{receptionState("caseId", "string", receptionText("")), receptionState("ticketId", "string", receptionText("")), receptionState("called", "boolean", receptionBool(false)), receptionState("cancelled", "boolean", receptionBool(false)), receptionState("nextNumber", "integer", receptionInt(1)), receptionState("ticketNumber", "integer", receptionInt(0))}
	issue := newReceptionDecision("lce.issue-case-ticket", "Issue or reuse case ticket", "Proposes one case-derived ticket; another case cannot take ownership of an existing ticket.", []BehaviorPort{receptionPort("currentCase", "string"), receptionPort("currentTicket", "string"), receptionPort("cancelled", "boolean"), receptionPort("caseId", "string"), receptionPort("currentNext", "integer"), receptionPort("currentNumber", "integer")})
	empty := issue.equalText("unowned", receptionInput("currentCase"), "")
	same := issue.binary("sameCase", "equal", receptionInput("currentCase"), receptionInput("caseId"))
	owner := issue.binary("ownerAllowed", "or", empty, same)
	named := receptionNonempty(issue, "caseNamed", receptionInput("caseId"))
	active := issue.node("active", "not", map[string]BehaviorBinding{"value": receptionInput("cancelled")})
	valid := issue.binary("validOwner", "and", owner, named)
	positive := issue.binary("positiveSequence", "less_than", receptionLiteral(receptionInt(0)), receptionInput("currentNext"))
	bounded := issue.binary("boundedSequence", "less_than", receptionInput("currentNext"), receptionLiteral(receptionInt(9007199254740991)))
	sequence := issue.binary("sequenceValid", "and", positive, bounded)
	eligible := issue.binary("eligible", "and", valid, active)
	accepted := issue.binary("accepted", "and", eligible, sequence)
	first := issue.binary("firstIssue", "and", accepted, empty)
	increment := issue.choose("increment", first, receptionLiteral(receptionInt(1)), receptionLiteral(receptionInt(0)))
	nextNumber := issue.binary("nextNumber", "add", receptionInput("currentNext"), increment)
	ticketNumber := issue.choose("ticketNumber", empty, receptionInput("currentNext"), receptionInput("currentNumber"))
	newTicket := issue.binary("newTicket", "concat", receptionLiteral(receptionText("ticket:")), receptionInput("caseId"))
	ticket := issue.choose("ticket", empty, newTicket, receptionInput("currentTicket"))
	issue.publish("accepted", "boolean", accepted)
	issue.publish("caseId", "string", receptionInput("caseId"))
	issue.publish("ticketId", "string", ticket)
	issue.publish("ticketNumber", "integer", ticketNumber)
	issue.publish("nextNumber", "integer", nextNumber)
	for _, tc := range []struct {
		name, current, old, request, ticket string
		cancelled, accepted                 bool
	}{{"Issue first ticket", "", "", "caseOne", "ticket:caseOne", false, true}, {"Same case reuses ticket", "caseOne", "ticket:caseOne", "caseOne", "ticket:caseOne", false, true}, {"Different case denied", "caseOne", "ticket:caseOne", "caseTwo", "ticket:caseOne", false, false}, {"Cancelled case denied", "caseOne", "ticket:caseOne", "caseOne", "ticket:caseOne", true, false}} {
		issue.definition.Tests = append(issue.definition.Tests, BehaviorCase{Name: tc.name, Inputs: map[string]TypedValue{"currentCase": receptionText(tc.current), "currentTicket": receptionText(tc.old), "caseId": receptionText(tc.request), "cancelled": receptionBool(tc.cancelled), "currentNext": receptionInt(100), "currentNumber": receptionInt(99)}, Expected: map[string]TypedValue{"accepted": receptionBool(tc.accepted), "caseId": receptionText(tc.request), "ticketId": receptionText(tc.ticket), "ticketNumber": receptionInt(func() int64 {
			if tc.current == "" {
				return 100
			}
			return 99
		}()), "nextNumber": receptionInt(func() int64 {
			if tc.current == "" && tc.accepted {
				return 101
			}
			return 100
		}())}})
	}
	m.Commands = append(m.Commands, StateCommand{Name: "issue", Inputs: []BehaviorPort{receptionPort("caseId", "string")}, Outputs: []BehaviorPort{receptionPort("ticketId", "string"), receptionPort("ticketNumber", "integer")}, Decision: issue.bundle(), Bindings: map[string]StateBinding{"currentCase": {State: "caseId"}, "currentTicket": {State: "ticketId"}, "cancelled": {State: "cancelled"}, "caseId": {Input: "caseId"}, "currentNext": {State: "nextNumber"}, "currentNumber": {State: "ticketNumber"}}, AcceptedOutput: "accepted", Writes: map[string]string{"caseId": "caseId", "ticketId": "ticketId", "nextNumber": "nextNumber", "ticketNumber": "ticketNumber"}, Results: map[string]string{"ticketId": "ticketId", "ticketNumber": "ticketNumber"}, Effects: []StateEffectBinding{}})
	for _, operation := range []string{"call", "cancel"} {
		d := newReceptionDecision("lce."+operation+"-case-ticket", operation+" case ticket", "Checks case ownership and existing ticket before changing a durable lifecycle flag.", []BehaviorPort{receptionPort("caseId", "string"), receptionPort("currentCase", "string"), receptionPort("ticketId", "string"), receptionPort("cancelled", "boolean")})
		same := d.binary("sameCase", "equal", receptionInput("caseId"), receptionInput("currentCase"))
		exists := receptionNonempty(d, "ticketExists", receptionInput("ticketId"))
		accepted := d.binary("ownedTicket", "and", same, exists)
		if operation == "call" {
			active := d.node("active", "not", map[string]BehaviorBinding{"value": receptionInput("cancelled")})
			accepted = d.binary("accepted", "and", accepted, active)
		}
		d.publish("accepted", "boolean", accepted)
		d.publish("ticketId", "string", receptionInput("ticketId"))
		slot := "called"
		if operation == "cancel" {
			slot = "cancelled"
		}
		m.Commands = append(m.Commands, StateCommand{Name: operation, Inputs: []BehaviorPort{receptionPort("caseId", "string")}, Outputs: []BehaviorPort{receptionPort("ticketId", "string")}, Decision: d.bundle(), Bindings: map[string]StateBinding{"caseId": {Input: "caseId"}, "currentCase": {State: "caseId"}, "ticketId": {State: "ticketId"}, "cancelled": {State: "cancelled"}}, AcceptedOutput: "accepted", Writes: map[string]string{slot: "accepted"}, Results: map[string]string{"ticketId": "ticketId"}, Effects: []StateEffectBinding{}})
	}
	return receptionMachineLock(m)
}

func receptionEncounterOwner() StateMachineLock {
	m := receptionOwnerBase("lce.case-encounter", "Case-scoped encounter identity", "Owns one explicit case/patient association and stable encounter reference. Distinct case IDs produce distinct encounter IDs even when the patient is the same. This is modeled ownership, not a patient registry or live clinical database.")
	m.State = []StateSlot{receptionState("caseId", "string", receptionText("")), receptionState("patientId", "string", receptionText("")), receptionState("encounterId", "string", receptionText(""))}
	d := newReceptionDecision("lce.open-case-encounter", "Open or reuse case encounter", "Requires named case and patient, and prevents another case/patient pair from replacing an already owned encounter.", []BehaviorPort{receptionPort("currentCase", "string"), receptionPort("currentPatient", "string"), receptionPort("currentEncounter", "string"), receptionPort("caseId", "string"), receptionPort("patientId", "string")})
	empty := d.equalText("unopened", receptionInput("currentCase"), "")
	sameCase := d.binary("sameCase", "equal", receptionInput("currentCase"), receptionInput("caseId"))
	samePatient := d.binary("samePatient", "equal", receptionInput("currentPatient"), receptionInput("patientId"))
	same := d.binary("sameVisit", "and", sameCase, samePatient)
	owner := d.binary("ownerAllowed", "or", empty, same)
	caseNamed := receptionNonempty(d, "caseNamed", receptionInput("caseId"))
	patientNamed := receptionNonempty(d, "patientNamed", receptionInput("patientId"))
	named := d.binary("named", "and", caseNamed, patientNamed)
	accepted := d.binary("accepted", "and", owner, named)
	newEncounter := d.binary("newEncounter", "concat", receptionLiteral(receptionText("encounter:")), receptionInput("caseId"))
	encounter := d.choose("encounter", empty, newEncounter, receptionInput("currentEncounter"))
	d.publish("accepted", "boolean", accepted)
	d.publish("caseId", "string", receptionInput("caseId"))
	d.publish("patientId", "string", receptionInput("patientId"))
	d.publish("encounterId", "string", encounter)
	m.Commands = []StateCommand{{Name: "open", Inputs: []BehaviorPort{receptionPort("caseId", "string"), receptionPort("patientId", "string")}, Outputs: []BehaviorPort{receptionPort("encounterId", "string")}, Decision: d.bundle(), Bindings: map[string]StateBinding{"currentCase": {State: "caseId"}, "currentPatient": {State: "patientId"}, "currentEncounter": {State: "encounterId"}, "caseId": {Input: "caseId"}, "patientId": {Input: "patientId"}}, AcceptedOutput: "accepted", Writes: map[string]string{"caseId": "caseId", "patientId": "patientId", "encounterId": "encounterId"}, Results: map[string]string{"encounterId": "encounterId"}, Effects: []StateEffectBinding{}}}
	return receptionMachineLock(m)
}

// A one-slot capacity owner can be configured as a clinician, desk or device.
// Selecting a candidate never changes this owner; reserve is the atomic gate.
func receptionAssignmentOwner() StateMachineLock {
	m := receptionOwnerBase("lce.single-slot-assignment", "Single-slot assignment owner", "Atomic durable reservation, receiving acceptance and release for one resource slot. Separate owners can model separate clinicians, desks or devices. Capacity starts at zero or one; serialized commands and exact receipts model races without claiming distributed production transactions.")
	m.State = []StateSlot{receptionState("resourceId", "string", receptionText("resource")), receptionState("available", "integer", receptionInt(1)), receptionState("holder", "string", receptionText("")), receptionState("accepted", "boolean", receptionBool(false))}
	for _, operation := range []string{"reserve", "accept", "release"} {
		inputs := []BehaviorPort{receptionPort("currentResource", "string"), receptionPort("available", "integer"), receptionPort("holder", "string"), receptionPort("alreadyAccepted", "boolean"), receptionPort("resourceId", "string"), receptionPort("caseId", "string")}
		if operation == "reserve" {
			inputs = append(inputs, receptionPort("eligible", "boolean"))
		}
		d := newReceptionDecision("lce."+operation+"-single-slot", operation+" one owned resource slot", "Checks exact resource/case ownership. Reserve consumes one available slot atomically; release restores it only for its holder; accept records the receiving party's separately supplied acceptance.", inputs)
		sameResource := d.binary("sameResource", "equal", receptionInput("currentResource"), receptionInput("resourceId"))
		named := receptionNonempty(d, "caseNamed", receptionInput("caseId"))
		target := d.binary("validTarget", "and", sameResource, named)
		sameHolder := d.binary("sameHolder", "equal", receptionInput("holder"), receptionInput("caseId"))
		reserved := d.binary("reserved", "equal", receptionInput("available"), receptionLiteral(receptionInt(0)))
		owned := d.binary("ownedReservation", "and", sameHolder, reserved)
		ownedTarget := d.binary("ownedTarget", "and", target, owned)
		var accepted BehaviorBinding
		if operation == "reserve" {
			free := d.binary("oneAvailable", "equal", receptionInput("available"), receptionLiteral(receptionInt(1)))
			unheld := d.equalText("unheld", receptionInput("holder"), "")
			slot := d.binary("freeSlot", "and", free, unheld)
			freeOrOwned := d.binary("freeOrOwned", "or", slot, ownedTarget)
			eligible := d.binary("eligibleSlot", "and", freeOrOwned, receptionInput("eligible"))
			accepted = d.binary("accepted", "and", target, eligible)
		} else {
			accepted = ownedTarget
		}
		available := int64(0)
		nextHolder := receptionInput("caseId")
		held := true
		received := operation == "accept"
		if operation == "release" {
			available = 1
			nextHolder = receptionLiteral(receptionText(""))
			held = false
			received = false
		}
		d.publish("accepted", "boolean", accepted)
		d.publish("available", "integer", receptionLiteral(receptionInt(available)))
		d.publish("holder", "string", nextHolder)
		alreadyReceived := d.binary("alreadyReceived", "and", ownedTarget, receptionInput("alreadyAccepted"))
		nextReceived := receptionLiteral(receptionBool(received))
		if operation == "reserve" {
			nextReceived = alreadyReceived
		}
		actualReceived := d.choose("actualReceived", accepted, nextReceived, alreadyReceived)
		actualHeld := d.choose("actualHeld", accepted, receptionLiteral(receptionBool(held)), ownedTarget)
		d.publish("received", "boolean", actualReceived)
		d.publish("held", "boolean", actualHeld)
		prefix := d.binary("tokenPrefix", "concat", receptionInput("currentResource"), receptionLiteral(receptionText(":")))
		token := d.binary("token", "concat", prefix, receptionInput("caseId"))
		actualToken := d.choose("actualToken", actualHeld, token, receptionLiteral(receptionText("")))
		d.publish("reservationToken", "string", actualToken)
		publicInputs := []BehaviorPort{receptionPort("resourceId", "string"), receptionPort("caseId", "string")}
		bindings := map[string]StateBinding{"currentResource": {State: "resourceId"}, "available": {State: "available"}, "holder": {State: "holder"}, "alreadyAccepted": {State: "accepted"}, "resourceId": {Input: "resourceId"}, "caseId": {Input: "caseId"}}
		if operation == "reserve" {
			publicInputs = append(publicInputs, receptionPort("eligible", "boolean"))
			bindings["eligible"] = StateBinding{Input: "eligible"}
		}
		m.Commands = append(m.Commands, StateCommand{Name: operation, Inputs: publicInputs, Outputs: []BehaviorPort{receptionPort("reservationToken", "string"), receptionPort("held", "boolean"), receptionPort("received", "boolean")}, Decision: d.bundle(), Bindings: bindings, AcceptedOutput: "accepted", Writes: map[string]string{"available": "available", "holder": "holder", "accepted": "received"}, Results: map[string]string{"reservationToken": "reservationToken", "held": "held", "received": "received"}, Effects: []StateEffectBinding{}})
	}
	return receptionMachineLock(m)
}

func ReceptionStateMachines() []StateMachineLock {
	return []StateMachineLock{receptionTicketOwner(), receptionEncounterOwner(), receptionAssignmentOwner()}
}
