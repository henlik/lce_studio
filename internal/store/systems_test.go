package store

import (
	"context"
	"errors"
	"testing"

	"software-studio/internal/conformance"
	"software-studio/internal/model"
	"software-studio/internal/system"
	"software-studio/internal/testutil"
)

func TestSystemPersistenceAndImmutableEvidence(t *testing.T) {
	ctx := context.Background()
	dsn := testutil.DatabaseURL(t)
	db, err := Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { db.Close() }()
	if items, err := db.Systems(ctx); err != nil || items == nil || len(items) != 0 {
		t.Fatalf("empty systems: %v %v", items, err)
	}
	legacy := model.ExampleProject()
	if err := db.SaveProject(ctx, legacy); err != nil {
		t.Fatal(err)
	}
	design := system.ReceptionExample()
	if err := db.SaveSystem(ctx, design); err != nil {
		t.Fatal(err)
	}
	evidence, err := NewSystemEvidence("evidence_test", design, conformance.Report{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SaveSystemEvidence(ctx, evidence); err != nil {
		t.Fatal(err)
	}
	if evidence.DesignHash == "" || evidence.ContractHash == "" {
		t.Fatal("missing traceability hashes")
	}
	if err := db.SaveSystemEvidence(ctx, evidence); err == nil {
		t.Fatal("immutable evidence was overwritten")
	}
	design.Name = "Edited after experiment"
	design.Revision = "2"
	if err := db.SaveSystem(ctx, design); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := db.System(ctx, design.ID)
	if err != nil || saved.Name != design.Name {
		t.Fatalf("design did not survive reopen: %#v %v", saved, err)
	}
	snapshot, err := db.SystemEvidence(ctx, evidence.ID)
	if err != nil || snapshot.Design.Name == design.Name || snapshot.DesignHash != evidence.DesignHash {
		t.Fatalf("evidence snapshot changed: %#v %v", snapshot, err)
	}
	if records, err := db.SystemEvidenceList(ctx, design.ID); err != nil || len(records) != 1 {
		t.Fatalf("evidence history: %v %v", records, err)
	}
	if records, err := db.SystemEvidenceList(ctx, "unrelated"); err != nil || len(records) != 0 {
		t.Fatalf("evidence scope: %v %v", records, err)
	}
	if _, err := db.System(ctx, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing design: %v", err)
	}
	if old, err := db.Project(ctx, legacy.ID); err != nil || old.Name != legacy.Name {
		t.Fatalf("existing project damaged: %v %v", old, err)
	}
}

func TestNewSystemEvidenceDetachesInputs(t *testing.T) {
	design := system.ReceptionExample()
	report := conformance.Report{Limitations: []string{"Original limitation"}}
	evidence, err := NewSystemEvidence("snapshot", design, report)
	if err != nil {
		t.Fatal(err)
	}
	design.Contract.Claims[0].Kind = "changed"
	design.Components[1].Bindings[0].ResourceID = "changed"
	report.Limitations[0] = "changed"
	if evidence.Design.Contract.Claims[0].Kind == "changed" || evidence.Design.Components[1].Bindings[0].ResourceID == "changed" || evidence.Report.Limitations[0] == "changed" {
		t.Fatal("evidence aliases mutable inputs")
	}
}
