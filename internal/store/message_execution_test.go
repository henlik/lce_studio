package store

import (
	"context"
	"encoding/json"
	"software-studio/internal/model"
	"software-studio/internal/system"
	"software-studio/internal/systemrun"
	"software-studio/internal/testutil"
	"testing"
)

func TestMessageExecutionSnapshotAndReplay(t *testing.T) {
	ctx := context.Background()
	db, err := Open(testutil.DatabaseURL(t))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	e, err := db.InstallMessageExample(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.InstallMessageExample(ctx); err != nil {
		t.Fatal("idempotent install", err)
	}
	p, err := db.PrepareSystemExecution(ctx, e.Design)
	if err != nil {
		if v, ok := err.(*systemrun.Invalid); ok {
			t.Fatal(v.Issues)
		}
		t.Fatal(err)
	}
	stock := systemrun.Key("stock", "stock")
	channel := p.Plan.Channels[0].ID
	zero := int64(0)
	q := int64(2)
	tenant, resource := "tenantDefault", "resourceDefault"
	var ref *model.DataTypeRef
	for _, task := range p.Plan.Tasks {
		if task.ID == stock {
			ref = task.State.Machine.Machine.Commands[0].Inputs[0].RecordRef
		}
	}
	scenario := systemrun.Scenario{Name: "Persistent transport", Steps: []systemrun.ScenarioStep{
		{Kind: "command", TaskID: stock, Calls: []systemrun.StateCall{{TaskID: stock, Command: "reserve", CommandID: "reserveA", ExpectedRevision: &zero}}, Inputs: []systemrun.Inputs{{TaskID: stock, Values: map[string]model.TypedValue{"request": {Type: "record", RecordRef: ref, Record: map[string]model.TypedValue{"tenantId": {Type: "string", String: &tenant}, "resourceId": {Type: "string", String: &resource}, "quantity": {Type: "integer", Integer: &q}}}}}}},
		{Kind: "dispatch", Message: &systemrun.MessageCall{ChannelID: channel, RouteID: "reservationToLedger", SourceCommandID: "reserveA", Alias: "notice", ExpectedRevision: &zero}},
		{Kind: "deliver", Message: &systemrun.MessageCall{ChannelID: channel, Alias: "notice", Fault: "after_recipient_commit_before_ack"}, ExpectedMessageStatus: "acknowledgement_lost"},
		{Kind: "broker_restart", Message: &systemrun.MessageCall{ChannelID: channel}},
		{Kind: "retry", Message: &systemrun.MessageCall{ChannelID: channel, Alias: "notice"}, ExpectedMessageStatus: "acknowledged"},
	}}
	run, err := db.ExecuteSystemScenario(ctx, "message-snapshot", e.Design, scenario)
	if err != nil || !run.Report.Scenario.Passed {
		t.Fatal(err, run.Report.Scenario)
	}
	saved, err := db.SystemExecution(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(run)
	b, _ := json.Marshal(saved)
	if string(a) != string(b) {
		t.Fatal("snapshot changed")
	}
	for _, modify := range []func(*system.SystemDesign){func(d *system.SystemDesign) { d.Components[0].MessageOwners = nil }, func(d *system.SystemDesign) { d.Components[0].MessageOwners[0].StateSlot = "missing" }, func(d *system.SystemDesign) { d.ExecutionProviders[1].Kind = "modeled-state" }, func(d *system.SystemDesign) { d.Components[0].Bindings = d.Components[0].Bindings[:1] }} {
		raw, _ := json.Marshal(e.Design)
		var d system.SystemDesign
		_ = json.Unmarshal(raw, &d)
		modify(&d)
		if _, err = db.PrepareSystemExecution(ctx, d); err == nil {
			t.Fatal("invalid message bindings prepared")
		}
	}
	if _, err = db.db.ExecContext(ctx, "DELETE FROM component_definitions; DELETE FROM delivery_models; DELETE FROM state_machines"); err != nil {
		t.Fatal(err)
	}
	replay, err := db.ReplaySystemExecution(ctx, "message-replay", run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if replay.Status != "matched" {
		t.Fatal(replay.Status, replay.Message, replay.Checks)
	}
	saved.Plan.Channels[0].Owners[0].TaskID = "missing"
	if r, _ := verifyExecution(ctx, saved, run.ID); r.Status == "matched" {
		t.Fatal("tampered channel reproduced")
	}
}
