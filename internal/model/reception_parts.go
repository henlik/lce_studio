package model

import "fmt"

// These decisions consume explicit synthetic task/adapter facts. They do not
// diagnose, authenticate people, query a registry, or perform external effects.
type receptionDecision struct {
	definition ComponentPackage
}

func receptionText(value string) TypedValue             { return TypedValue{Type: "string", String: &value} }
func receptionBool(value bool) TypedValue               { return TypedValue{Type: "boolean", Boolean: &value} }
func receptionInt(value int64) TypedValue               { return TypedValue{Type: "integer", Integer: &value} }
func receptionInput(name string) BehaviorBinding        { return BehaviorBinding{Input: name} }
func receptionOutput(node string) BehaviorBinding       { return BehaviorBinding{Node: node, Port: "result"} }
func receptionLiteral(value TypedValue) BehaviorBinding { return BehaviorBinding{Literal: &value} }
func receptionPort(name, kind string) BehaviorPort      { return BehaviorPort{Name: name, Type: kind} }

func newReceptionDecision(id, name, description string, inputs []BehaviorPort) *receptionDecision {
	return &receptionDecision{definition: ComponentPackage{SchemaVersion: 2, ID: id, Version: "1.0.0", Name: name, Description: description, Author: "LCE Studio contributors", License: "MIT", Kernel: "composite", Capabilities: []string{"design", "evaluate"}, Behavior: &Behavior{Inputs: inputs, Outputs: []BehaviorPort{}, Nodes: []BehaviorNode{}, Bindings: map[string]BehaviorBinding{}}, Tests: []BehaviorCase{}}}
}
func (d *receptionDecision) node(id, op string, inputs map[string]BehaviorBinding) BehaviorBinding {
	index := len(d.definition.Behavior.Nodes)
	d.definition.Behavior.Nodes = append(d.definition.Behavior.Nodes, BehaviorNode{ID: id, Op: op, Inputs: inputs, Position: Position{X: float64(index%4) * 360, Y: float64(index/4) * 200}})
	return receptionOutput(id)
}
func (d *receptionDecision) binary(id, op string, left, right BehaviorBinding) BehaviorBinding {
	return d.node(id, op, map[string]BehaviorBinding{"left": left, "right": right})
}
func (d *receptionDecision) choose(id string, condition, yes, no BehaviorBinding) BehaviorBinding {
	return d.node(id, "select", map[string]BehaviorBinding{"condition": condition, "trueValue": yes, "falseValue": no})
}
func (d *receptionDecision) equalText(id string, value BehaviorBinding, text string) BehaviorBinding {
	return d.binary(id, "equal", value, receptionLiteral(receptionText(text)))
}
func (d *receptionDecision) publish(name, kind string, source BehaviorBinding) {
	d.definition.Behavior.Outputs = append(d.definition.Behavior.Outputs, receptionPort(name, kind))
	d.definition.Behavior.Bindings[name] = source
}
func (d *receptionDecision) bundle() ComponentBundle {
	bundle, issues := PrepareComponentBundle(d.definition, []PackageLock{})
	if len(issues) != 0 {
		panic(fmt.Sprintf("invalid reception decision %s: %+v", d.definition.ID, issues))
	}
	return bundle
}

