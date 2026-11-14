package store

import (
	"context"
	"testing"

	"software-studio/internal/model"
	"software-studio/internal/system"
	"software-studio/internal/testutil"
)

func TestSystemCatalogResolvesAllKindsAndDetectsChangedContent(t *testing.T) {
	ctx := context.Background()
	db, err := Open(testutil.DatabaseURL(t))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	component := model.ExampleComponentPackages()[0]
	data := model.ExampleDataFixtures()[0].Contract.Contract
	state := model.ExampleStateScenarios()[0].Machine.Machine
	messages := model.ExampleDeliveryScenarios()[0].Model.Model
	workflow := model.HospitalReceptionWorkflow().Workflow
	if _, _, err = db.InstallComponent(ctx, component); err != nil {
		t.Fatal(err)
	}
	if _, _, err = db.InstallDataContract(ctx, data); err != nil {
		t.Fatal(err)
	}
	if _, _, err = db.InstallStateMachine(ctx, state); err != nil {
		t.Fatal(err)
	}
	if _, _, err = db.InstallDeliveryModel(ctx, messages); err != nil {
		t.Fatal(err)
	}
	if _, _, err = db.InstallWorkflowPackage(ctx, workflow); err != nil {
		t.Fatal(err)
	}
	design := system.ReceptionExample()
	design.Components[0].Definition = system.DefinitionRef{ID: component.ID, Version: component.Version}
	design.Components[1].Artifacts = []system.ArtifactRef{
		{Kind: "data", ID: data.ID, Version: data.Version},
		{Kind: "state", ID: state.ID, Version: state.Version},
		{Kind: "messages", ID: messages.ID, Version: messages.Version},
		{Kind: "workflow", ID: workflow.ID, Version: workflow.Version},
	}
	resolution, err := db.ResolveSystem(ctx, design)
	if err != nil || !resolution.Valid || len(resolution.References) != 6 {
		t.Fatalf("resolve: %+v %v", resolution, err)
	}
	for _, r := range resolution.References {
		if len(r.Digest) != 64 {
			t.Fatalf("missing digest: %+v", r)
		}
	}
	// Corrupt the persisted payload without updating its recorded digest.
	if _, err = db.db.ExecContext(ctx, `UPDATE data_contracts SET body=jsonb_set(body::jsonb,'{name}','"Tampered name"')::json WHERE id=$1`, data.ID); err != nil {
		t.Fatal(err)
	}
	resolution, err = db.ResolveSystem(ctx, design)
	if err != nil || resolution.Valid || len(resolution.Issues) != 1 || resolution.Issues[0].Path != "components[1].artifacts[0]" {
		t.Fatalf("corruption not localized: %+v %v", resolution, err)
	}
	// Missing exact versions never silently use another installed version.
	design.Components[0].Definition.Version = "99.0.0"
	resolution, err = db.ResolveSystem(ctx, design)
	if err != nil || len(resolution.Issues) != 2 {
		t.Fatalf("missing version: %+v %v", resolution, err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ResolveSystem(ctx, design); err == nil {
		t.Fatal("unavailable storage returned a resolution")
	}
}
