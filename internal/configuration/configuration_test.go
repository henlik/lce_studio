package configuration

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func minimal() Project {
	return Project{Language: Version, Entry: "main.lce", Files: map[string]string{"main.lce": `language "1.0";
module "test.demo" version "1.0.0";
parameter amount : integer = 5;
fixture shared = { n = integer(amount); };
data Types { records = [{ name = "Count"; fields = [{ name = "n"; type = "integer"; }]; }]; }
behavior Echo {
 behavior = {
  inputs = [{ name = "n"; type = "integer"; }]; outputs = [{ name = "n"; type = "integer"; }];
  nodes = []; bindings = { n = { input = "n"; }; };
 };
}
experiment checks { target = target(Echo); scenario = { inputs = shared; }; expect = { "outputs.n.integer" = 5; }; }
`}}
}
func TestFoundationAndParameters(t *testing.T) {
	p := minimal()
	a := Compile(p)
	if !a.Valid {
		t.Fatalf("%+v", a.Diagnostics)
	}
	b := Compile(p)
	if Hash(a) != Hash(b) {
		t.Fatal("nondeterministic")
	}
	r, e := Run(context.Background(), p)
	if e != nil || !r.Passed {
		t.Fatalf("%+v %v", r, e)
	}
	p.Parameters = map[string]any{"amount": json.Number("7")}
	b = Compile(p)
	if !b.Valid || a.Artifacts[0].ID == b.Artifacts[0].ID {
		t.Fatalf("specialization missing %+v", b.Diagnostics)
	}
	r, e = Run(context.Background(), p)
	if e != nil || r.Passed {
		t.Fatal("wrong oracle must fail")
	}
	p.Lock = &a.Lock
	if Compile(p).Valid {
		t.Fatal("stale lock accepted")
	}
}
func TestDiagnosticsAndClosedImports(t *testing.T) {
	p := minimal()
	p.Files["main.lce"] = strings.Replace(p.Files["main.lce"], "input = \"n\"", "input = \"missing\"", 1)
	r := Compile(p)
	if r.Valid || len(r.Diagnostics) == 0 || r.Diagnostics[0].Line < 2 {
		t.Fatalf("%+v", r.Diagnostics)
	}
	p = minimal()
	p.Files["main.lce"] += "\nfixture bad = absent;\nfixture also = missing;"
	r = Compile(p)
	if len(r.Diagnostics) < 2 {
		t.Fatalf("%+v", r.Diagnostics)
	}
	p = minimal()
	p.Files["main.lce"] += "\nimport \"../outside.lce\" as outside;"
	if Compile(p).Valid {
		t.Fatal("escape accepted")
	}
	p = minimal()
	p.Files["main.lce"] += "\nimport \"main.lce\" as cycle;"
	if Compile(p).Valid {
		t.Fatal("cycle accepted")
	}
}
func TestInspectionProject(t *testing.T) {
	p, e := InspectionProject()
	if e != nil {
		t.Fatal(e)
	}
	c := Compile(p)
	if !c.Valid {
		t.Fatalf("%+v", c.Diagnostics)
	}
	t.Logf("%d files, %d artifacts, %d experiments", len(p.Files), len(c.Artifacts), len(c.Experiments))
	r, e := Run(context.Background(), p)
	if e != nil {
		t.Fatal(e)
	}
	for _, c := range r.Cases {
		if !c.Passed {
			t.Errorf("%s: %s %+v", c.Name, c.Error, c.Assertions)
		}
	}
	r2, e := Run(context.Background(), p)
	if e != nil || Hash(r) != Hash(r2) {
		t.Fatal("fresh execution not reproducible", e)
	}
}
func TestEditorSemanticGuards(t *testing.T) {
	p := minimal()
	p.Files[p.Entry] += "\n-- keep this comment: Echo\n"
	c := Compile(p)
	r, e := Format(EditRequest{Project: p, ExpectedHash: c.SourceHash})
	if e != nil {
		t.Fatal(e)
	}
	r2, e := Format(EditRequest{Project: r.Project, ExpectedHash: r.Compilation.SourceHash})
	if e != nil || Hash(r.Project) != Hash(r2.Project) {
		t.Fatal("format not idempotent", e)
	}
	r, e = Rename(EditRequest{Project: p, ExpectedHash: c.SourceHash, File: p.Entry, Declaration: "Echo", Name: "Renamed"})
	if e != nil || !strings.Contains(r.Project.Files[p.Entry], "-- keep this comment: Echo") {
		t.Fatal("rename/comments", e)
	}
	if !sameArtifacts(c, r.Compilation) {
		t.Fatal("rename changed semantics")
	}
	_, e = Patch(EditRequest{Project: p, ExpectedHash: "stale", File: p.Entry, Declaration: "Echo", Path: "name", Value: "new"})
	if e == nil {
		t.Fatal("stale edit allowed")
	}
	r, e = Patch(EditRequest{Project: p, ExpectedHash: c.SourceHash, File: p.Entry, Declaration: "Echo", Path: "name", Value: "new"})
	if e != nil || !r.Compilation.Valid || !strings.Contains(r.Project.Files[p.Entry], "-- keep this comment") {
		t.Fatal("patch", e, r.Compilation.Diagnostics)
	}
	offset := strings.Index(p.Files[p.Entry], "target(Echo)") + 7
	span, e := Definition(p, p.Entry, offset)
	if e != nil || span.Start != strings.Index(p.Files[p.Entry], "Echo {") {
		t.Fatal("navigation", e, span)
	}
	cs := Complete(p, p.Entry, "Echo", "behavior")
	found := false
	for _, v := range cs {
		found = found || v.Label == "inputs" && v.Required
	}
	if !found {
		t.Fatal("missing typed completion")
	}
}
func TestModuleReuseAndTypedDiagnostics(t *testing.T) {
	p := minimal()
	p.Entry = "entry.lce"
	p.Files[p.Entry] = `language "1.0"; module "test.entry" version "1.0.0";
import "main.lce" as first;
import "main.lce" as same with { amount = 5; };
import "main.lce" as different with { amount = 7; };
`
	c := Compile(p)
	if !c.Valid || len(c.Artifacts) != 4 || len(c.Experiments) != 2 {
		t.Fatalf("equivalent defaults not shared %+v %d %d", c.Diagnostics, len(c.Artifacts), len(c.Experiments))
	}
	p = minimal()
	p.Files[p.Entry] = strings.Replace(p.Files[p.Entry], "type = \"integer\"", "type = null", 1)
	c = Compile(p)
	if c.Valid || c.Diagnostics[0].Line < 5 || c.Diagnostics[0].Column < 20 {
		t.Fatalf("typed source location %+v", c.Diagnostics)
	}
	p = minimal()
	p.Files[p.Entry] = strings.Replace(p.Files[p.Entry], "type = \"integer\"", "unknownField = \"integer\"", 1)
	c = Compile(p)
	if c.Valid || !strings.Contains(c.Diagnostics[0].Message, "unknownField") {
		t.Fatalf("unknown field %+v", c.Diagnostics)
	}
}
func FuzzConfigurationCompiler(f *testing.F) {
	f.Add(minimal().Files["main.lce"])
	f.Add("language \"1.0\"; module \"test.empty\" version \"1.0.0\";")
	f.Add("-- 🌍\nfixture")
	f.Fuzz(func(t *testing.T, text string) {
		if len(text) > 10000 {
			t.Skip()
		}
		p := Project{Language: Version, Entry: "main.lce", Files: map[string]string{"main.lce": text}}
		a, b := Compile(p), Compile(p)
		if Hash(a) != Hash(b) {
			t.Fatal("nondeterministic compilation")
		}
	})
}
func TestFixtureExpansionLimit(t *testing.T) {
	p := minimal()
	text := p.Files[p.Entry] + "\nfixture chunk = \"" + strings.Repeat("x", 20000) + "\";\n"
	last := "chunk"
	for i := 0; i < 12; i++ {
		next := fmt.Sprintf("repeat%d", i)
		text += "fixture " + next + " = [" + last + ", " + last + "];\n"
		last = next
	}
	p.Files[p.Entry] = text
	c := Compile(p)
	if c.Valid {
		t.Fatal("exponential fixture expansion accepted")
	}
	found := false
	for _, d := range c.Diagnostics {
		found = found || d.Code == "limit.expansion"
	}
	if !found {
		t.Fatalf("%+v", c.Diagnostics)
	}
}
