package configuration

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func emptyAuthoringProject() Project {
	return Project{Language: Version, Entry: "main.lce", Files: map[string]string{"main.lce": moduleHeader("test.empty")}}
}
func TestCreateComponentFromEmptyWorkspace(t *testing.T) {
	for _, typ := range []string{"boolean", "integer", "string"} {
		t.Run(typ, func(t *testing.T) {
			p := emptyAuthoringProject()
			before := Hash(p)
			r, err := NewComponent(EditRequest{Project: p, ExpectedHash: SourceHash(p), Name: "First", ValueType: typ})
			if err != nil {
				t.Fatal(err)
			}
			if Hash(p) != before || len(r.Project.Files) != 3 || len(r.Compilation.Artifacts) != 1 || len(r.Compilation.Experiments) != 1 {
				t.Fatal("creation must be atomic, imported, and independently tested")
			}
			if r.Paths["component"] != "components/First/component.lce" || r.Paths["testbench"] != "components/First/testbench.lce" {
				t.Fatal(r.Paths)
			}
			report, err := Run(context.Background(), r.Project)
			if err != nil || report.Passed || len(report.Cases) != 1 {
				t.Fatalf("unfinished scaffold passed: %+v %v", report, err)
			}
			if _, err := NewComponent(EditRequest{Project: r.Project, ExpectedHash: SourceHash(r.Project), Name: "First", ValueType: typ}); err == nil {
				t.Fatal("overwrote existing component")
			}
		})
	}
	for _, name := range []string{"../Bad", "a/b", "", "2Bad", strings.Repeat("X", 49)} {
		p := emptyAuthoringProject()
		if _, err := NewComponent(EditRequest{Project: p, ExpectedHash: SourceHash(p), Name: name, ValueType: "string"}); err == nil {
			t.Fatalf("accepted unsafe name %q", name)
		}
	}
	p := emptyAuthoringProject()
	for _, req := range []EditRequest{
		{Project: p, ExpectedHash: "stale", Name: "First", ValueType: "string"},
		{Project: p, ExpectedHash: SourceHash(p), Name: "First", ValueType: "record"},
	} {
		if _, err := NewComponent(req); err == nil {
			t.Fatal("accepted invalid creation")
		}
	}
	p.Files["components/First/testbench.lce"] = ""
	if _, err := NewComponent(EditRequest{Project: p, ExpectedHash: SourceHash(p), Name: "First", ValueType: "string"}); err == nil {
		t.Fatal("empty existing file overwritten")
	}
}

