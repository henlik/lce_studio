package configuration

import (
	"context"
	"strings"
	"testing"
)

func TestHierarchyWorkspaceAndScopedEvidence(t *testing.T) {
	p := HierarchyProject()
	c := Compile(p)
	if !c.Valid {
		t.Fatalf("%+v", c.Diagnostics)
	}
	if len(c.Artifacts) != 4 || len(c.Experiments) != 4 || c.Workspace == nil || len(c.Workspace.Roots) != 1 {
		t.Fatalf("bad hierarchy %+v", c.Workspace)
	}
	r, e := Run(context.Background(), p)
	if e != nil || !r.Passed || len(r.Cases) != 4 {
		t.Fatalf("%+v %v", r, e)
	}
	var queue Artifact
	for _, a := range c.Artifacts {
		if strings.HasSuffix(a.Symbol, "#Queue") {
			queue = a
		}
	}
	scope := &TestScope{Target: targetOf(queue), Descendants: true}
	r, e = RunScoped(context.Background(), p, scope)
	if e != nil || !r.Passed || len(r.Cases) != 2 || r.Scope == nil {
		t.Fatalf("%+v %v", r, e)
	}
	scope.Descendants = false
	r, e = RunScoped(context.Background(), p, scope)
	if e != nil || len(r.Cases) != 1 {
		t.Fatalf("%+v %v", r, e)
	}
	pkg, e := Pack(context.Background(), "test.hierarchy", "1.0.0", p, false)
	if e != nil {
		t.Fatal(e)
	}
	pkg.Report = &r
	pkg.Digest = PackageDigest(pkg)
	if _, e = VerifyPackage(pkg); e == nil {
		t.Fatal("subset evidence accepted as complete package")
	}
}
func TestOrganizePreservesModulesImportsAndEvidence(t *testing.T) {
	for _, makeProject := range []func() (Project, error){func() (Project, error) { return HierarchyProject(), nil }, InspectionProject} {
		p, e := makeProject()
		if e != nil {
			t.Fatal(e)
		}
		c := Compile(p)
		r, e := Organize(EditRequest{Project: p, ExpectedHash: c.SourceHash})
		if e != nil {
			t.Fatal(e)
		}
		if !sameArtifacts(c, r.Compilation) {
			t.Fatal("changed semantics")
		}
		if len(r.Paths) != len(p.Files) {
			t.Fatal("lost files")
		}
		again, e := Organize(EditRequest{Project: r.Project, ExpectedHash: r.Compilation.SourceHash})
		if e != nil || Hash(r.Project) != Hash(again.Project) {
			t.Fatalf("not idempotent: %v", e)
		}
		for old, new := range r.Paths {
			if strings.Count(p.Files[old], "--") != strings.Count(r.Project.Files[new], "--") {
				t.Fatal("comments lost")
			}
		}
	}
	if _, e := Organize(EditRequest{Project: HierarchyProject(), ExpectedHash: "stale"}); e == nil {
		t.Fatal("stale edit accepted")
	}
}
func TestNewTestbenchIsExplicitlyUnfinished(t *testing.T) {
	p := HierarchyProject()
	delete(p.Files, "reception/components/queue/components/ticket/testbench.lce")
	p.Files[p.Entry] = strings.ReplaceAll(p.Files[p.Entry], "import \"reception/components/queue/components/ticket/testbench.lce\" as ticketTests;", "")
	c := Compile(p)
	var a Artifact
	for _, v := range c.Artifacts {
		if strings.HasSuffix(v.Symbol, "#Ticket") {
			a = v
		}
	}
	scope := &TestScope{Target: targetOf(a)}
	if _, e := RunScoped(context.Background(), p, scope); e == nil {
		t.Fatal("empty test selection passed")
	}
	edit, e := NewTestbench(EditRequest{Project: p, ExpectedHash: c.SourceHash, Name: a.Symbol})
	if e != nil {
		t.Fatal(e)
	}
	r, e := RunScoped(context.Background(), edit.Project, scope)
	if e != nil || r.Passed || len(r.Cases) != 1 {
		t.Fatalf("scaffold must not pass: %+v %v", r, e)
	}
}

func TestTestbenchForEntryDeclarationUsesASeparateSuiteRoot(t *testing.T) {
	p := HierarchyProject()
	p.Entry = "reception/components/queue/components/ticket/component.lce"
	c := Compile(p)
	if !c.Valid {
		t.Fatal(c.Diagnostics)
	}
	r, e := NewTestbench(EditRequest{Project: p, ExpectedHash: c.SourceHash, Name: c.Artifacts[0].Symbol})
	if e != nil {
		t.Fatal(e)
	}
	if r.Project.Entry == p.Entry || r.Project.Files[p.Entry] != p.Files[p.Entry] {
		t.Fatal("entry component must remain independent of its testbench")
	}
}
