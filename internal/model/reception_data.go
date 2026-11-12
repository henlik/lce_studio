package model

import "fmt"

func ReceptionDataContract() DataContractLock {
	c := DataContract{SchemaVersion: 1, ID: "health.reception-values", Version: "1.0.0", Name: "Synthetic reception adapter facts", Description: "Explicit candidate eligibility evidence for bounded reception experiments. Fields are supplied by a synthetic workforce adapter, not inferred by the runner.", Author: "LCE Studio contributors", License: "MIT", Records: []DataRecord{{Name: "CandidateFacts", Fields: []DataField{{Name: "resourceId", Type: "string"}, {Name: "facilityMatches", Type: "boolean"}, {Name: "serviceMatches", Type: "boolean"}, {Name: "qualified", Type: "boolean"}, {Name: "onDuty", Type: "boolean"}, {Name: "available", Type: "integer"}}}}, Entities: []DataEntity{}, Relationships: []DataRelationship{}}
	digest, err := DataContractDigest(c)
	if err != nil {
		panic(err)
	}
	return DataContractLock{Contract: c, Digest: digest}
}
func receptionCandidateRef() *DataTypeRef {
	c := ReceptionDataContract()
	return &DataTypeRef{ContractID: c.Contract.ID, Version: c.Contract.Version, Digest: c.Digest, Record: "CandidateFacts"}
}
func receptionCandidateValue(resource string, available int64) TypedValue {
	return TypedValue{Type: "record", RecordRef: receptionCandidateRef(), Record: map[string]TypedValue{"resourceId": receptionText(resource), "facilityMatches": receptionBool(true), "serviceMatches": receptionBool(true), "qualified": receptionBool(true), "onDuty": receptionBool(true), "available": receptionInt(available)}}
}
func receptionTypedCandidateDecision() ComponentBundle {
	scalar := receptionCandidateDecision()
	d := newReceptionDecision("lce.typed-candidate-eligible", "Read typed candidate eligibility", "Reads exact adapter facts and checks the configured resource identity before reusing the scalar eligibility decision.", []BehaviorPort{{Name: "candidate", Type: "record", RecordRef: receptionCandidateRef()}, receptionPort("expectedResource", "string")})
	d.definition.SchemaVersion = 3
	d.definition.DataContracts = []DataContractLock{ReceptionDataContract()}
	bindings := map[string]BehaviorBinding{}
	for _, field := range []string{"resourceId", "facilityMatches", "serviceMatches", "qualified", "onDuty", "available"} {
		d.definition.Behavior.Nodes = append(d.definition.Behavior.Nodes, BehaviorNode{ID: field, Op: "record_get", Field: field, Inputs: map[string]BehaviorBinding{"value": receptionInput("candidate")}, Position: Position{X: float64(len(bindings)) * 280, Y: 0}})
		bindings[field] = receptionOutput(field)
	}
	delete(bindings, "resourceId")
	ref := scalar.Root
	d.definition.Behavior.Nodes = append(d.definition.Behavior.Nodes, BehaviorNode{ID: "eligibility", Component: &ref, Inputs: bindings, Position: Position{X: 360, Y: 200}})
	same := d.binary("sameResource", "equal", receptionOutput("resourceId"), receptionInput("expectedResource"))
	allowed := d.binary("allowed", "and", same, BehaviorBinding{Node: "eligibility", Port: "eligible"})
	d.publish("eligible", "boolean", allowed)
	for i, name := range []string{"Eligible known resource", "Wrong resource", "No capacity"} {
		candidate := receptionCandidateValue("providerA", 1)
		if i == 1 {
			candidate.Record["resourceId"] = receptionText("other")
		}
		if i == 2 {
			candidate.Record["available"] = receptionInt(0)
		}
		d.definition.Tests = append(d.definition.Tests, BehaviorCase{Name: name, Inputs: map[string]TypedValue{"candidate": candidate, "expectedResource": receptionText("providerA")}, Expected: map[string]TypedValue{"eligible": receptionBool(i == 0)}})
	}
	bundle, issues := PrepareComponentBundle(d.definition, scalar.Packages)
	if len(issues) > 0 {
		panic(fmt.Sprintf("typed candidate: %+v", issues))
	}
	return bundle
}
func receptionBooleanDecision() ComponentBundle {
	d := newReceptionDecision("lce.forward-boolean", "Branch on explicit boolean", "Forwards an explicit boolean without deriving additional authority.", []BehaviorPort{receptionPort("value", "boolean")})
	d.publish("result", "boolean", receptionInput("value"))
	return d.bundle()
}
func receptionNamedDecision() ComponentBundle {
	d := newReceptionDecision("lce.confirm-named-result", "Confirm named response", "Accepts a named value only when an explicit response confirms it.", []BehaviorPort{receptionPort("confirmed", "boolean"), receptionPort("value", "string")})
	valid := d.binary("valid", "and", receptionInput("confirmed"), receptionNonempty(d, "named", receptionInput("value")))
	d.publish("valid", "boolean", valid)
	return d.bundle()
}
func receptionEmergencyDecision() ComponentBundle {
	d := newReceptionDecision("lce.begin-interrupt", "Begin exclusive interrupt path", "Marks the ordinary path inactive and clears its release flag before cleanup and explicit receiving acceptance.", []BehaviorPort{})
	d.publish("routineActive", "boolean", receptionLiteral(receptionBool(false)))
	d.publish("emergencyActive", "boolean", receptionLiteral(receptionBool(true)))
	d.publish("clinicalReleased", "boolean", receptionLiteral(receptionBool(false)))
	return d.bundle()
}
func receptionHandoffDecision() ComponentBundle {
	d := newReceptionDecision("lce.confirm-current-handoff", "Confirm receiving handoff", "Requires explicit acceptance tied to the current case and selected resource, with a nonempty receiving reference.", []BehaviorPort{receptionPort("accepted", "boolean"), receptionPort("resourceId", "string"), receptionPort("expectedResource", "string"), receptionPort("caseId", "string"), receptionPort("expectedCase", "string"), receptionPort("reference", "string")})
	resource := d.binary("resource", "equal", receptionInput("resourceId"), receptionInput("expectedResource"))
	caseMatch := d.binary("caseMatch", "equal", receptionInput("caseId"), receptionInput("expectedCase"))
	matches := d.binary("matches", "and", resource, caseMatch)
	named := receptionNonempty(d, "reference", receptionInput("reference"))
	evidence := d.binary("evidence", "and", matches, named)
	valid := d.binary("valid", "and", evidence, receptionInput("accepted"))
	d.publish("valid", "boolean", valid)
	return d.bundle()
}
func receptionECUDecision() ComponentBundle {
	d := newReceptionDecision("lce.confirm-exception-handoff", "Confirm exceptional receiving handoff", "Requires explicit acceptance for the current case, named receiving unit, encounter and receiving reference. This is a response check, not emergency triage.", []BehaviorPort{receptionPort("accepted", "boolean"), receptionPort("caseId", "string"), receptionPort("expectedCase", "string"), receptionPort("receivingUnit", "string"), receptionPort("encounterId", "string"), receptionPort("reference", "string")})
	match := d.binary("caseMatch", "equal", receptionInput("caseId"), receptionInput("expectedCase"))
	valid := d.binary("acceptedCase", "and", match, receptionInput("accepted"))
	for _, field := range []string{"receivingUnit", "encounterId", "reference"} {
		valid = d.binary(field+"Valid", "and", valid, receptionNonempty(d, field+"Named", receptionInput(field)))
	}
	d.publish("valid", "boolean", valid)
	return d.bundle()
}
