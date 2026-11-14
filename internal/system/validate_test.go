package system

import "testing"

func TestExampleAndReferenceValidation(t *testing.T) {
	if v := Validate(ReceptionExample()); !v.Valid {
		t.Fatalf("example invalid: %+v", v)
	}
	cases := []struct {
		name   string
		mutate func(*SystemDesign)
	}{
		{"floating version", func(d *SystemDesign) { d.Components[0].Definition.Version = "latest" }},
		{"dangling connection", func(d *SystemDesign) { d.Connections[0].Target = "missing" }},
		{"database placement", func(d *SystemDesign) { d.DeploymentPlans[0].Placements[0].ResourceID = "encounters-db" }},
		{"missing plan binding", func(d *SystemDesign) {
			d.DeploymentPlans = append(d.DeploymentPlans, DeploymentPlan{ID: "other", Resources: []Resource{{ID: "host", Kind: "compute"}}})
		}},
		{"unknown claim", func(d *SystemDesign) { d.Contract.Claims[0].Kind = "run_shell" }},
		{"duplicate claim", func(d *SystemDesign) { d.Contract.Claims[1].ID = d.Contract.Claims[0].ID }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := ReceptionExample()
			tc.mutate(&d)
			if Validate(d).Valid {
				t.Fatal("invalid design accepted")
			}
		})
	}
}

func TestPersistenceMustPrecedeSuccessfulResponse(t *testing.T) {
	events := []Event{{Sequence: 1, Kind: "request_received", RequestID: "r"}, {Sequence: 2, Kind: "response_success", RequestID: "r", EncounterID: "e"}, {Sequence: 3, Kind: "persisted", RequestID: "r", EncounterID: "e"}}
	v := Evaluate(ReceptionExample().Contract, events)
	if v[1].Status != "violated" || v[2].Status != "violated" {
		t.Fatalf("late persistence accepted: %+v", v)
	}
}

func TestMalformedSequenceAndMissingReceiptAreInconclusive(t *testing.T) {
	for _, events := range [][]Event{
		{{Sequence: 1, Kind: "request_received", RequestID: "r"}, {Sequence: 1, Kind: "persisted", RequestID: "r", EncounterID: "e"}, {Sequence: 2, Kind: "response_success", RequestID: "r", EncounterID: "e"}},
		{{Sequence: 1, Kind: "persisted", RequestID: "r", EncounterID: "e"}, {Sequence: 2, Kind: "request_received", RequestID: "r"}, {Sequence: 3, Kind: "response_success", RequestID: "r", EncounterID: "e"}},
	} {
		for _, v := range Evaluate(ReceptionExample().Contract, events) {
			if v.Status != "inconclusive" {
				t.Fatalf("malformed trace accepted: %+v", v)
			}
		}
	}
}
