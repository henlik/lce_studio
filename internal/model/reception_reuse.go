package model

// ServiceDeskWorkflow uses exactly the reception ticket and single-slot owner
// locks in a nonclinical context. Only the orchestration and explicit tasks vary.
func ServiceDeskWorkflow() WorkflowLock {
	ticket, slot := receptionTicketOwner(), receptionAssignmentOwner()
	handoff := receptionHandoffDecision()
	f := receptionFlow{refs: map[string]ComponentRef{handoff.Root.PackageID: handoff.Root}, definition: WorkflowDefinition{SchemaVersion: 1, ID: "example.service-desk", Version: "1.0.0", Name: "Service desk reusable ownership", Description: "A nonclinical service desk reuses the exact case-ticket and single-slot assignment owners from hospital reception. Explicit human call and receiving acceptance are synthetic response tasks.", Author: "LCE Studio contributors", License: "MIT", DataContracts: []DataContractLock{}, Decisions: []ComponentBundle{handoff}, Machines: []StateMachineLock{ticket, slot}, Owners: []WorkflowOwner{{ID: "queue", Name: "Service queue", Machine: DeliveryMachineRef{MachineID: ticket.Machine.ID, Version: ticket.Machine.Version, Digest: ticket.Digest}, Initial: map[string]WorkflowBinding{"nextNumber": receptionWInput("queueStart")}}, {ID: "desk", Name: "Service desk slot", Machine: DeliveryMachineRef{MachineID: slot.Machine.ID, Version: slot.Machine.Version, Digest: slot.Digest}, Initial: map[string]WorkflowBinding{"resourceId": receptionWLiteral(receptionText("deskA"))}}}, Inputs: []BehaviorPort{receptionPort("caseId", "string"), receptionPort("queueStart", "integer")}, Outputs: []BehaviorPort{receptionPort("ticketNumber", "integer"), receptionPort("reference", "string")}, Variables: []WorkflowVariable{}, Entry: "issueTicket", Nodes: []WorkflowNode{}, Signals: []WorkflowSignal{}}}
	for _, name := range []string{"gate", "held", "received", "accepted"} {
		f.definition.Variables = append(f.definition.Variables, WorkflowVariable{Name: name, Type: "boolean", Initial: receptionBool(false)})
	}
	for _, name := range []string{"ticketId", "token", "resource", "responseCase", "reference"} {
		f.definition.Variables = append(f.definition.Variables, WorkflowVariable{Name: name, Type: "string", Initial: receptionText("")})
	}
	f.definition.Variables = append(f.definition.Variables, WorkflowVariable{Name: "ticketNumber", Type: "integer", Initial: receptionInt(0)})
	f.command("issueTicket", "queue", "issue", map[string]WorkflowBinding{"caseId": receptionWInput("caseId")}, map[string]string{"ticketId": "ticketId", "ticketNumber": "ticketNumber"}, "gate", "", receptionBranch("gate", "callCustomer", "dependencyReview"))
	f.task("callCustomer", "human", "Call service ticket", "Confirm the called customer is present.", map[string]WorkflowBinding{"caseId": receptionWInput("caseId"), "ticketNumber": receptionVar("ticketNumber")}, []BehaviorPort{receptionPort("ready", "boolean")}, map[string]string{"gate": "ready"}, "", receptionBranch("gate", "markCalled", "callCustomer"))
	f.command("markCalled", "queue", "call", map[string]WorkflowBinding{"caseId": receptionWInput("caseId")}, map[string]string{}, "gate", "", receptionBranch("gate", "reserveDesk", "dependencyReview"))
	input := map[string]WorkflowBinding{"caseId": receptionWInput("caseId"), "resourceId": receptionWLiteral(receptionText("deskA")), "eligible": receptionWLiteral(receptionBool(true))}
	f.command("reserveDesk", "desk", "reserve", input, map[string]string{"held": "held", "token": "reservationToken", "received": "received"}, "gate", "", receptionBranch("gate", "deskAcceptance", "dependencyReview"))
	f.task("deskAcceptance", "human", "Desk accepts customer", "Explicitly accept this case at deskA with a receiving reference.", map[string]WorkflowBinding{"caseId": receptionWInput("caseId"), "resourceId": receptionWLiteral(receptionText("deskA")), "token": receptionVar("token")}, []BehaviorPort{receptionPort("accepted", "boolean"), receptionPort("resourceId", "string"), receptionPort("caseId", "string"), receptionPort("reference", "string")}, map[string]string{"accepted": "accepted", "resource": "resourceId", "responseCase": "caseId", "reference": "reference"}, "validateAcceptance", nil)
	f.decision("validateAcceptance", "lce.confirm-current-handoff", map[string]WorkflowBinding{"accepted": receptionVar("accepted"), "resourceId": receptionVar("resource"), "expectedResource": receptionWLiteral(receptionText("deskA")), "caseId": receptionVar("responseCase"), "expectedCase": receptionWInput("caseId"), "reference": receptionVar("reference")}, map[string]string{"gate": "valid"}, "", receptionBranch("gate", "acceptDesk", "releaseDesk"))
	for _, command := range []string{"accept", "release"} {
		next := "deskAcceptance"
		if command == "accept" {
			next = "complete"
		}
		f.command(command+"Desk", "desk", command, map[string]WorkflowBinding{"resourceId": receptionWLiteral(receptionText("deskA")), "caseId": receptionWInput("caseId")}, map[string]string{"held": "held", "received": "received", "token": "reservationToken"}, "gate", "", receptionBranch("gate", next, "dependencyReview"))
	}
	// After a decline the released slot must be reserved again before a new handoff.
	for i := range f.definition.Nodes {
		if f.definition.Nodes[i].ID == "releaseDesk" {
			f.definition.Nodes[i].Branch.Then = "reserveDesk"
		}
	}
	f.task("dependencyReview", "human", "Review service dependency", "Keep the request pending until an operator handles the failed owned command.", map[string]WorkflowBinding{"caseId": receptionWInput("caseId")}, []BehaviorPort{receptionPort("reviewed", "boolean")}, map[string]string{"gate": "reviewed"}, "", receptionBranch("gate", "reviewClosed", "dependencyReview"))
	f.add(WorkflowNode{ID: "complete", Name: "Service accepted", Kind: "end", End: &WorkflowEnd{Outcome: "serviceAccepted", Outputs: map[string]WorkflowBinding{"ticketNumber": receptionVar("ticketNumber"), "reference": receptionVar("reference")}}})
	f.add(WorkflowNode{ID: "reviewClosed", Name: "Request closed after manual review", Kind: "end", End: &WorkflowEnd{Outcome: "serviceReviewClosed", Outputs: map[string]WorkflowBinding{"ticketNumber": receptionVar("ticketNumber"), "reference": receptionVar("reference")}}})
	digest, err := WorkflowDigest(f.definition)
	if err != nil {
		panic(err)
	}
	return WorkflowLock{Workflow: f.definition, Digest: digest}
}
func receptionServiceDeskExample() WorkflowExample {
	return WorkflowExample{Workflow: ServiceDeskWorkflow(), Scenario: WorkflowScenario{SchemaVersion: 1, Name: "Service desk reuses ticket and assignment owners", Inputs: map[string]TypedValue{"caseId": receptionText("desk-case-7"), "queueStart": receptionInt(7)}, Events: []WorkflowEvent{receptionResponse(1, map[string]TypedValue{"ready": receptionBool(true)}), receptionResponse(2, map[string]TypedValue{"accepted": receptionBool(true), "resourceId": receptionText("deskA"), "caseId": receptionText("desk-case-7"), "reference": receptionText("service-acceptance-7")})}, Expected: receptionExpected("completed", "serviceAccepted", "", 2, 2, 0, 2, map[string]TypedValue{"ticketNumber": receptionInt(7), "held": receptionBool(true), "received": receptionBool(true)}, []StateExpectation{{InstanceID: "queue", State: map[string]TypedValue{"caseId": receptionText("desk-case-7"), "ticketId": receptionText("ticket:desk-case-7"), "called": receptionBool(true), "cancelled": receptionBool(false), "nextNumber": receptionInt(8), "ticketNumber": receptionInt(7)}, Revision: 2, Receipts: 2, Effects: 0}, {InstanceID: "desk", State: map[string]TypedValue{"resourceId": receptionText("deskA"), "available": receptionInt(0), "holder": receptionText("desk-case-7"), "accepted": receptionBool(true)}, Revision: 2, Receipts: 2, Effects: 0}})}}
}
