package systemrun

import (
	"context"
	"encoding/json"
	"reflect"
	"software-studio/internal/model"
	"testing"
)

func sharedFixture(t *testing.T) (Plan, Scenario) {
	p, inputs := stateFixture(t)
	invoke := func(id string, rev, qty int64, status, downstream, fault string) ScenarioStep {
		b, _ := json.Marshal(inputs)
		var owned []Inputs
		_ = json.Unmarshal(b, &owned)
		owned[0].Values["request"].Record["quantity"] = model.TypedValue{Type: "integer", Integer: &qty}
		return ScenarioStep{Kind: "invoke", Inputs: owned, Calls: []StateCall{{TaskID: "stock", CommandID: id, ExpectedRevision: &rev, Fault: fault}}, ExpectedStatuses: map[string]string{"stock": status, "summary": downstream}}
	}
	tenant, resource := "tenantDefault", "resourceDefault"
	available, zero, revision := int64(2), int64(0), int64(2)
	two := 2
	return p, Scenario{Name: "Retry and restart", Steps: []ScenarioStep{invoke("reserveA", 0, 2, "committed", "completed", ""), invoke("reserveA", 0, 2, "replayed", "completed", ""), invoke("stale", 0, 1, "revision_conflict", "skipped", ""), invoke("reserveB", 1, 1, "reply_lost", "skipped", "after_commit_before_reply"), {Kind: "restart", TaskID: "stock", ExpectedStatuses: map[string]string{"stock": "restarted"}}, invoke("reserveB", 1, 1, "replayed", "completed", "")}, Expected: []FinalExpectation{{TaskID: "stock", Revision: &revision, Receipts: &two, Effects: &two, State: map[string]model.TypedValue{"tenantId": {Type: "string", String: &tenant}, "resourceId": {Type: "string", String: &resource}, "available": {Type: "integer", Integer: &available}, "acceptedSinceRestart": {Type: "integer", Integer: &zero}}}}}
}
func TestSharedScenarioStateContinuityIndependentAssertionsAndReplay(t *testing.T) {
	p, s := sharedFixture(t)
	r, err := RunScenario(context.Background(), p, s)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Scenario.Verified || !r.Scenario.Passed || r.ClaimsEvaluated || len(r.Scenario.Assertions) != 12 {
		t.Fatalf("verdict %+v", r.Scenario)
	}
	for _, e := range r.Events {
		if e.ScenarioStep == 3 || e.ScenarioStep == 4 {
			if e.Kind == "transfer" && e.TaskID == "stock" {
				t.Fatal("undelivered reply routed", e)
			}
		}
	}
	final := r.Scenario.Final[0].Instance
	if final.Revision != 2 || len(final.Receipts) != 2 || len(final.Effects) != 2 || *final.State["available"].Integer != 2 || *final.State["acceptedSinceRestart"].Integer != 0 {
		t.Fatal(final)
	}
	again, err := RunScenario(context.Background(), p, s)
	if err != nil || !reflect.DeepEqual(again, r) {
		t.Fatal("nondeterministic or stale state", err)
	}
	wrong := int64(999)
	s.Expected[0].State["available"] = model.TypedValue{Type: "integer", Integer: &wrong}
	failed, err := RunScenario(context.Background(), p, s)
	if err != nil || failed.Scenario.Passed || !failed.Scenario.Verified {
		t.Fatal("false expectation passed", err)
	}
	s.Expected = nil
	for i := range s.Steps {
		s.Steps[i].ExpectedStatuses = nil
	}
	unverified, err := RunScenario(context.Background(), p, s)
	if err != nil || unverified.Scenario.Verified || unverified.Scenario.Passed {
		t.Fatal("empty assertions certified", err)
	}
}
func TestSharedScenarioRollbackIdentityConflictAndHistoricalReceipts(t *testing.T) {
	p, s := sharedFixture(t)
	s.Expected = nil
	s.Steps = []ScenarioStep{s.Steps[0], s.Steps[3], s.Steps[1]}
	s.Steps[1].Calls[0].Fault = ""
	for i := range s.Steps {
		s.Steps[i].ExpectedStatuses = nil
	}
	r, err := RunScenario(context.Background(), p, s)
	if err != nil {
		t.Fatal(err)
	}
	last := r.Results[len(r.Results)-1]
	if *last.Outputs["remaining"].Integer != 3 || *r.Scenario.Final[0].Instance.State["available"].Integer != 2 {
		t.Fatal("historical reply restored old state")
	}
	s.Steps[1].Calls[0].Fault = "before_commit"
	s.Steps[2].Calls[0].CommandID = "reserveB"
	s.Steps[2].Calls[0].ExpectedRevision = s.Steps[1].Calls[0].ExpectedRevision
	r, err = RunScenario(context.Background(), p, s)
	if err != nil {
		t.Fatal(err)
	}
	if r.Results[4].Status != "rolled_back" || r.Results[5].Status != "skipped" || r.Scenario.Final[0].Instance.Revision != 2 {
		t.Fatal("rollback reserved identity or advanced state", r.Scenario)
	}
	s.Steps[2].Calls[0].CommandID = "reserveA"
	r, err = RunScenario(context.Background(), p, s)
	if err != nil {
		t.Fatal(err)
	}
	if r.Results[7].Status != "id_conflict" || r.Results[8].Status != "skipped" {
		t.Fatal("identity conflict not retained", r.Results)
	}
}
func TestSharedScenarioInvalidEnvelopeAndLimitsExposeNoPartialReport(t *testing.T) {
	p, s := sharedFixture(t)
	check := func(s Scenario) {
		r, err := RunScenario(context.Background(), p, s)
		if err == nil || len(r.Events) > 0 || r.Scenario != nil {
			t.Fatal("invalid scenario produced success", err)
		}
	}
	broken := s
	broken.Steps = append([]ScenarioStep{}, s.Steps...)
	broken.Steps[0].Calls = nil
	check(broken)
	broken = s
	broken.Expected = append([]FinalExpectation{}, s.Expected...)
	broken.Expected[0].Revision = nil
	check(broken)
	broken = s
	broken.Steps = make([]ScenarioStep, 65)
	check(broken)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := RunScenario(ctx, p, s); err == nil {
		t.Fatal("cancel ignored")
	}
}
