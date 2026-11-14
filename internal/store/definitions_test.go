package store

import (
	"context"
	"encoding/json"
	"errors"
	"software-studio/internal/model"
	"software-studio/internal/system"
	"software-studio/internal/testutil"
	"testing"
)

func TestDefinitionOwnershipAndExactEditors(t *testing.T) {
	ctx := context.Background()
	db, err := Open(testutil.DatabaseURL(t))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	// A composite behavior has an editor; timing-only profiles intentionally do not.
	component := compositeLeaf("test.behavior")
	if _, _, err = db.InstallComponent(ctx, component); err != nil {
		t.Fatal(err)
	}
	if _, _, err = db.InstallDataContract(ctx, model.ExampleDataFixtures()[0].Contract.Contract); err != nil {
		t.Fatal(err)
	}
	if _, _, err = db.InstallStateMachine(ctx, model.ExampleStateScenarios()[0].Machine.Machine); err != nil {
		t.Fatal(err)
	}
	if _, _, err = db.InstallDeliveryModel(ctx, model.ExampleDeliveryScenarios()[0].Model.Model); err != nil {
		t.Fatal(err)
	}
	if _, _, err = db.InstallWorkflowPackage(ctx, model.HospitalReceptionWorkflow().Workflow); err != nil {
		t.Fatal(err)
	}
	catalog, err := db.SystemCatalog(ctx)
	if err != nil {
		t.Fatal(err)
	}
	d := system.ComponentDefinition{SchemaVersion: 1, ID: "test.reception", Name: "Reception", Version: "1.0.0", Aspects: []system.AspectRef{}}
	workspaces := map[string]string{"component": "components", "data": "data", "state": "state", "messages": "messages", "workflow": "workflows"}
	for _, e := range catalog {
		if e.Source != "installed-catalog" {
			continue
		}
		a := system.AspectRef{Slot: e.Kind, Kind: e.Kind, ID: e.ID, Version: e.Version, Digest: e.Digest}
		d.Aspects = append(d.Aspects, a)
		editor, err := db.AspectEditor(ctx, a)
		if err != nil || editor["workspace"] != workspaces[e.Kind] {
			t.Fatalf("editor %s: %+v %v", e.Kind, editor, err)
		}
		raw, _ := json.Marshal(editor["body"])
		if len(raw) < 100 {
			t.Fatalf("empty editor: %s", raw)
		}
	}
	if len(d.Aspects) != 5 {
		t.Fatalf("missing kinds: %+v", d.Aspects)
	}
	record, err := db.InstallDefinition(ctx, d)
	if err != nil {
		t.Fatal(err)
	}
	again, err := db.InstallDefinition(ctx, d)
	if err != nil || record.Digest != again.Digest {
		t.Fatalf("idempotency: %v", err)
	}
	d.Name = "Updated reception"
	if _, err = db.InstallDefinition(ctx, d); !errors.Is(err, ErrDefinitionConflict) {
		t.Fatalf("immutable: %v", err)
	}
	d.Version = "1.1.0"
	if _, err = db.InstallDefinition(ctx, d); err != nil {
		t.Fatal(err)
	}
	records, err := db.Definitions(ctx)
	if err != nil || len(records) != 2 {
		t.Fatalf("versions: %+v %v", records, err)
	}
	design := system.ReceptionExample()
	design.Components[0].Definition = system.DefinitionRef{Kind: "definition", ID: d.ID, Version: "1.0.0"}
	result, err := db.ResolveSystem(ctx, design)
	if err != nil || !result.Valid || len(result.References) != 7 {
		t.Fatalf("expanded resolution: %+v %v", result, err)
	}
	bad := d
	bad.Aspects = append([]system.AspectRef{}, d.Aspects...)
	bad.Aspects[0].Digest = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if _, err = db.InstallDefinition(ctx, bad); err == nil {
		t.Fatal("bad pin installed")
	}
	if _, err = db.AspectEditor(ctx, bad.Aspects[0]); err == nil {
		t.Fatal("bad pin opened")
	}
	if _, err = db.db.ExecContext(ctx, `UPDATE component_definitions SET body=jsonb_set(body::jsonb,'{name}','"Tampered"')::json WHERE id=$1 AND version='1.0.0'`, d.ID); err != nil {
		t.Fatal(err)
	}
	result, err = db.ResolveSystem(ctx, design)
	if err != nil || result.Valid {
		t.Fatalf("corrupt definition resolved: %+v %v", result, err)
	}
}
