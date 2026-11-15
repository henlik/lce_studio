package store

import (
	"context"
	"fmt"
	"reflect"
	sourceexamples "software-studio/examples/source"
	"software-studio/internal/source"
	"software-studio/internal/system"
	"software-studio/internal/systemrun"
	"software-studio/internal/testutil"
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
