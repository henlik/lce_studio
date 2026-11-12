package model

import (
	"encoding/json"
	"fmt"
)

func receptionScenarioInputs() map[string]TypedValue {
	return map[string]TypedValue{"caseId": receptionText("case-100"), "arrivalId": receptionText("arrival-100"), "facilityId": receptionText("hospital-alpha"), "serviceId": receptionText("outpatient"), "identifier": receptionText("synthetic-id-100"), "capacityA": receptionInt(1), "capacityB": receptionInt(1), "queueStart": receptionInt(100)}
}
func receptionResponse(index int, values map[string]TypedValue) WorkflowEvent {
	values["actorRef"] = receptionText("synthetic-actor")
	values["evidenceRef"] = receptionText(fmt.Sprintf("fixture-%d", index))
	return WorkflowEvent{ID: fmt.Sprintf("response%d", index), Kind: "response", TaskID: fmt.Sprintf("task%d", index), Values: values}
}
func receptionEmergencyEvent() WorkflowEvent {
	return WorkflowEvent{ID: "emergencyInterrupt", Kind: "signal", Signal: "escalateEmergency", Values: map[string]TypedValue{"actorRef": receptionText("synthetic-clinician"), "assessmentRef": receptionText("urgent-assessment")}}
}
func receptionECUResponse(index int, accepted bool) WorkflowEvent {
	return receptionResponse(index, map[string]TypedValue{"accepted": receptionBool(accepted), "caseId": receptionText("case-100"), "receivingUnit": receptionText("ecu-alpha"), "encounterId": receptionText("emergency:case-100"), "reference": receptionText("ecu-acceptance-100")})
}
func receptionRoutineEvents() []WorkflowEvent {
	return []WorkflowEvent{
		receptionResponse(1, map[string]TypedValue{"assessmentRecorded": receptionBool(true), "emergencyIdentified": receptionBool(false), "assessmentRef": receptionText("screen-100")}),
		receptionResponse(2, map[string]TypedValue{"ready": receptionBool(true)}),
		receptionResponse(3, map[string]TypedValue{"completed": receptionBool(true), "matchCount": receptionInt(1), "patientId": receptionText("patient-42")}),
		receptionResponse(4, map[string]TypedValue{"confirmed": receptionBool(true), "patientId": receptionText("patient-42")}),
		receptionResponse(5, map[string]TypedValue{"status": receptionText("verified")}),
		receptionResponse(6, map[string]TypedValue{"observationsAccepted": receptionBool(true), "feverAlert": receptionBool(false), "observationRef": receptionText("observations-100"), "assessmentRef": receptionText("assessment-100")}),
		receptionResponse(7, map[string]TypedValue{"disposition": receptionText("released"), "authorized": receptionBool(true), "assessmentRef": receptionText("assessment-100")}),
		receptionResponse(8, map[string]TypedValue{"disposition": receptionText("allow")}),
		receptionResponse(9, map[string]TypedValue{"candidateA": receptionCandidateValue("providerA", 1), "candidateB": receptionCandidateValue("providerB", 1)}),
		receptionResponse(10, map[string]TypedValue{"choice": receptionText("candidateA")}),
		receptionResponse(11, map[string]TypedValue{"accepted": receptionBool(true), "resourceId": receptionText("providerA"), "caseId": receptionText("case-100"), "reference": receptionText("consultation-acceptance-100")}),
	}
}

// These state oracles are authored from the protocol: issue/call/cancel each
// commit once, encounter opens once, and reserve/accept/release each commit once.
// They are not generated from execution output.
func receptionOwnerExpectations(queueRevision int64, encounter bool, patient string, doctorRevision int64, doctorReceipts int, held, received bool, capacity int64) []StateExpectation {
	issued := queueRevision > 0
	called := queueRevision >= 2
	cancelled := queueRevision >= 3
	caseID, ticket := "", ""
	number, next := int64(0), int64(100)
	if issued {
		caseID = "case-100"
		ticket = "ticket:case-100"
		number = 100
		next = 101
	}
	owners := []StateExpectation{{InstanceID: "queue", State: map[string]TypedValue{"caseId": receptionText(caseID), "ticketId": receptionText(ticket), "called": receptionBool(called), "cancelled": receptionBool(cancelled), "ticketNumber": receptionInt(number), "nextNumber": receptionInt(next)}, Revision: queueRevision, Receipts: int(queueRevision), Effects: 0}}
	caseID, encounterID, patientID := "", "", ""
	revision := int64(0)
	if encounter {
		caseID = "case-100"
		encounterID = "encounter:case-100"
		patientID = patient
		revision = 1
	}
	owners = append(owners, StateExpectation{InstanceID: "encounter", State: map[string]TypedValue{"caseId": receptionText(caseID), "patientId": receptionText(patientID), "encounterId": receptionText(encounterID)}, Revision: revision, Receipts: int(revision), Effects: 0})
	holder := ""
	if held {
		holder = "case-100"
		capacity = 0
	}
	owners = append(owners, StateExpectation{InstanceID: "doctorA", State: map[string]TypedValue{"resourceId": receptionText("providerA"), "available": receptionInt(capacity), "holder": receptionText(holder), "accepted": receptionBool(received)}, Revision: doctorRevision, Receipts: doctorReceipts, Effects: 0}, StateExpectation{InstanceID: "doctorB", State: map[string]TypedValue{"resourceId": receptionText("providerB"), "available": receptionInt(1), "holder": receptionText(""), "accepted": receptionBool(false)}, Revision: 0, Receipts: 0, Effects: 0})
	return owners
}
func receptionExpected(status, outcome, active string, tasks, completed, cancelled, receipts int, variables map[string]TypedValue, owners []StateExpectation) *WorkflowExpectation {
	return &WorkflowExpectation{Status: status, Outcome: outcome, ActiveTaskNode: active, Variables: variables, Owners: owners, Tasks: tasks, CompletedTasks: completed, CancelledTasks: cancelled, EventReceipts: receipts}
}

