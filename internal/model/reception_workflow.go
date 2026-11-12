package model

import "fmt"

func receptionWInput(name string) WorkflowBinding        { return WorkflowBinding{Input: name} }
func receptionVar(name string) WorkflowBinding           { return WorkflowBinding{Variable: name} }
func receptionWLiteral(value TypedValue) WorkflowBinding { return WorkflowBinding{Literal: &value} }
func receptionBranch(variable, yes, no string) *WorkflowBranch {
	return &WorkflowBranch{Variable: variable, Then: yes, Else: no}
}

type receptionFlow struct {
	definition WorkflowDefinition
	refs       map[string]ComponentRef
}

func (f *receptionFlow) add(node WorkflowNode) {
	i := len(f.definition.Nodes)
	node.Position = Position{X: float64(i%4) * 400, Y: float64(i/4) * 220}
	f.definition.Nodes = append(f.definition.Nodes, node)
}
func (f *receptionFlow) decision(id, component string, inputs map[string]WorkflowBinding, assign map[string]string, next string, branch *WorkflowBranch) {
	f.add(WorkflowNode{ID: id, Name: id, Kind: "decision", Decision: &WorkflowDecision{Component: f.refs[component], Inputs: inputs, Assign: assign}, Next: next, Branch: branch})
}
func (f *receptionFlow) gate(id, variable, yes, no string) {
	f.decision(id, "lce.forward-boolean", map[string]WorkflowBinding{"value": receptionVar(variable)}, map[string]string{"gate": "result"}, "", receptionBranch("gate", yes, no))
}
func (f *receptionFlow) task(id, kind, title, instructions string, request map[string]WorkflowBinding, ports []BehaviorPort, assign map[string]string, next string, branch *WorkflowBranch) {
	ports = append(ports, receptionPort("actorRef", "string"), receptionPort("evidenceRef", "string"))
	f.add(WorkflowNode{ID: id, Name: title, Kind: "task", Task: &WorkflowTask{Kind: kind, Title: title, Instructions: instructions + " Responses are explicit synthetic fixtures; actorRef and evidenceRef record their supplied provenance. No live integration is performed.", Request: request, Response: ports, Assign: assign}, Next: next, Branch: branch})
}
func (f *receptionFlow) command(id, owner, command string, inputs map[string]WorkflowBinding, assign map[string]string, accepted, next string, branch *WorkflowBranch) {
	f.add(WorkflowNode{ID: id, Name: id, Kind: "command", Command: &WorkflowCommand{Owner: owner, Command: command, Inputs: inputs, Assign: assign, AcceptedVariable: accepted, ErrorNext: "dependencyReview"}, Next: next, Branch: branch})
}