func receptionLookupDecision() ComponentBundle {
	d := newReceptionDecision("lce.lookup-disposition", "Classify explicit lookup result", "Distinguishes confirmed zero/one/many matches from unavailable or invalid lookup results. No lookup or identity verification is performed.", []BehaviorPort{receptionPort("completed", "boolean"), receptionPort("matchCount", "integer")})
	valid := d.binary("validCount", "less_or_equal", receptionLiteral(receptionInt(0)), receptionInput("matchCount"))
	zero := d.binary("zeroMatches", "equal", receptionInput("matchCount"), receptionLiteral(receptionInt(0)))
	one := d.binary("oneMatch", "equal", receptionInput("matchCount"), receptionLiteral(receptionInt(1)))
	known := d.binary("knownResult", "and", receptionInput("completed"), valid)
	matched := d.choose("matchedRoute", one, receptionLiteral(receptionText("confirm")), receptionLiteral(receptionText("review")))
	completed := d.choose("completedRoute", zero, receptionLiteral(receptionText("register")), matched)
	route := d.choose("route", known, completed, receptionLiteral(receptionText("review")))
	d.publish("route", "string", route)
	d.publish("register", "boolean", d.binary("canRegister", "and", known, zero))
	d.publish("confirm", "boolean", d.binary("canConfirm", "and", known, one))
	for _, test := range []struct {
		name      string
		completed bool
		count     int64
		route     string
	}{{"Confirmed absence", true, 0, "register"}, {"One candidate needs confirmation", true, 1, "confirm"}, {"Ambiguous matches", true, 2, "review"}, {"Unavailable is not absence", false, 0, "review"}, {"Invalid negative count", true, -1, "review"}} {
		d.definition.Tests = append(d.definition.Tests, BehaviorCase{Name: test.name, Inputs: map[string]TypedValue{"completed": receptionBool(test.completed), "matchCount": receptionInt(test.count)}, Expected: map[string]TypedValue{"route": receptionText(test.route), "register": receptionBool(test.route == "register"), "confirm": receptionBool(test.route == "confirm")}})
	}
	return d.bundle()
}

func receptionAffiliationDecision() ComponentBundle {
	d := newReceptionDecision("lce.affiliation-disposition", "Classify affiliation evidence", "Only explicit verified or confirmedNone evidence permits routine continuation. Unknown, unavailable, expired and contradictory evidence remains unresolved; absence is never inferred.", []BehaviorPort{receptionPort("status", "string")})
	verified := d.equalText("verified", receptionInput("status"), "verified")
	none := d.equalText("confirmedNone", receptionInput("status"), "confirmedNone")
	resolved := d.binary("resolved", "or", verified, none)
	route := d.choose("route", resolved, receptionLiteral(receptionText("continue")), receptionLiteral(receptionText("review")))
	d.publish("route", "string", route)
	d.publish("noAffiliation", "boolean", none)
	d.publish("resolved", "boolean", resolved)
	for _, status := range []string{"verified", "confirmedNone", "unknown", "unavailable", "expired", "contradictory"} {
		route := "review"
		if status == "verified" || status == "confirmedNone" {
			route = "continue"
		}
		d.definition.Tests = append(d.definition.Tests, BehaviorCase{Name: status, Inputs: map[string]TypedValue{"status": receptionText(status)}, Expected: map[string]TypedValue{"route": receptionText(route), "noAffiliation": receptionBool(status == "confirmedNone"), "resolved": receptionBool(route == "continue")}})
	}
	return d.bundle()
}

