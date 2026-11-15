package store

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	sourceexamples "software-studio/examples/source"
	"software-studio/internal/source"
	"software-studio/internal/system"
	"software-studio/internal/systemrun"
	"software-studio/internal/testutil"
	"strings"
	"testing"
)

func TestSourceM1EquivalentExecution(t *testing.T) {
	ctx := context.Background()
	db, err := Open(testutil.DatabaseURL(t))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	compiled := source.Compile(sourceexamples.M1)
	if !compiled.Valid {
		t.Fatal(compiled.Diagnostics)
	}
	// Compile does not install or resolve dependencies implicitly.
	if _, err = db.PrepareSystemExecution(ctx, *compiled.Design); err == nil {
		t.Fatal("missing catalog accepted")
	}
	pack, err := db.InstallM1Example(ctx)
	if err != nil {
		t.Fatal(err)
	}
	original, err := db.PrepareSystemExecution(ctx, pack.Design)
	if err != nil {
		t.Fatal(err)
	}
	fromSource, err := db.PrepareSystemExecution(ctx, *compiled.Design)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(original.Plan, fromSource.Plan) {
		t.Fatal("source changed execution semantics")
	}
	cases, err := systemrun.M1Scenarios(fromSource.Plan)
	if err != nil {
		t.Fatal(err)
	}
	for i, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			result, err := db.ExecuteSystemScenario(ctx, fmt.Sprintf("source-%d", i), *compiled.Design, c.Scenario)
			if err != nil || !result.Report.Scenario.Passed {
				t.Fatal("source execution", err)
			}
			expected, err := systemrun.RunScenario(ctx, original.Plan, c.Scenario)
			if err != nil || !reflect.DeepEqual(expected, result.Report) {
				t.Fatal("reports diverged", err)
			}
			replay, err := db.ReplaySystemExecution(ctx, fmt.Sprintf("source-replay-%d", i), result.ID)
			if err != nil || replay.Status != "matched" {
				t.Fatal("replay", err)
			}
		})
	}
	bad := *compiled.Design
	bad.Components = append([]system.ComponentInstance{}, bad.Components...)
	bad.Components[0].WorkflowOwners = []system.WorkflowOwnerBinding{{Slot: "flow", OwnerID: "stock", ComponentID: "ledger", StateSlot: "ledger"}}
	if _, err = db.PrepareSystemExecution(ctx, bad); err == nil {
		t.Fatal("source mappings bypassed exact owner checks")
	}
}

func TestCompleteSourceInstallationAndSystemExecution(t *testing.T) {
	ctx := context.Background()
	db, err := Open(testutil.DatabaseURL(t))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	compiled := source.Compile(sourceexamples.StockReservation)
	if !compiled.Valid {
		t.Fatal(compiled.Diagnostics)
	}
	if _, err := db.PrepareSystemExecution(ctx, *compiled.Design); err == nil {
		t.Fatal("compile silently installed a catalog")
	}
	for i := 0; i < 2; i++ {
		if _, err = db.InstallSourceComponent(ctx, sourceexamples.StockReservation); err != nil {
			t.Fatal("install", err)
		}
	}
	if _, err = db.PrepareSystemExecution(ctx, *compiled.Design); err != nil {
		t.Fatal("generated system cannot execute", err)
	}
	want, err := source.RunTestbench(ctx, sourceexamples.StockReservation)
	if err != nil {
		t.Fatal(err)
	}
	key := systemrun.Key("subject", "owner")
	for i, c := range compiled.Testbench.Cases {
		if len(c.Scenario.Instances[0].Initial) > 0 {
			continue
		} // Per-case fixture overrides are testbench-only.
		scenario := systemrun.Scenario{Name: c.Scenario.Name, Steps: []systemrun.ScenarioStep{}, Expected: []systemrun.FinalExpectation{}}
		for j, step := range c.Scenario.Steps {
			if step.Kind == "restart" {
				scenario.Steps = append(scenario.Steps, systemrun.ScenarioStep{Kind: "restart", TaskID: key})
				continue
			}
			status := c.Steps[j].Status
			if c.Steps[j].ReplyLost {
				status = "reply_lost"
			}
			scenario.Steps = append(scenario.Steps, systemrun.ScenarioStep{Kind: "command", TaskID: key, Inputs: []systemrun.Inputs{{TaskID: key, Values: step.Inputs}}, Calls: []systemrun.StateCall{{TaskID: key, Command: step.Command, CommandID: step.CommandID, ExpectedRevision: step.ExpectedRevision, Fault: step.Fault}}, ExpectedStatuses: map[string]string{key: status}})
		}
		final := c.Scenario.Expected[0]
		scenario.Expected = append(scenario.Expected, systemrun.FinalExpectation{TaskID: key, State: final.State, Revision: &final.Revision, Receipts: &final.Receipts, Effects: &final.Effects})
		run, err := db.ExecuteSystemScenario(ctx, fmt.Sprintf("component-source-%d", i), *compiled.Design, scenario)
		if err != nil {
			t.Fatal(c.Scenario.Name, err)
		}
		if !run.Report.Scenario.Passed {
			t.Fatal(c.Scenario.Name, run.Report.Scenario.Assertions)
		}
		got := run.Report.Scenario.Final[0].Instance
		expected := want.Cases[i].Result.Instances[0]
		if !reflect.DeepEqual(got.State, expected.State) || got.Revision != expected.Revision || len(got.Receipts) != len(expected.Receipts) || len(got.Effects) != len(expected.Effects) {
			t.Fatal(c.Scenario.Name, "system state differs from testbench")
		}
		for j, receipt := range got.Receipts {
			other := expected.Receipts[j]
			if receipt.CommandID != other.CommandID || receipt.Accepted != other.Accepted || receipt.Revision != other.Revision || !reflect.DeepEqual(receipt.Outputs, other.Outputs) {
				t.Fatal(c.Scenario.Name, "receipt mismatch")
			}
		}
		for j, event := range got.Effects {
			other := expected.Effects[j]
			if event.CommandID != other.CommandID || event.Effect != other.Effect || !reflect.DeepEqual(event.Payload, other.Payload) {
				t.Fatal(c.Scenario.Name, "event mismatch")
			}
		}
		replay, err := db.ReplaySystemExecution(ctx, fmt.Sprintf("component-replay-%d", i), run.ID)
		if err != nil || replay.Status != "matched" {
			t.Fatal("source replay", err)
		}
	}
	changed := strings.Replace(sourceexamples.StockReservation, "available : integer := 5", "available : integer := 9", 1)
	if _, err = db.InstallSourceComponent(ctx, changed); !errors.Is(err, ErrStateMachineConflict) {
		t.Fatal("overwrote an immutable state artifact", err)
	}
	if _, err = db.InstallSourceComponent(ctx, strings.Replace(changed, "version \"1.0.0\"", "version \"1.0.1\"", 1)); err != nil {
		t.Fatal("new version", err)
	}
}