// HospitalReceptionWorkflow composes explicit tasks with reusable locked pure
// decisions and owned commands. Every run is a bounded synthetic case experiment.
func HospitalReceptionWorkflow() WorkflowLock {
	bundles := ReceptionDecisionBundles()
	machines := ReceptionStateMachines()
	f := receptionFlow{refs: map[string]ComponentRef{}}
	for _, b := range bundles {
		f.refs[b.Root.PackageID] = b.Root
	}
	w := WorkflowDefinition{SchemaVersion: 1, ID: "health.hospital-reception", Version: "1.0.0", Name: "Hospital reception", Description: "A synthetic case-level reception workflow: explicit screening, ticket issue/call, identity and affiliation evidence, owned encounter, observations, professional release, payment policy, dispatcher choice, atomic capacity reservation and receiving acceptance. An emergency signal interrupts any active routine task, cancels the case ticket and releases its reservation before awaiting ECU acceptance. Numeric queue sequence is configured per experiment; independent runs do not share a global facility priority queue.", Author: "LCE Studio contributors", License: "MIT", DataContracts: []DataContractLock{ReceptionDataContract()}, Decisions: bundles, Machines: machines, Owners: []WorkflowOwner{}, Inputs: []BehaviorPort{}, Outputs: []BehaviorPort{}, Variables: []WorkflowVariable{}, Entry: "initialScreening", Nodes: []WorkflowNode{}, Signals: []WorkflowSignal{}}
	for _, name := range []string{"caseId", "arrivalId", "facilityId", "serviceId", "identifier"} {
		w.Inputs = append(w.Inputs, receptionPort(name, "string"))
	}
	for _, name := range []string{"capacityA", "capacityB", "queueStart"} {
		w.Inputs = append(w.Inputs, receptionPort(name, "integer"))
	}
	for _, owner := range []struct {
		id, name string
		index    int
		initial  map[string]WorkflowBinding
	}{{"queue", "Case ticket", 0, map[string]WorkflowBinding{"nextNumber": receptionWInput("queueStart")}}, {"encounter", "Case encounter", 1, map[string]WorkflowBinding{}}, {"doctorA", "Provider A slot", 2, map[string]WorkflowBinding{"resourceId": receptionWLiteral(receptionText("providerA")), "available": receptionWInput("capacityA")}}, {"doctorB", "Provider B slot", 2, map[string]WorkflowBinding{"resourceId": receptionWLiteral(receptionText("providerB")), "available": receptionWInput("capacityB")}}} {
		m := machines[owner.index]
		w.Owners = append(w.Owners, WorkflowOwner{ID: owner.id, Name: owner.name, Machine: DeliveryMachineRef{MachineID: m.Machine.ID, Version: m.Machine.Version, Digest: m.Digest}, Initial: owner.initial})
	}
	for _, name := range []string{"routineActive", "emergencyActive", "assessmentRecorded", "emergencyIdentified", "routineAllowed", "ticketIssued", "identityConfirmed", "lookupCompleted", "noAffiliation", "observationsAccepted", "feverAlert", "clinicalAuthorized", "clinicalReleased", "candidateAEligible", "candidateBEligible", "selectedA", "reservationHeld", "handoffAccepted", "doctorAccepted", "gate", "lookupRegister", "lookupConfirm"} {
		w.Variables = append(w.Variables, WorkflowVariable{Name: name, Type: "boolean", Initial: receptionBool(name == "routineActive")})
	}
	for _, name := range []string{"ticketId", "patientId", "affiliationStatus", "encounterId", "observationRef", "assessmentRef", "clinicalDisposition", "reviewAssessmentRef", "paymentDisposition", "choice", "selectedResource", "reservationToken", "responseResource", "responseCase", "handoffRef", "receivingUnit", "emergencyEncounterId"} {
		w.Variables = append(w.Variables, WorkflowVariable{Name: name, Type: "string", Initial: receptionText("")})
	}
	for _, name := range []string{"ticketNumber", "matchCount"} {
		w.Variables = append(w.Variables, WorkflowVariable{Name: name, Type: "integer", Initial: receptionInt(0)})
	}
	for _, name := range []string{"candidateA", "candidateB"} {
		w.Variables = append(w.Variables, WorkflowVariable{Name: name, Type: "record", RecordRef: receptionCandidateRef(), Initial: receptionCandidateValue("", 0)})
	}
	outputs := map[string]WorkflowBinding{}
	for _, name := range []string{"patientId", "encounterId", "ticketId", "handoffRef", "selectedResource"} {
		w.Outputs = append(w.Outputs, receptionPort(name, "string"))
		outputs[name] = receptionVar(name)
	}
	w.Outputs = append(w.Outputs, receptionPort("ticketNumber", "integer"), receptionPort("noAffiliation", "boolean"), receptionPort("reservationHeld", "boolean"))
	for _, name := range []string{"ticketNumber", "noAffiliation", "reservationHeld"} {
		outputs[name] = receptionVar(name)
	}
	w.Signals = append(w.Signals, WorkflowSignal{Name: "escalateEmergency", Inputs: []BehaviorPort{receptionPort("actorRef", "string"), receptionPort("assessmentRef", "string")}, When: "routineActive", Set: map[string]WorkflowBinding{"routineActive": receptionWLiteral(receptionBool(false)), "emergencyActive": receptionWLiteral(receptionBool(true)), "assessmentRef": receptionWInput("assessmentRef")}, Target: "beginEmergency"})
	f.definition = w
	common := func() map[string]WorkflowBinding {
		return map[string]WorkflowBinding{"caseId": receptionWInput("caseId"), "arrivalId": receptionWInput("arrivalId"), "facilityId": receptionWInput("facilityId")}
	}
	f.task("initialScreening", "human", "Initial emergency screening", "Record explicit initial screening. This form does not assess or diagnose the patient.", common(), []BehaviorPort{receptionPort("assessmentRecorded", "boolean"), receptionPort("emergencyIdentified", "boolean"), receptionPort("assessmentRef", "string")}, map[string]string{"assessmentRecorded": "assessmentRecorded", "emergencyIdentified": "emergencyIdentified", "assessmentRef": "assessmentRef"}, "entryRoute", nil)
	f.decision("entryRoute", "health.reception.entry-route", map[string]WorkflowBinding{"assessmentRecorded": receptionVar("assessmentRecorded"), "emergencyIdentified": receptionVar("emergencyIdentified")}, map[string]string{"routineAllowed": "routineAllowed"}, "", receptionBranch("emergencyIdentified", "beginEmergency", "screeningGate"))
	f.gate("screeningGate", "routineAllowed", "issueTicket", "initialScreening")
	f.command("issueTicket", "queue", "issue", map[string]WorkflowBinding{"caseId": receptionWInput("caseId")}, map[string]string{"ticketId": "ticketId", "ticketNumber": "ticketNumber"}, "ticketIssued", "", receptionBranch("ticketIssued", "callNext", "dependencyReview"))
	callReq := common()
	callReq["ticketNumber"] = receptionVar("ticketNumber")
	f.task("callNext", "human", "Call numeric ticket", "Call the displayed queue number and explicitly confirm attendance.", callReq, []BehaviorPort{receptionPort("ready", "boolean")}, map[string]string{"gate": "ready"}, "", receptionBranch("gate", "markCalled", "callNext"))
	f.command("markCalled", "queue", "call", map[string]WorkflowBinding{"caseId": receptionWInput("caseId")}, map[string]string{}, "gate", "", receptionBranch("gate", "lookupIdentity", "dependencyReview"))
	lookupReq := common()
	lookupReq["identifier"] = receptionWInput("identifier")
	f.task("lookupIdentity", "adapter", "Query identity registry", "Supply completed=false on lookup failure, even when matchCount=0. Only a completed zero-match lookup permits registration.", lookupReq, []BehaviorPort{receptionPort("completed", "boolean"), receptionPort("matchCount", "integer"), receptionPort("patientId", "string")}, map[string]string{"lookupCompleted": "completed", "matchCount": "matchCount", "patientId": "patientId"}, "classifyLookup", nil)
	f.decision("classifyLookup", "lce.lookup-disposition", map[string]WorkflowBinding{"completed": receptionVar("lookupCompleted"), "matchCount": receptionVar("matchCount")}, map[string]string{"lookupRegister": "register", "lookupConfirm": "confirm"}, "", receptionBranch("lookupRegister", "registerPatient", "identityRoute"))
	f.gate("identityRoute", "lookupConfirm", "confirmIdentity", "identityReview")
	for _, step := range []struct{ id, kind, title, instructions string }{{"registerPatient", "adapter", "Register new patient", "Create a named patient only after confirmed zero-match lookup; return confirmed=false on registration failure."}, {"confirmIdentity", "human", "Confirm existing patient identity", "Explicitly confirm the candidate identity; no demographic merge is automatic."}, {"identityReview", "human", "Review unresolved identity", "Resolve unavailable, ambiguous or unconfirmed identity explicitly. confirmed=false remains in review."}} {
		r := common()
		r["patientId"] = receptionVar("patientId")
		r["lookupCompleted"] = receptionVar("lookupCompleted")
		r["matchCount"] = receptionVar("matchCount")
		f.task(step.id, step.kind, step.title, step.instructions, r, []BehaviorPort{receptionPort("confirmed", "boolean"), receptionPort("patientId", "string")}, map[string]string{"identityConfirmed": "confirmed", "patientId": "patientId"}, "validateIdentity", nil)
	}
	f.decision("validateIdentity", "lce.confirm-named-result", map[string]WorkflowBinding{"confirmed": receptionVar("identityConfirmed"), "value": receptionVar("patientId")}, map[string]string{"gate": "valid"}, "", receptionBranch("gate", "lookupAffiliation", "identityReview"))
	for _, step := range []struct{ id, kind, title string }{{"lookupAffiliation", "adapter", "Query affiliation evidence"}, {"affiliationReview", "human", "Review unresolved affiliation"}} {
		r := common()
		r["patientId"] = receptionVar("patientId")
		f.task(step.id, step.kind, step.title, "Use verified or confirmedNone only with explicit evidence. unknown, unavailable, expired and contradictory require review.", r, []BehaviorPort{receptionPort("status", "string")}, map[string]string{"affiliationStatus": "status"}, "classifyAffiliation", nil)
	}
	f.decision("classifyAffiliation", "lce.affiliation-disposition", map[string]WorkflowBinding{"status": receptionVar("affiliationStatus")}, map[string]string{"noAffiliation": "noAffiliation", "gate": "resolved"}, "", receptionBranch("gate", "openEncounter", "affiliationReview"))
	f.command("openEncounter", "encounter", "open", map[string]WorkflowBinding{"caseId": receptionWInput("caseId"), "patientId": receptionVar("patientId")}, map[string]string{"encounterId": "encounterId"}, "gate", "", receptionBranch("gate", "recordVitals", "dependencyReview"))
	for _, step := range []struct{ id, title string }{{"recordVitals", "Record observations and vitals"}, {"observationReview", "Review missing or invalid observations"}} {
		r := common()
		r["encounterId"] = receptionVar("encounterId")
		f.task(step.id, "human", step.title, "Record whether observations are accepted and supply their reference. Missing observations remain unresolved; feverAlert is an explicit professional observation, not a diagnostic threshold calculated here.", r, []BehaviorPort{receptionPort("observationsAccepted", "boolean"), receptionPort("feverAlert", "boolean"), receptionPort("observationRef", "string"), receptionPort("assessmentRef", "string")}, map[string]string{"observationsAccepted": "observationsAccepted", "feverAlert": "feverAlert", "observationRef": "observationRef", "assessmentRef": "assessmentRef"}, "validateObservations", nil)
	}
	f.decision("validateObservations", "lce.confirm-named-result", map[string]WorkflowBinding{"confirmed": receptionVar("observationsAccepted"), "value": receptionVar("observationRef")}, map[string]string{"gate": "valid"}, "", receptionBranch("gate", "feverGate", "observationReview"))
	f.gate("feverGate", "feverAlert", "feverReview", "clinicalReview")
	for _, step := range []struct{ id, title string }{{"feverReview", "Clinician fever assessment and release"}, {"clinicalReview", "Clinician review and release"}} {
		r := common()
		r["assessmentRef"] = receptionVar("assessmentRef")
		r["observationRef"] = receptionVar("observationRef")
		r["feverAlert"] = receptionVar("feverAlert")
		f.task(step.id, "human", step.title, "A professional explicitly supplies released, review or emergency. Release must be authorized and tied to the current assessment reference; no automatic clinical clearance is inferred.", r, []BehaviorPort{receptionPort("disposition", "string"), receptionPort("authorized", "boolean"), receptionPort("assessmentRef", "string")}, map[string]string{"clinicalDisposition": "disposition", "clinicalAuthorized": "authorized", "reviewAssessmentRef": "assessmentRef"}, "classifyClinical", nil)
	}
	f.decision("classifyClinical", "lce.review-disposition", map[string]WorkflowBinding{"disposition": receptionVar("clinicalDisposition"), "authorized": receptionVar("clinicalAuthorized"), "assessmentRef": receptionVar("reviewAssessmentRef"), "currentAssessmentRef": receptionVar("assessmentRef")}, map[string]string{"clinicalReleased": "released", "emergencyIdentified": "emergency"}, "", receptionBranch("emergencyIdentified", "beginEmergency", "clinicalGate"))
	f.gate("clinicalGate", "clinicalReleased", "paymentPolicy", "feverGate")
	for _, step := range []struct{ id, kind, title string }{{"paymentPolicy", "adapter", "Evaluate payment policy"}, {"paymentReview", "human", "Review financing policy"}} {
		r := common()
		r["noAffiliation"] = receptionVar("noAffiliation")
		r["affiliationStatus"] = receptionVar("affiliationStatus")
		f.task(step.id, step.kind, step.title, "Supply explicit allow, defer or waive to continue; review, policyError or unknown remain in review. Emergency interrupts bypass this task.", r, []BehaviorPort{receptionPort("disposition", "string")}, map[string]string{"paymentDisposition": "disposition"}, "classifyPayment", nil)
	}
	f.decision("classifyPayment", "lce.payment-disposition", map[string]WorkflowBinding{"disposition": receptionVar("paymentDisposition")}, map[string]string{"gate": "proceed"}, "", receptionBranch("gate", "queryWorkforce", "paymentReview"))
	workforceReq := common()
	workforceReq["serviceId"] = receptionWInput("serviceId")
	f.task("queryWorkforce", "adapter", "Read workforce availability", "Return explicit candidate facts for providerA and providerB. Snapshot availability is only a proposal; the owned slot command is the final atomic gate.", workforceReq, []BehaviorPort{{Name: "candidateA", Type: "record", RecordRef: receptionCandidateRef()}, {Name: "candidateB", Type: "record", RecordRef: receptionCandidateRef()}}, map[string]string{"candidateA": "candidateA", "candidateB": "candidateB"}, "eligibleA", nil)
	for _, candidate := range []string{"A", "B"} {
		next := "selectDoctor"
		if candidate == "A" {
			next = "eligibleB"
		}
		f.decision("eligible"+candidate, "lce.typed-candidate-eligible", map[string]WorkflowBinding{"candidate": receptionVar("candidate" + candidate), "expectedResource": receptionWLiteral(receptionText("provider" + candidate))}, map[string]string{"candidate" + candidate + "Eligible": "eligible"}, next, nil)
	}
	dispatchReq := common()
	dispatchReq["candidateAEligible"] = receptionVar("candidateAEligible")
	dispatchReq["candidateBEligible"] = receptionVar("candidateBEligible")
	f.task("selectDoctor", "human", "Dispatcher selects eligible doctor", "Explicitly select candidateA, candidateB or wait. Selection does not reserve capacity.", dispatchReq, []BehaviorPort{receptionPort("choice", "string")}, map[string]string{"choice": "choice"}, "validateChoice", nil)
	f.decision("validateChoice", "lce.confirm-dispatch-choice", map[string]WorkflowBinding{"choice": receptionVar("choice"), "candidateAEligible": receptionVar("candidateAEligible"), "candidateBEligible": receptionVar("candidateBEligible"), "resourceA": receptionWLiteral(receptionText("providerA")), "resourceB": receptionWLiteral(receptionText("providerB"))}, map[string]string{"gate": "available", "selectedA": "selectedA", "selectedResource": "resourceId"}, "", receptionBranch("gate", "reserveChoice", "waitDoctor"))
	f.gate("reserveChoice", "selectedA", "reserveA", "reserveB")
	f.task("waitDoctor", "human", "Wait or reselect doctor", "No assignment has completed. Explicitly request another availability check; the emergency signal remains available.", common(), []BehaviorPort{receptionPort("recheck", "boolean")}, map[string]string{"gate": "recheck"}, "", receptionBranch("gate", "queryWorkforce", "waitDoctor"))
	for _, candidate := range []string{"A", "B"} {
		f.command("reserve"+candidate, "doctor"+candidate, "reserve", map[string]WorkflowBinding{"resourceId": receptionVar("selectedResource"), "caseId": receptionWInput("caseId"), "eligible": receptionVar("candidate" + candidate + "Eligible")}, map[string]string{"reservationHeld": "held", "reservationToken": "reservationToken", "doctorAccepted": "received"}, "gate", "", receptionBranch("gate", "physicianAcceptance", "waitDoctor"))
	}
	handoffReq := common()
	handoffReq["resourceId"] = receptionVar("selectedResource")
	handoffReq["reservationToken"] = receptionVar("reservationToken")
	handoffReq["encounterId"] = receptionVar("encounterId")
	f.task("physicianAcceptance", "human", "Physician accepts consultation", "Receiving clinician explicitly accepts this case and selected resource. A reservation alone is not receiving acceptance.", handoffReq, []BehaviorPort{receptionPort("accepted", "boolean"), receptionPort("resourceId", "string"), receptionPort("caseId", "string"), receptionPort("reference", "string")}, map[string]string{"handoffAccepted": "accepted", "responseResource": "resourceId", "responseCase": "caseId", "handoffRef": "reference"}, "validatePhysician", nil)
	f.decision("validatePhysician", "lce.confirm-current-handoff", map[string]WorkflowBinding{"accepted": receptionVar("handoffAccepted"), "resourceId": receptionVar("responseResource"), "expectedResource": receptionVar("selectedResource"), "caseId": receptionVar("responseCase"), "expectedCase": receptionWInput("caseId"), "reference": receptionVar("handoffRef")}, map[string]string{"gate": "valid"}, "", receptionBranch("gate", "acceptChoice", "declineChoice"))
	f.gate("acceptChoice", "selectedA", "acceptA", "acceptB")
	f.gate("declineChoice", "selectedA", "declineA", "declineB")
	for _, candidate := range []string{"A", "B"} {
		for _, op := range []string{"accept", "decline", "emergencyRelease"} {
			command, next := "release", "waitDoctor"
			var branch *WorkflowBranch
			if op == "accept" {
				command = "accept"
				next = ""
				branch = receptionBranch("gate", "routineComplete", "dependencyReview")
			}
			if op == "emergencyRelease" {
				next = "ecuAcceptance"
			}
			f.command(op+candidate, "doctor"+candidate, command, map[string]WorkflowBinding{"resourceId": receptionVar("selectedResource"), "caseId": receptionWInput("caseId")}, map[string]string{"reservationHeld": "held", "reservationToken": "reservationToken", "doctorAccepted": "received"}, "gate", next, branch)
		}
	}
	f.decision("beginEmergency", "lce.begin-interrupt", map[string]WorkflowBinding{}, map[string]string{"routineActive": "routineActive", "emergencyActive": "emergencyActive", "clinicalReleased": "clinicalReleased"}, "", receptionBranch("ticketIssued", "cancelTicket", "emergencyReservationGate"))
	f.command("cancelTicket", "queue", "cancel", map[string]WorkflowBinding{"caseId": receptionWInput("caseId")}, map[string]string{}, "gate", "emergencyReservationGate", nil)
	f.gate("emergencyReservationGate", "reservationHeld", "emergencyReleaseChoice", "ecuAcceptance")
	f.gate("emergencyReleaseChoice", "selectedA", "emergencyReleaseA", "emergencyReleaseB")
	for _, step := range []struct{ id, title string }{{"ecuAcceptance", "Emergency unit receiving acceptance"}, {"ecuPending", "Emergency unit unavailable: seek receiving acceptance"}} {
		r := common()
		r["assessmentRef"] = receptionVar("assessmentRef")
		r["patientId"] = receptionVar("patientId")
		r["previousEncounterId"] = receptionVar("encounterId")
		f.task(step.id, "human", step.title, "Explicit receiving-unit acceptance is required. accepted=false keeps emergency handoff pending. Supply the current case, receiving unit, emergency encounter and handoff reference.", r, []BehaviorPort{receptionPort("accepted", "boolean"), receptionPort("caseId", "string"), receptionPort("receivingUnit", "string"), receptionPort("encounterId", "string"), receptionPort("reference", "string")}, map[string]string{"handoffAccepted": "accepted", "responseCase": "caseId", "receivingUnit": "receivingUnit", "emergencyEncounterId": "encounterId", "handoffRef": "reference"}, "validateECU", nil)
	}
	f.decision("validateECU", "lce.confirm-exception-handoff", map[string]WorkflowBinding{"accepted": receptionVar("handoffAccepted"), "caseId": receptionVar("responseCase"), "expectedCase": receptionWInput("caseId"), "receivingUnit": receptionVar("receivingUnit"), "encounterId": receptionVar("emergencyEncounterId"), "reference": receptionVar("handoffRef")}, map[string]string{"gate": "valid"}, "", receptionBranch("gate", "emergencyComplete", "ecuPending"))
	f.task("dependencyReview", "human", "Review owned dependency failure", "An owned command did not complete. This bounded experiment keeps the case pending for review; it does not pretend a retry or external recovery succeeded.", common(), []BehaviorPort{receptionPort("reviewed", "boolean")}, map[string]string{}, "dependencyReview", nil)
	f.add(WorkflowNode{ID: "routineComplete", Name: "Consultation accepted", Kind: "end", End: &WorkflowEnd{Outcome: "consultationAccepted", Outputs: outputs}})
	emergencyOutputs := map[string]WorkflowBinding{}
	for k, v := range outputs {
		emergencyOutputs[k] = v
	}
	emergencyOutputs["encounterId"] = receptionVar("emergencyEncounterId")
	emergencyOutputs["selectedResource"] = receptionVar("receivingUnit")
	f.add(WorkflowNode{ID: "emergencyComplete", Name: "Emergency receiving unit accepted", Kind: "end", End: &WorkflowEnd{Outcome: "emergencyAccepted", Outputs: emergencyOutputs}})
	digest, err := WorkflowDigest(f.definition)
	if err != nil {
		panic(fmt.Sprintf("hospital reception: %v", err))
	}
	return WorkflowLock{Workflow: f.definition, Digest: digest}
}