func receptionClinicalDecision() ComponentBundle {
	d := newReceptionDecision("lce.review-disposition", "Record professional review disposition", "Release requires an explicit authorized response tied to the current assessment. An emergency disposition takes precedence over ordinary release. This decision performs no clinical assessment or treatment.", []BehaviorPort{receptionPort("disposition", "string"), receptionPort("authorized", "boolean"), receptionPort("assessmentRef", "string"), receptionPort("currentAssessmentRef", "string")})
	emergency := d.equalText("emergency", receptionInput("disposition"), "emergency")
	released := d.equalText("released", receptionInput("disposition"), "released")
	current := d.binary("currentAssessment", "equal", receptionInput("assessmentRef"), receptionInput("currentAssessmentRef"))
	named := d.node("assessmentNamed", "not", map[string]BehaviorBinding{"value": d.equalText("emptyAssessment", receptionInput("assessmentRef"), "")})
	authority := d.binary("currentAuthority", "and", receptionInput("authorized"), current)
	valid := d.binary("validAssessment", "and", authority, named)
	allow := d.binary("releaseAllowed", "and", valid, released)
	routine := d.choose("routineRoute", allow, receptionLiteral(receptionText("released")), receptionLiteral(receptionText("review")))
	route := d.choose("route", emergency, receptionLiteral(receptionText("emergency")), routine)
	d.publish("route", "string", route)
	d.publish("released", "boolean", allow)
	d.publish("emergency", "boolean", emergency)
	for _, test := range []struct {
		name, disposition, assessment, current string
		authorized                             bool
		route                                  string
	}{{"Explicit current release", "released", "assessment1", "assessment1", true, "released"}, {"Review remains pending", "review", "assessment1", "assessment1", true, "review"}, {"Old assessment cannot clear new review", "released", "assessment1", "assessment2", true, "review"}, {"Unauthorized release rejected", "released", "assessment1", "assessment1", false, "review"}, {"Unnamed assessment rejected", "released", "", "", true, "review"}, {"Emergency bypasses routine paperwork", "emergency", "", "assessment2", false, "emergency"}} {
		d.definition.Tests = append(d.definition.Tests, BehaviorCase{Name: test.name, Inputs: map[string]TypedValue{"disposition": receptionText(test.disposition), "authorized": receptionBool(test.authorized), "assessmentRef": receptionText(test.assessment), "currentAssessmentRef": receptionText(test.current)}, Expected: map[string]TypedValue{"route": receptionText(test.route), "released": receptionBool(test.route == "released"), "emergency": receptionBool(test.route == "emergency")}})
	}
	return d.bundle()
}

func receptionPaymentDecision() ComponentBundle {
	d := newReceptionDecision("lce.payment-disposition", "Interpret explicit payment policy", "Consumes an explicit policy disposition. Allow, defer and waive permit routine continuation; review, error and unknown values require review. It does not charge money, infer coverage or block emergency routing.", []BehaviorPort{receptionPort("disposition", "string")})
	allow := d.equalText("allow", receptionInput("disposition"), "allow")
	deferRoute := d.equalText("defer", receptionInput("disposition"), "defer")
	waive := d.equalText("waive", receptionInput("disposition"), "waive")
	alternative := d.binary("alternatives", "or", deferRoute, waive)
	proceed := d.binary("proceed", "or", allow, alternative)
	route := d.choose("route", proceed, receptionLiteral(receptionText("proceed")), receptionLiteral(receptionText("review")))
	d.publish("route", "string", route)
	d.publish("proceed", "boolean", proceed)
	for _, status := range []string{"allow", "defer", "waive", "review", "policyError", "unknown"} {
		route := "review"
		if status == "allow" || status == "defer" || status == "waive" {
			route = "proceed"
		}
		d.definition.Tests = append(d.definition.Tests, BehaviorCase{Name: status, Inputs: map[string]TypedValue{"disposition": receptionText(status)}, Expected: map[string]TypedValue{"route": receptionText(route), "proceed": receptionBool(route == "proceed")}})
	}
	return d.bundle()
}

func receptionCandidateDecision() ComponentBundle {
	d := newReceptionDecision("lce.candidate-eligible", "Check explicit candidate eligibility", "Combines explicit authority-supplied facility/service/qualification/duty facts and positive capacity. The result is only a proposal; the capacity owner must reserve atomically.", []BehaviorPort{receptionPort("facilityMatches", "boolean"), receptionPort("serviceMatches", "boolean"), receptionPort("qualified", "boolean"), receptionPort("onDuty", "boolean"), receptionPort("available", "integer")})
	place := d.binary("rightPlace", "and", receptionInput("facilityMatches"), receptionInput("serviceMatches"))
	professional := d.binary("onQualifiedDuty", "and", receptionInput("qualified"), receptionInput("onDuty"))
	eligible := d.binary("eligibleAuthority", "and", place, professional)
	capacity := d.binary("hasCapacity", "less_than", receptionLiteral(receptionInt(0)), receptionInput("available"))
	allowed := d.binary("eligible", "and", eligible, capacity)
	d.publish("eligible", "boolean", allowed)
	for i, name := range []string{"Eligible", "Wrong facility", "Wrong service", "Not qualified", "Off duty", "Full", "Invalid capacity"} {
		values := map[string]TypedValue{"facilityMatches": receptionBool(true), "serviceMatches": receptionBool(true), "qualified": receptionBool(true), "onDuty": receptionBool(true), "available": receptionInt(1)}
		switch i {
		case 1:
			values["facilityMatches"] = receptionBool(false)
		case 2:
			values["serviceMatches"] = receptionBool(false)
		case 3:
			values["qualified"] = receptionBool(false)
		case 4:
			values["onDuty"] = receptionBool(false)
		case 5:
			values["available"] = receptionInt(0)
		case 6:
			values["available"] = receptionInt(-1)
		}
		d.definition.Tests = append(d.definition.Tests, BehaviorCase{Name: name, Inputs: values, Expected: map[string]TypedValue{"eligible": receptionBool(i == 0)}})
	}
	return d.bundle()
}

