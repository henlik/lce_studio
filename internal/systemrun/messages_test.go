package systemrun

import (
	"context"
	"encoding/json"
	"software-studio/internal/model"
	"software-studio/internal/system"
	"strings"
	"testing"
)

func messagePlan(t *testing.T) Plan {
	t.Helper()
	e := system.MessageExecutionExample()
	tasks := []Task{}
	for i, id := range []string{"stock", "ledger"} {
		tasks = append(tasks, Task{ID: Key(id, id), ComponentID: id, Slot: id, Name: id, State: &StateTask{Machine: e.Model.Model.Machines[i], Provider: e.Design.ExecutionProviders[0]}})
	}
	p, err := Prepare(tasks, nil, MessageChannel{ID: Key("stock", "notices"), Model: e.Model, Provider: e.Design.ExecutionProviders[1], Owners: []MessageOwner{{"stock", tasks[0].ID}, {"ledger", tasks[1].ID}}})
	if err != nil {
		t.Fatal(err)
	}
	return p
}
func messageScenario(p Plan) Scenario {
	stock := p.Tasks[0]
	ledger := p.Tasks[1]
	channel := p.Channels[0].ID
	zero := int64(0)
	text := func(v string) model.TypedValue { return model.TypedValue{Type: "string", String: &v} }
	integer := func(v int64) model.TypedValue { return model.TypedValue{Type: "integer", Integer: &v} }
	input := map[string]model.TypedValue{"request": {Type: "record", RecordRef: stock.State.Machine.Machine.Commands[0].Inputs[0].RecordRef, Record: map[string]model.TypedValue{"tenantId": text("tenantDefault"), "resourceId": text("resourceDefault"), "quantity": integer(2)}}}
	command := func(id, fault, status string) ScenarioStep {
		return ScenarioStep{Kind: "command", TaskID: stock.ID, Calls: []StateCall{{TaskID: stock.ID, Command: "reserve", CommandID: id, ExpectedRevision: &zero, Fault: fault}}, Inputs: []Inputs{{TaskID: stock.ID, Values: input}}, ExpectedStatuses: map[string]string{stock.ID: status}}
	}
	dispatch := func(id, alias, status string) ScenarioStep {
		return ScenarioStep{Kind: "dispatch", Message: &MessageCall{ChannelID: channel, RouteID: "reservationToLedger", SourceCommandID: id, Alias: alias, ExpectedRevision: &zero}, ExpectedMessageStatus: status}
	}
	deliver := func(kind, fault, status string) ScenarioStep {
		return ScenarioStep{Kind: kind, Message: &MessageCall{ChannelID: channel, Alias: "reservationA", Fault: fault}, ExpectedMessageStatus: status}
	}
	yes := true
	one := 1
	none := 0
	rev := int64(1)
	stockState := map[string]model.TypedValue{}
	ledgerState := map[string]model.TypedValue{}
	for _, s := range stock.State.Machine.Machine.State {
		stockState[s.Name] = s.Initial
	}
	available := *stockState["available"].Integer
	stockState["available"] = integer(available - 2)
	for _, s := range ledger.State.Machine.Machine.State {
		ledgerState[s.Name] = s.Initial
	}
	ledgerState["totalReserved"] = integer(2)
	return Scenario{Name: "Stock to ledger faults", Steps: []ScenarioStep{
		command("rollback", "before_commit", "rolled_back"), dispatch("rollback", "noMessage", "missing_intent"), command("reserveA", "", "committed"), dispatch("reserveA", "reservationA", "dispatched"), deliver("deliver", "drop_before_receive", "dropped"), deliver("retry", "before_recipient_commit", "recipient_unacknowledged"), deliver("retry", "after_recipient_commit_before_ack", "acknowledgement_lost"), {Kind: "broker_restart", Message: &MessageCall{ChannelID: channel}, ExpectedMessageStatus: "broker_restarted"}, {Kind: "restart", TaskID: ledger.ID}, {Kind: "restart", TaskID: stock.ID}, deliver("retry", "", "acknowledged"), deliver("retry", "", "already_acknowledged"), command("reserveA", "", "replayed"), dispatch("reserveA", "reservationA", "dispatch_replayed")}, Expected: []FinalExpectation{{TaskID: stock.ID, State: stockState, Revision: &rev, Receipts: &one, Effects: &one}, {TaskID: ledger.ID, State: ledgerState, Revision: &rev, Receipts: &one, Effects: &none}}, ExpectedMessages: []MessageExpectation{{ChannelID: channel, DeliveryMessageExpectation: model.DeliveryMessageExpectation{Name: "noMessage", Status: "absent"}}, {ChannelID: channel, DeliveryMessageExpectation: model.DeliveryMessageExpectation{Name: "reservationA", Status: "acknowledged", Attempts: 4, Accepted: &yes}}}}
}
func TestSharedMessages(t *testing.T) {
	p := messagePlan(t)
	s := messageScenario(p)
	r, err := RunScenario(context.Background(), p, s)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Scenario.Passed || r.EngineVersion != "system-scenario-3" {
		t.Fatalf("%+v", r.Scenario)
	}
	seen := map[string]bool{}
	lostPending := false
	for _, e := range r.Events {
		seen[e.Kind] = true
		if e.Kind == "acknowledgement_lost" {
			lostPending = e.Message.Messages[0].Status == "pending" && e.Message.Messages[0].Acknowledgement == nil
		}
	}
	for _, kind := range []string{"intent_created", "dispatched", "delivered", "recipient_committed", "recipient_replayed", "acknowledged", "broker_restarted"} {
		if !seen[kind] {
			t.Fatal("missing journey", kind)
		}
	}
	if !lostPending {
		t.Fatal("lost ack resolved prematurely")
	}
	again, err := RunScenario(context.Background(), p, s)
	a, _ := json.Marshal(r)
	b, _ := json.Marshal(again)
	if err != nil || string(a) != string(b) {
		t.Fatal("not deterministic", err)
	}
	s.ExpectedMessages[1].Attempts = 3
	wrong, err := RunScenario(context.Background(), p, s)
	if err != nil || wrong.Scenario.Passed {
		t.Fatal("false assertion accepted", err)
	}
	if _, err = Run(context.Background(), p, nil); err == nil {
		t.Fatal("message semantics silently ignored")
	}
}
func TestMessageBindingsAndMalformedSteps(t *testing.T) {
	for _, change := range []func(*Plan){func(p *Plan) { p.Channels[0].Owners[0].TaskID = p.Tasks[1].ID }, func(p *Plan) { p.Channels[0].Provider.Kind = "modeled-state" }, func(p *Plan) { p.Channels[0].Owners = nil }, func(p *Plan) { p.Channels[0].Model.Digest = strings.Repeat("0", 64) }} {
		p := messagePlan(t)
		change(&p)
		if _, err := Prepare(p.Tasks, p.Links, p.Channels...); err == nil {
			t.Fatal("invalid binding accepted")
		}
	}
	for _, change := range []func(*Scenario){func(s *Scenario) { s.Steps[3].Message.ExpectedRevision = nil }, func(s *Scenario) { s.Steps[4].Message.Alias = "unknown" }, func(s *Scenario) { s.Steps[7].Message.Alias = "unexpected" }, func(s *Scenario) { s.Steps[0].Message = &MessageCall{} }, func(s *Scenario) { s.Steps[4].Message.Fault = "unknown" }} {
		p := messagePlan(t)
		s := messageScenario(p)
		change(&s)
		if _, err := RunScenario(context.Background(), p, s); err == nil {
			t.Fatal("malformed step accepted")
		}
	}
}