// ExampleWorkflowScenarios returns fresh portable synthetic cases with explicit,
// independently written task counts, owner state and final-variable assertions.
func ExampleWorkflowScenarios() []WorkflowExample {
	lock := HospitalReceptionWorkflow()
	examples := []WorkflowExample{}
	add := func(name string, events []WorkflowEvent, expected *WorkflowExpectation) {
		examples = append(examples, WorkflowExample{Workflow: lock, Scenario: WorkflowScenario{SchemaVersion: 1, Name: name, Inputs: receptionScenarioInputs(), Events: events, Expected: expected}})
	}
	normal := func(patient string, noAffiliation bool) *WorkflowExpectation {
		return receptionExpected("completed", "consultationAccepted", "", 11, 11, 0, 11, map[string]TypedValue{"patientId": receptionText(patient), "ticketNumber": receptionInt(100), "clinicalReleased": receptionBool(true), "noAffiliation": receptionBool(noAffiliation), "reservationHeld": receptionBool(true), "doctorAccepted": receptionBool(true), "emergencyActive": receptionBool(false)}, receptionOwnerExpectations(2, true, patient, 2, 2, true, true, 1))
	}
	add("Routine existing patient", receptionRoutineEvents(), normal("patient-42", false))
	events := receptionRoutineEvents()
	events[2].Values["matchCount"] = receptionInt(0)
	events[2].Values["patientId"] = receptionText("")
	events[3].Values["patientId"] = receptionText("new-patient-100")
	add("Register new patient", events, normal("new-patient-100", false))
	events = receptionRoutineEvents()
	events[4].Values["status"] = receptionText("confirmedNone")
	events[7].Values["disposition"] = receptionText("waive")
	add("Confirmed no affiliation with explicit waiver", events, normal("patient-42", true))
	events = receptionRoutineEvents()
	events[5].Values["feverAlert"] = receptionBool(true)
	add("Fever with explicit clinician release", events, normal("patient-42", false))
	events = receptionRoutineEvents()[:7]
	events[5].Values["feverAlert"] = receptionBool(true)
	events[6].Values["disposition"] = receptionText("review")
	add("Fever remains in clinician review", events, receptionExpected("waiting", "", "feverReview", 8, 7, 0, 7, map[string]TypedValue{"clinicalReleased": receptionBool(false), "reservationHeld": receptionBool(false)}, receptionOwnerExpectations(2, true, "patient-42", 0, 0, false, false, 1)))
	events = receptionRoutineEvents()[:7]
	events[6].Values["assessmentRef"] = receptionText("obsolete-assessment")
	add("Stale clinical release rejected", events, receptionExpected("waiting", "", "clinicalReview", 8, 7, 0, 7, map[string]TypedValue{"clinicalReleased": receptionBool(false)}, receptionOwnerExpectations(2, true, "patient-42", 0, 0, false, false, 1)))
	add("Emergency before ticket", []WorkflowEvent{receptionEmergencyEvent(), receptionECUResponse(2, true)}, receptionExpected("completed", "emergencyAccepted", "", 2, 1, 1, 2, map[string]TypedValue{"ticketNumber": receptionInt(0), "routineActive": receptionBool(false), "emergencyActive": receptionBool(true), "reservationHeld": receptionBool(false), "clinicalReleased": receptionBool(false)}, receptionOwnerExpectations(0, false, "", 0, 0, false, false, 1)))
	events = receptionRoutineEvents()[:10]
	events = append(events, receptionEmergencyEvent(), receptionECUResponse(12, true), receptionRoutineEvents()[10])
	add("Emergency after reservation fences late physician response", events, receptionExpected("completed", "emergencyAccepted", "", 12, 11, 1, 13, map[string]TypedValue{"routineActive": receptionBool(false), "emergencyActive": receptionBool(true), "reservationHeld": receptionBool(false), "doctorAccepted": receptionBool(false), "clinicalReleased": receptionBool(false)}, receptionOwnerExpectations(3, true, "patient-42", 2, 2, false, false, 1)))
	events = receptionRoutineEvents()[:10]
	events[8].Values["candidateA"] = receptionCandidateValue("providerA", 0)
	events[8].Values["candidateB"] = receptionCandidateValue("providerB", 0)
	events[9].Values["choice"] = receptionText("wait")
	add("No eligible doctor", events, receptionExpected("waiting", "", "waitDoctor", 11, 10, 0, 10, map[string]TypedValue{"reservationHeld": receptionBool(false), "doctorAccepted": receptionBool(false)}, receptionOwnerExpectations(2, true, "patient-42", 0, 0, false, false, 1)))
	for _, test := range []struct {
		name      string
		completed bool
		count     int64
	}{{"Registry unavailable is not absence", false, 0}, {"Ambiguous identity requires review", true, 2}} {
		events = receptionRoutineEvents()[:3]
		events[2].Values["completed"] = receptionBool(test.completed)
		events[2].Values["matchCount"] = receptionInt(test.count)
		events[2].Values["patientId"] = receptionText("")
		add(test.name, events, receptionExpected("waiting", "", "identityReview", 4, 3, 0, 3, map[string]TypedValue{"lookupRegister": receptionBool(false), "lookupConfirm": receptionBool(false), "ticketNumber": receptionInt(100)}, receptionOwnerExpectations(2, false, "", 0, 0, false, false, 1)))
	}
	events = receptionRoutineEvents()[:5]
	events[4].Values["status"] = receptionText("unknown")
	add("Unknown affiliation requires review", events, receptionExpected("waiting", "", "affiliationReview", 6, 5, 0, 5, map[string]TypedValue{"noAffiliation": receptionBool(false), "encounterId": receptionText("")}, receptionOwnerExpectations(2, false, "", 0, 0, false, false, 1)))
	add("ECU unavailable keeps handoff pending", []WorkflowEvent{receptionEmergencyEvent(), receptionECUResponse(2, false)}, receptionExpected("waiting", "", "ecuPending", 3, 1, 1, 2, map[string]TypedValue{"routineActive": receptionBool(false), "emergencyActive": receptionBool(true), "reservationHeld": receptionBool(false)}, receptionOwnerExpectations(0, false, "", 0, 0, false, false, 1)))
	events = receptionRoutineEvents()[:8]
	events[7].Values["disposition"] = receptionText("policyError")
	add("Payment policy failure requires review", events, receptionExpected("waiting", "", "paymentReview", 9, 8, 0, 8, map[string]TypedValue{"reservationHeld": receptionBool(false)}, receptionOwnerExpectations(2, true, "patient-42", 0, 0, false, false, 1)))
	events = receptionRoutineEvents()[:6]
	events[5].Values["observationsAccepted"] = receptionBool(false)
	events[5].Values["observationRef"] = receptionText("")
	add("Missing observations remain unresolved", events, receptionExpected("waiting", "", "observationReview", 7, 6, 0, 6, map[string]TypedValue{"clinicalReleased": receptionBool(false), "reservationHeld": receptionBool(false)}, receptionOwnerExpectations(2, true, "patient-42", 0, 0, false, false, 1)))
	events = receptionRoutineEvents()[:10]
	add("Capacity reservation denies stale availability", events, receptionExpected("waiting", "", "waitDoctor", 11, 10, 0, 10, map[string]TypedValue{"reservationHeld": receptionBool(false), "doctorAccepted": receptionBool(false)}, receptionOwnerExpectations(2, true, "patient-42", 0, 1, false, false, 0)))
	examples[len(examples)-1].Scenario.Inputs["capacityA"] = receptionInt(0)
	events = receptionRoutineEvents()[:7]
	events[6].Values["disposition"] = receptionText("emergency")
	events = append(events, receptionECUResponse(8, true))
	add("Clinician escalates emergency", events, receptionExpected("completed", "emergencyAccepted", "", 8, 8, 0, 8, map[string]TypedValue{"clinicalReleased": receptionBool(false), "emergencyActive": receptionBool(true), "reservationHeld": receptionBool(false)}, receptionOwnerExpectations(3, true, "patient-42", 0, 0, false, false, 1)))
	add("Reception ready for initial screening", []WorkflowEvent{}, receptionExpected("waiting", "", "initialScreening", 1, 0, 0, 0, map[string]TypedValue{"ticketNumber": receptionInt(0)}, receptionOwnerExpectations(0, false, "", 0, 0, false, false, 1)))
	examples = append(examples, receptionServiceDeskExample())
	// Each returned experiment owns its definition and values independently.
	encoded, err := json.Marshal(examples)
	if err != nil {
		panic(err)
	}
	var independent []WorkflowExample
	if err = json.Unmarshal(encoded, &independent); err != nil {
		panic(err)
	}
	return independent
}