func receptionDispatcherDecision() ComponentBundle {
	d := newReceptionDecision("lce.confirm-dispatch-choice", "Validate a human dispatcher choice", "A dispatcher explicitly selects candidateA or candidateB. Only an eligible selected candidate advances to atomic reservation. No candidate is silently chosen when neither is eligible.", []BehaviorPort{receptionPort("choice", "string"), receptionPort("candidateAEligible", "boolean"), receptionPort("candidateBEligible", "boolean"), receptionPort("resourceA", "string"), receptionPort("resourceB", "string")})
	a := d.binary("allowA", "and", d.equalText("choseA", receptionInput("choice"), "candidateA"), receptionInput("candidateAEligible"))
	b := d.binary("allowB", "and", d.equalText("choseB", receptionInput("choice"), "candidateB"), receptionInput("candidateBEligible"))
	bRoute := d.choose("bRoute", b, receptionLiteral(receptionText("candidateB")), receptionLiteral(receptionText("wait")))
	route := d.choose("route", a, receptionLiteral(receptionText("candidateA")), bRoute)
	d.publish("route", "string", route)
	available := d.binary("available", "or", a, b)
	d.publish("available", "boolean", available)
	d.publish("selectedA", "boolean", a)
	chosen := d.choose("chosenResource", a, receptionInput("resourceA"), receptionInput("resourceB"))
	d.publish("resourceId", "string", d.choose("resourceId", available, chosen, receptionLiteral(receptionText(""))))
	for _, test := range []struct {
		name, choice string
		a, b         bool
		route        string
	}{{"Select eligible A", "candidateA", true, true, "candidateA"}, {"Select eligible B", "candidateB", true, true, "candidateB"}, {"Selected A ineligible", "candidateA", false, true, "wait"}, {"No eligible candidate", "candidateB", false, false, "wait"}, {"Explicitly wait", "wait", true, true, "wait"}} {
		resource := ""
		if test.route == "candidateA" {
			resource = "resourceA"
		} else if test.route == "candidateB" {
			resource = "resourceB"
		}
		d.definition.Tests = append(d.definition.Tests, BehaviorCase{Name: test.name, Inputs: map[string]TypedValue{"choice": receptionText(test.choice), "candidateAEligible": receptionBool(test.a), "candidateBEligible": receptionBool(test.b), "resourceA": receptionText("resourceA"), "resourceB": receptionText("resourceB")}, Expected: map[string]TypedValue{"route": receptionText(test.route), "available": receptionBool(test.route != "wait"), "selectedA": receptionBool(test.route == "candidateA"), "resourceId": receptionText(resource)}})
	}
	return d.bundle()
}

// ReceptionDecisionBundles returns independently constructed reusable decisions.
func ReceptionDecisionBundles() []ComponentBundle {
	return []ComponentBundle{receptionEntryDecision(), receptionLookupDecision(), receptionAffiliationDecision(), receptionClinicalDecision(), receptionPaymentDecision(), receptionCandidateDecision(), receptionDispatcherDecision(), receptionTypedCandidateDecision(), receptionBooleanDecision(), receptionNamedDecision(), receptionEmergencyDecision(), receptionHandoffDecision(), receptionECUDecision()}
}