func TestMessageRetrySharesLaterOwnerRevision(t *testing.T) {
	p := messagePlan(t)
	s := messageScenario(p)
	ledger := p.Tasks[1]
	one := int64(1)
	q := int64(1)
	remaining := int64(3)
	tenant, resource := "tenantDefault", "resourceDefault"
	direct := ScenarioStep{Kind: "command", TaskID: ledger.ID, Calls: []StateCall{{TaskID: ledger.ID, Command: "recordReservation", CommandID: "manual", ExpectedRevision: &one}}, Inputs: []Inputs{{TaskID: ledger.ID, Values: map[string]model.TypedValue{"notice": {Type: "record", RecordRef: ledger.State.Machine.Machine.Commands[0].Inputs[0].RecordRef, Record: map[string]model.TypedValue{"tenantId": {Type: "string", String: &tenant}, "resourceId": {Type: "string", String: &resource}, "quantity": {Type: "integer", Integer: &q}, "remaining": {Type: "integer", Integer: &remaining}}}}}}, ExpectedStatuses: map[string]string{ledger.ID: "committed"}}
	s.Steps = append(s.Steps[:7], append([]ScenarioStep{direct}, s.Steps[7:]...)...)
	two := int64(2)
	receipts := 2
	three := int64(3)
	s.Expected[1].Revision = &two
	s.Expected[1].Receipts = &receipts
	s.Expected[1].State["totalReserved"] = model.TypedValue{Type: "integer", Integer: &three}
	r, err := RunScenario(context.Background(), p, s)
	if err != nil || !r.Scenario.Passed {
		t.Fatal(err, r.Scenario)
	}
	found := false
	for _, e := range r.Events {
		if e.Kind == "recipient_replayed" {
			found = true
			if e.Message.Step.State.Receipt.Revision != 1 || e.Message.Step.State.AfterRevision != 2 {
				t.Fatal("historical receipt rewound shared state")
			}
		}
	}
	if !found {
		t.Fatal("no receipt replay")
	}
}

func TestGraphCommandEmitsIntoSharedChannel(t *testing.T) {
	p := messagePlan(t)
	s := messageScenario(p)
	stock, ledger := p.Tasks[0], p.Tasks[1]
	zero := int64(0)
	one := int64(1)
	// A graph invocation touches both owners; ledger's separate business write
	// advances its revision before the subsequent broker dispatch.
	notice := map[string]model.TypedValue{}
	for k, v := range s.Steps[2].Inputs[0].Values["request"].Record {
		notice[k] = v
	}
	notice["remaining"] = model.TypedValue{Type: "integer", Integer: &one}
	s.Steps[2] = ScenarioStep{Kind: "invoke", Inputs: []Inputs{s.Steps[2].Inputs[0], {TaskID: ledger.ID, Values: map[string]model.TypedValue{"notice": {Type: "record", RecordRef: ledger.State.Machine.Machine.Commands[0].Inputs[0].RecordRef, Record: notice}}}}, Calls: []StateCall{{TaskID: stock.ID, CommandID: "reserveA", ExpectedRevision: &zero}, {TaskID: ledger.ID, CommandID: "graphLedger", ExpectedRevision: &zero}}, ExpectedStatuses: map[string]string{stock.ID: "committed", ledger.ID: "committed"}}
	s.Steps[3].Message.ExpectedRevision = &one
	s.Steps[13].Message.ExpectedRevision = &one
	two := int64(2)
	n := 2
	four := int64(4)
	s.Expected[1].Revision = &two
	s.Expected[1].Receipts = &n
	s.Expected[1].State["totalReserved"] = model.TypedValue{Type: "integer", Integer: &four}
	r, err := RunScenario(context.Background(), p, s)
	if err != nil || !r.Scenario.Passed {
		t.Fatal(err, r.Scenario)
	}
}
