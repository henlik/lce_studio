package store

import (
	"context"
	"encoding/json"
	"errors"
	"software-studio/internal/model"
	"software-studio/internal/system"
	"software-studio/internal/testutil"
	"strings"
	"testing"
)

func TestDefinitionCompatibilityChecksLinksAndCatalogClosure(t *testing.T) {
	ctx := context.Background()
	s, err := Open(testutil.DatabaseURL(t))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	child := compositeLeaf("test.child")
	bundle := compositeFixture(t, "test.parent", child)
	if _, _, err = s.InstallComponentBundle(ctx, bundle); err != nil {
		t.Fatal(err)
	}
	root := bundle.Root
	d := system.ComponentDefinition{SchemaVersion: 1, ID: "test.connected", Version: "1.0.0", Name: "Connected", Aspects: []system.AspectRef{
		{Slot: "producer", Kind: "component", ID: root.PackageID, Version: root.Version, Digest: root.Digest},
		{Slot: "consumer", Kind: "component", ID: child.ID, Version: child.Version, Digest: compositeLock(t, child).Digest},
	}}
	report, err := s.CheckDefinition(ctx, d)
	if err != nil || !report.Valid {
		t.Fatalf("initial %+v %v", report, err)
	}
	var source, target system.AspectEndpoint
	for _, p := range report.Ports {
		if p.Slot == "producer" && p.Direction == "output" {
			source = p.AspectEndpoint
		}
		if p.Slot == "consumer" && p.Direction == "input" {
			target = p.AspectEndpoint
		}
	}
	d.Links = []system.AspectLink{{ID: "pass", Source: source, Target: target}}
	report, err = s.CheckDefinition(ctx, d)
	if err != nil || !report.Valid || len(report.Dependencies) < 5 {
		t.Fatalf("valid link/closure: %+v %v", report, err)
	}
	if _, err = s.InstallDefinition(ctx, d); err != nil {
		t.Fatal(err)
	}
	// Changing a declared endpoint is rejected at install, not merely in the UI.
	bad := d
	bad.Links = append([]system.AspectLink{}, d.Links...)
	bad.Links[0].Target.Port = "missing"
	if _, err = s.InstallDefinition(ctx, bad); err == nil {
		t.Fatal("bad endpoint installed")
	} else {
		var invalid *DefinitionValidationError
		if !errors.As(err, &invalid) {
			t.Fatal(err)
		}
	}
	// The direct parent is unchanged; a transitive dependency disappears.
	if _, err = s.db.ExecContext(ctx, `DELETE FROM component_packages WHERE id=$1`, child.ID); err != nil {
		t.Fatal(err)
	}
	d.Aspects = d.Aspects[:1]
	d.Links = nil
	report, err = s.CheckDefinition(ctx, d)
	if err != nil || report.Valid || !strings.Contains(report.Issues[0].Message, "closure") {
		t.Fatalf("missing descendant accepted: %+v %v", report, err)
	}
	design := system.ReceptionExample()
	design.Components[0].Definition = system.DefinitionRef{Kind: "definition", ID: "test.connected", Version: "1.0.0"}
	resolution, err := s.ResolveSystem(ctx, design)
	if err != nil || resolution.Valid {
		t.Fatalf("installed broken closure accepted: %+v %v", resolution, err)
	}
}
func TestDefinitionRejectsConflictingEmbeddedDependency(t *testing.T) {
	ctx := context.Background()
	s, err := Open(testutil.DatabaseURL(t))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	root := compositeLeaf("test.structured-one")
	root.SchemaVersion = 3
	root.DataContracts = []model.DataContractLock{model.ExampleDataFixtures()[0].Contract}
	encoded, err := json.Marshal(root)
	if err != nil {
		t.Fatal(err)
	}
	var second model.ComponentPackage
	if err = json.Unmarshal(encoded, &second); err != nil {
		t.Fatal(err)
	}
	second.ID = "test.structured-two"
	second.DataContracts[0].Contract.Name += " altered"
	second.DataContracts[0].Digest, _ = model.DataContractDigest(second.DataContracts[0].Contract)
	if issues := model.ValidateComponentPackage(second); len(issues) > 0 {
		t.Fatal(issues)
	}
	one, _, err := s.InstallComponent(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	two, _, err := s.InstallComponent(ctx, second)
	if err != nil {
		t.Fatal(err)
	}
	d := system.ComponentDefinition{SchemaVersion: 1, ID: "test.conflicting", Version: "1.0.0", Name: "Conflicting", Aspects: []system.AspectRef{{Slot: "one", Kind: "component", ID: one.Package.ID, Version: one.Package.Version, Digest: one.Digest}, {Slot: "two", Kind: "component", ID: two.Package.ID, Version: two.Package.Version, Digest: two.Digest}}}
	report, err := s.CheckDefinition(ctx, d)
	if err != nil || report.Valid {
		t.Fatalf("conflicting closure accepted: %+v %v", report, err)
	}
	found := false
	for _, i := range report.Issues {
		if strings.Contains(i.Message, "Conflicting content") {
			found = true
		}
	}
	if !found {
		t.Fatalf("wrong failure %+v", report)
	}
}

func TestDefinitionDoesNotIgnoreInvalidTimingDependency(t *testing.T) {
	ctx := context.Background()
	s, err := Open(testutil.DatabaseURL(t))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	timing := model.ExampleComponentPackages()[0]
	record, _, err := s.InstallComponent(ctx, timing)
	if err != nil {
		t.Fatal(err)
	}
	d := system.ComponentDefinition{SchemaVersion: 1, ID: "test.timing", Version: "1.0.0", Name: "Timing", Aspects: []system.AspectRef{{Slot: "profile", Kind: "component", ID: timing.ID, Version: timing.Version, Digest: record.Digest}}}
	if r, err := s.CheckDefinition(ctx, d); err != nil || !r.Valid || len(r.Ports) != 0 {
		t.Fatalf("timing root: %+v %v", r, err)
	}
	parent := compositeLeaf("test.bad-parent")
	parent.Behavior.Nodes[0].Op = ""
	parent.Behavior.Nodes[0].Component = &model.ComponentRef{PackageID: timing.ID, Version: timing.Version, Digest: record.Digest}
	parentRecord, _, err := s.InstallComponent(ctx, parent)
	if err != nil {
		t.Fatal(err)
	}
	d.Aspects[0] = system.AspectRef{Slot: "parent", Kind: "component", ID: parent.ID, Version: parent.Version, Digest: parentRecord.Digest}
	if r, err := s.CheckDefinition(ctx, d); err != nil || r.Valid {
		t.Fatalf("timing descendant silently ignored: %+v %v", r, err)
	}
}