func TestExtractScalarPrimitivesPreservesIndependentParentExpectations(t *testing.T) {
	cases := []struct{ op, args, expected string }{
		{"identity", `value={literal=integer(9);};`, `9`},
		{"equal", `left={literal=string("A");};right={literal=string("B");};`, `false`},
		{"less_than", `left={literal=integer(2);};right={literal=integer(3);};`, `true`},
		{"less_or_equal", `left={literal=integer(3);};right={literal=integer(3);};`, `true`},
		{"and", `left={literal=boolean(true);};right={literal=boolean(false);};`, `false`},
		{"or", `left={literal=boolean(false);};right={literal=boolean(true);};`, `true`},
		{"not", `value={literal=boolean(false);};`, `true`},
		{"select", `condition={literal=boolean(true);};trueValue={literal=string("yes");};falseValue={literal=string("no");};`, `"yes"`},
		{"select", `condition={literal=boolean(false);};trueValue={literal=string("yes");};falseValue={literal=string("no");};`, `"no"`},
		{"add", `left={literal=integer(2);};right={literal=integer(3);};`, `5`},
		{"subtract", `left={literal=integer(2);};right={literal=integer(3);};`, `-1`},
		{"concat", `left={literal=string("A");};right={literal=string("B");};`, `"AB"`},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%s%d", tc.op, i), func(t *testing.T) {
			typ := "integer"
			if tc.expected == "true" || tc.expected == "false" {
				typ = "boolean"
			} else if strings.HasPrefix(tc.expected, `"`) {
				typ = "string"
			}
			p := emptyAuthoringProject()
			p.Files[p.Entry] += `
fixture arguments = {` + tc.args + `};
behavior Parent { behavior={inputs=[];outputs=[{name="result";type="` + typ + `";}];
 nodes=[{id="work"; -- preserve this explanation
 op="` + tc.op + `";inputs=arguments;position={x=75;y=100;};color="#123456";}];
 bindings={result={node="work";port="result";};};};}
experiment ParentTest {target=target(Parent);scenario={inputs={};};expect={"outputs.result.` + typ + `"=` + tc.expected + `;};}
`
			before := Hash(p)
			r, err := ExtractPrimitive(EditRequest{Project: p, ExpectedHash: SourceHash(p), File: p.Entry, Declaration: "Parent", Path: "behavior.nodes[0]", Name: "Child"})
			if err != nil {
				t.Fatal(err)
			}
			if Hash(p) != before || len(r.Compilation.Artifacts) != 2 || len(r.Compilation.Workspace.Roots) != 1 {
				t.Fatal("extraction mutated input or lost hierarchy")
			}
			for _, text := range []string{"-- preserve this explanation", "inputs=arguments", "position={x=75;y=100;}", `color="#123456"`} {
				if !strings.Contains(r.Project.Files[p.Entry], text) {
					t.Fatalf("lost source: %s", text)
				}
			}
			report, err := Run(context.Background(), r.Project)
			if err != nil || report.Passed || len(report.Cases) != 2 {
				t.Fatalf("parent plus unfinished child: %+v %v", report, err)
			}
			parentPassed := false
			for _, test := range report.Cases {
				if strings.HasSuffix(test.Name, ".ParentTest") {
					parentPassed = test.Passed
				}
			}
			if !parentPassed {
				t.Fatalf("parent expectation changed: %+v", report)
			}
			standalone := r.Project
			standalone.Entry = r.Paths["component"]
			standalone.Files = map[string]string{standalone.Entry: r.Project.Files[standalone.Entry]}
			if c := Compile(standalone); !c.Valid || len(c.Artifacts) != 1 {
				t.Fatalf("child depends on parent: %+v", c.Diagnostics)
			}
		})
	}
}

func TestExtractFromNestedGraphWithUpstreamComponent(t *testing.T) {
	p := HierarchyProject()
	f := "reception/component.lce"
	p.Files[f] = strings.Replace(p.Files[f], `{id="identification"`, `{id="increment";op="add";inputs={left={node="queue";port="ticket";};right={literal=integer(1);};};},
 {id="identification"`, 1)
	p.Files[f] = strings.Replace(p.Files[f], `bindings = {ticket={node="queue";port="ticket"`, `bindings = {ticket={node="increment";port="result"`, 1)
	p.Files["reception/testbench.lce"] = strings.Replace(p.Files["reception/testbench.lce"], `"outputs.ticket.integer"=17`, `"outputs.ticket.integer"=18`, 1)
	if c := Compile(p); !c.Valid {
		t.Fatal(c.Diagnostics)
	}
	r, err := ExtractPrimitive(EditRequest{Project: p, ExpectedHash: SourceHash(p), File: f, Declaration: "Reception", Path: "behavior.nodes[1]", Name: "NextTicket"})
	if err != nil {
		t.Fatal(err)
	}
	if r.Paths["component"] != "reception/components/NextTicket/component.lce" {
		t.Fatal(r.Paths)
	}
	report, err := Run(context.Background(), r.Project)
	if err != nil || len(report.Cases) != 5 {
		t.Fatal(err, report)
	}
	passed := 0
	for _, c := range report.Cases {
		if c.Passed {
			passed++
		}
	}
	if passed != 4 {
		t.Fatalf("all four existing independent tests must pass: %+v", report)
	}
	for _, test := range Compile(p).Experiments {
		if r.Project.Files[test.Origin.File] != p.Files[test.Origin.File] {
			t.Fatal("rewrote an existing testbench")
		}
	}
	for _, req := range []EditRequest{
		{Project: p, ExpectedHash: "stale", File: f, Declaration: "Reception", Path: "behavior.nodes[1]", Name: "Child"},
		{Project: p, ExpectedHash: SourceHash(p), File: f, Declaration: "Reception", Path: "behavior.nodes[0]", Name: "Child"},
		{Project: p, ExpectedHash: SourceHash(p), File: f, Declaration: "Reception", Path: "behavior.nodes[900]", Name: "Child"},
	} {
		if _, err := ExtractPrimitive(req); err == nil {
			t.Fatal("accepted stale source, component call, or invalid node")
		}
	}
}
