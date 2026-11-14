package store

import (
	"context"
	"software-studio/internal/model"
	"software-studio/internal/system"
	"software-studio/internal/testutil"
	"strings"
	"testing"
)

func TestSystemCompatibilityReceptionConsultation(t *testing.T) {
	ctx := context.Background()
	s, err := Open(testutil.DatabaseURL(t))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	fixture, err := s.InstallConnectionExample(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.InstallConnectionExample(ctx); err != nil {
		t.Fatal("fixture not idempotent", err)
	}
	valid, err := s.ResolveSystem(ctx, fixture.Design)
	if err != nil || !valid.Valid || valid.Compatibility != "checked" || len(valid.Ports) != 4 {
		t.Fatalf("valid: %+v %v", valid, err)
	}
	invalid, err := s.ResolveSystem(ctx, fixture.Mismatch)
	if err != nil || invalid.Valid || invalid.Compatibility != "invalid" {
		t.Fatalf("mismatch: %+v %v", invalid, err)
	}
	if !strings.HasPrefix(invalid.Issues[0].Path, "connections[0]") {
		t.Fatal("mismatch not localized", invalid.Issues)
	}
	legacy := system.ReceptionExample()
	report, err := s.ResolveSystem(ctx, legacy)
	if err != nil || !report.Valid || report.Compatibility != "incomplete" {
		t.Fatalf("legacy: %+v %v", report, err)
	}
	incomplete := fixture.Design
	incomplete.Contract.Claims = nil
	incomplete.Connections[0].TargetPort = nil
	inspection, err := s.InspectSystemPorts(ctx, incomplete)
	if err != nil || len(inspection.Ports) != 4 || len(inspection.Issues) > 0 {
		t.Fatalf("ports blocked by incomplete connection/claims: %+v %v", inspection, err)
	}
}
func TestSystemClosureIncludesAttachedArtifactsAndInternalBindings(t *testing.T) {
	ctx := context.Background()
	s, err := Open(testutil.DatabaseURL(t))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	leaf := compositeLeaf("system.leaf")
	bundle := compositeFixture(t, "system.parent", leaf)
	if _, _, err = s.InstallComponentBundle(ctx, bundle); err != nil {
		t.Fatal(err)
	}
	digest, _ := model.ComponentDigest(leaf)
	definition := system.ComponentDefinition{SchemaVersion: 1, ID: "system.manifest", Version: "1.0.0", Name: "Manifest", Aspects: []system.AspectRef{{Slot: "one", Kind: "component", ID: leaf.ID, Version: leaf.Version, Digest: digest}, {Slot: "two", Kind: "component", ID: leaf.ID, Version: leaf.Version, Digest: digest}}, Links: []system.AspectLink{{ID: "copy", Source: system.AspectEndpoint{Slot: "one", Interface: "outputs", Port: "result"}, Target: system.AspectEndpoint{Slot: "two", Interface: "inputs", Port: "value"}}}}
	if _, err = s.InstallDefinition(ctx, definition); err != nil {
		t.Fatal(err)
	}
	d := system.ReceptionExample()
	d.Components[0].Definition = system.DefinitionRef{ID: leaf.ID, Version: leaf.Version}
	d.Components[1].Definition = system.DefinitionRef{Kind: "definition", ID: definition.ID, Version: definition.Version}
	d.Connections[0].SourcePort = &system.AspectEndpoint{Slot: "root", Interface: "outputs", Port: "result"}
	d.Connections[0].TargetPort = &system.AspectEndpoint{Slot: "two", Interface: "inputs", Port: "value"}
	r, err := s.ResolveSystem(ctx, d)
	if err != nil || r.Valid || !strings.Contains(r.Issues[0].Message, "producer") {
		t.Fatalf("internal binding conflict: %+v %v", r, err)
	}
	d.Connections[0].TargetPort.Slot = "one"
	r, err = s.ResolveSystem(ctx, d)
	if err != nil || !r.Valid {
		t.Fatalf("free manifest input: %+v %v", r, err)
	}
	// Attached artifacts also participate in full closure checking.
	d = system.ReceptionExample()
	d.Components[0].Artifacts = []system.ArtifactRef{{Kind: "component", ID: bundle.Root.PackageID, Version: bundle.Root.Version}}
	if _, err = s.db.ExecContext(ctx, `DELETE FROM component_packages WHERE id=$1`, leaf.ID); err != nil {
		t.Fatal(err)
	}
	r, err = s.ResolveSystem(ctx, d)
	if err != nil || r.Valid || !strings.HasPrefix(r.Issues[0].Path, "components[0].artifacts[0]") {
		t.Fatalf("broken attachment closure: %+v %v", r, err)
	}
}

func TestSystemRejectsConflictingDataAcrossInstances(t *testing.T) {
	ctx := context.Background()
	s, err := Open(testutil.DatabaseURL(t))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	packages := []model.ComponentPackage{}
	for i, name := range []string{"Original", "Changed"} {
		p := compositeLeaf("system.data-owner-" + name)
		p.SchemaVersion = 3
		lock := model.ExampleDataFixtures()[0].Contract
		if i == 1 {
			lock.Contract.Name = name
			lock.Digest, _ = model.DataContractDigest(lock.Contract)
		}
		p.DataContracts = []model.DataContractLock{lock}
		if _, _, err = s.InstallComponent(ctx, p); err != nil {
			t.Fatal(err)
		}
		packages = append(packages, p)
	}
	d := system.ReceptionExample()
	d.Connections = nil
	for i := range d.Components {
		d.Components[i].Definition = system.DefinitionRef{ID: packages[i].ID, Version: packages[i].Version}
	}
	r, err := s.ResolveSystem(ctx, d)
	if err != nil || r.Valid {
		t.Fatalf("conflicting data accepted: %+v %v", r, err)
	}
	if !strings.Contains(r.Issues[0].Message, "Conflicting dependency") {
		t.Fatal(r.Issues)
	}
}
