package configuration

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"software-studio/internal/assembly"
	"software-studio/internal/model"
	"software-studio/internal/source"
	"software-studio/internal/system"
	"software-studio/internal/systemrun"
	"strings"
)

type rawExpr string

func literal(v any) string { b, _ := json.Marshal(v); return string(b) }
func render(v any, level int) string {
	pad := strings.Repeat("  ", level)
	switch x := v.(type) {
	case rawExpr:
		return string(x)
	case map[string]any:
		if len(x) == 0 {
			return "{}"
		}
		lines := []string{"{"}
		for _, k := range sortedKeys(x) {
			key := k
			if !identifier.MatchString(k) {
				key = literal(k)
			}
			lines = append(lines, pad+"  "+key+" = "+render(x[k], level+1)+";")
		}
		return strings.Join(lines, "\n") + "\n" + pad + "}"
	case []any:
		if len(x) == 0 {
			return "[]"
		}
		parts := []string{}
		for _, v := range x {
			parts = append(parts, pad+"  "+render(v, level+1))
		}
		return "[\n" + strings.Join(parts, ",\n") + "\n" + pad + "]"
	default:
		return literal(v)
	}
}

// Export converts validated canonical models into editable declarations. Every
// embedded dependency is emitted once and referenced by its computed exact lock.
type Exporter struct {
	Artifacts   []Artifact
	Experiments []Experiment
	seen        map[string]string
}

func NewExporter() *Exporter { return &Exporter{seen: map[string]string{}} }
func (x *Exporter) Add(kind string, v any) (Artifact, error) {
	body := cloneJSON(v).(map[string]any)
	a, issues, err := validateArtifact(kind, body)
	if err != nil {
		return a, err
	}
	if len(issues) > 0 {
		return a, fmt.Errorf("%s: %s", issues[0].Path, issues[0].Message)
	}
	if a.Version == "" {
		a.Version = "1.0.0"
	}
	key := a.Kind + "/" + a.ID + "/" + a.Version
	if digest, ok := x.seen[key]; ok {
		if digest != a.Digest {
			return a, fmt.Errorf("conflicting exported artifact %s", key)
		}
		return a, nil
	}
	x.seen[key] = a.Digest
	var walk func(any) error
	walk = func(v any) error {
		switch m := v.(type) {
		case map[string]any:
			if _, ok := m["digest"]; ok {
				for key, kind := range map[string]string{"package": "behavior", "contract": "data", "machine": "state", "model": "messages", "workflow": "workflow"} {
					if child, ok := m[key].(map[string]any); ok {
						if _, e := x.Add(kind, child); e != nil {
							return e
						}
						return nil
					}
				}
			}
			for _, k := range sortedKeys(m) {
				if e := walk(m[k]); e != nil {
					return e
				}
			}
		case []any:
			for _, v := range m {
				if e := walk(v); e != nil {
					return e
				}
			}
		}
		return nil
	}
	if err = walk(body); err != nil {
		return a, err
	}
	x.Artifacts = append(x.Artifacts, a)
	return a, nil
}
func (x *Exporter) Case(name string, a Artifact, scenario any, expect map[string]any) {
	b, _ := json.Marshal(scenario)
	x.Experiments = append(x.Experiments, Experiment{Name: name, Target: Target{a.Kind, a.ID, a.Version, a.Digest}, Scenario: b, Expect: cloneJSON(expect).(map[string]any)})
}
func (x *Exporter) Project() (Project, error) {
	names := map[string]string{}
	usedNames := map[string]bool{}
	for _, a := range x.Artifacts {
		name := a.Kind + "_" + regexp.MustCompile(`[^A-Za-z0-9_]`).ReplaceAllString(a.ID, "_")
		if len(name) > 50 {
			name = name[:50]
		}
		if usedNames[name] {
			name += "_" + Hash(a.ID + "@" + a.Version)[:8]
		}
		usedNames[name] = true
		names[a.Kind+"/"+a.ID+"/"+a.Version] = name
	}
	find := func(kind string, v any, h any) string {
		m, ok := v.(map[string]any)
		if !ok {
			return ""
		}
		id, _ := m["id"].(string)
		version, _ := m["version"].(string)
		key := kind + "/" + id + "/" + version
		if h != nil && x.seen[key] != h {
			return ""
		}
		return names[key]
	}
	var transform func(any) any
	transform = func(v any) any {
		switch m := v.(type) {
		case map[string]any:
			if h, ok := m["digest"]; ok {
				for key, kind := range map[string]string{"package": "behavior", "contract": "data", "machine": "state", "model": "messages", "workflow": "workflow"} {
					if name := find(kind, m[key], h); name != "" {
						return rawExpr("lock(" + name + ")")
					}
				}
			}
			if _, ok := m["packages"]; ok {
				if r, ok := m["root"].(map[string]any); ok {
					if name := find("behavior", map[string]any{"id": r["packageId"], "version": r["version"]}, r["digest"]); name != "" {
						return rawExpr("bundle(" + name + ")")
					}
				}
			}
			if len(m) == 3 {
				for field, kind := range map[string]string{"packageId": "behavior", "machineId": "state"} {
					if name := find(kind, map[string]any{"id": m[field], "version": m["version"]}, m["digest"]); name != "" {
						return rawExpr("ref(" + name + ")")
					}
				}
			}
			if len(m) == 4 {
				if name := find("data", map[string]any{"id": m["contractId"], "version": m["version"]}, m["digest"]); name != "" {
					return rawExpr("record(" + name + ", " + literal(m["record"]) + ")")
				}
			}
			if len(m) == 5 && m["slot"] != nil {
				kind, _ := m["kind"].(string)
				if kind == "component" {
					kind = "behavior"
				}
				if name := find(kind, m, m["digest"]); name != "" {
					return rawExpr("aspect(" + literal(m["slot"]) + ", " + name + ")")
				}
			}
			if len(m) == 3 && m["kind"] != nil {
				kind, _ := m["kind"].(string)
				if kind == "component" {
					kind = "behavior"
				}
				if name := find(kind, m, nil); name != "" {
					return rawExpr("use(" + name + ")")
				}
			}
			if len(m) == 2 {
				kind, _ := m["type"].(string)
				if (kind == "boolean" || kind == "integer" || kind == "string") && m[kind] != nil {
					return rawExpr(kind + "(" + literal(m[kind]) + ")")
				}
			}
			out := map[string]any{}
			for k, v := range m {
				out[k] = transform(v)
			}
			return out
		case []any:
			out := []any{}
			for _, v := range m {
				out = append(out, transform(v))
			}
			return out
		default:
			return v
		}
	}
	// Split declarations across bounded modules, all imported by the entry. Dependencies
	// are shared through one artifacts module when it fits; large exports use one file
	// per artifact with explicit direct imports (no ambient global namespace).
	p := Project{Language: Version, Entry: "main.lce", Files: map[string]string{}}
	for _, a := range x.Artifacts {
		name := names[a.Kind+"/"+a.ID+"/"+a.Version]
		body := render(transform(decodeAny(a.Body)), 0)
		imports := []string{}
		for _, other := range x.Artifacts {
			n := names[other.Kind+"/"+other.ID+"/"+other.Version]
			if n == name {
				continue
			} // Constructors use exact symbol tokens, never strings.
			tokens, _ := tokenize("", body)
			used := false
			for _, t := range tokens {
				if t.Kind == "name" && t.Text == n {
					used = true
				}
			}
			if used {
				body = qualify(body, n, "m_"+n+"."+n)
				imports = append(imports, "import "+literal(n+".lce")+" as m_"+n+";")
			}
		}
		moduleVersion := a.Version
		if !versionID.MatchString(moduleVersion) {
			moduleVersion = "1.0.0"
		}
		p.Files[name+".lce"] = "language \"1.0\";\nmodule " + literal("export."+name) + " version " + literal(moduleVersion) + ";\n" + strings.Join(imports, "\n") + "\n\n" + a.Kind + " " + name + " " + body + "\n"
	}
	main := "language \"1.0\";\nmodule \"export.project\" version \"1.0.0\";\n"
	for _, a := range x.Artifacts {
		n := names[a.Kind+"/"+a.ID+"/"+a.Version]
		main += "import " + literal(n+".lce") + " as m_" + n + ";\n"
	}

	for i, e := range x.Experiments {
		name := fmt.Sprintf("case%02d", i+1)
		n := names[e.Target.Kind+"/"+e.Target.ID+"/"+e.Target.Version]
		body := render(map[string]any{"target": rawExpr("target(" + n + ")"), "scenario": transform(decodeAny(e.Scenario)), "expect": transform(e.Expect)}, 0)
		imports := []string{}
		for _, a := range x.Artifacts {
			symbol := names[a.Kind+"/"+a.ID+"/"+a.Version]
			ts, _ := tokenize("", body)
			used := false
			for _, t := range ts {
				if t.Kind == "name" && t.Text == symbol {
					used = true
				}
			}
			if used {
				body = qualify(body, symbol, "m_"+symbol+"."+symbol)
				imports = append(imports, "import "+literal(symbol+".lce")+" as m_"+symbol+";")
			}
		}
		filename := "test_" + name + ".lce"
		p.Files[filename] = "language \"1.0\";\nmodule " + literal("tests."+name) + " version \"1.0.0\";\n" + strings.Join(imports, "\n") + "\n\n-- " + strings.ReplaceAll(e.Name, "\n", " ") + "\nexperiment " + name + " " + body + "\n"
		main += "import " + literal(filename) + " as test_" + name + ";\n"
	}

	p.Files[p.Entry] = main
	c := Compile(p)
	if !c.Valid {
		return p, fmt.Errorf("export compilation: %+v", c.Diagnostics)
	}
	p.Lock = &c.Lock
	return p, nil
}
func qualify(text, name, replacement string) string {
	ts, _ := tokenize("", text)
	for i := len(ts) - 1; i >= 0; i-- {
		t := ts[i]
		if t.Kind == "name" && t.Text == name {
			text = text[:t.Start] + replacement + text[t.End:]
		}
	}
	return text
}
func InspectionProject() (Project, error) {
	x := NewExporter()
	for _, c := range model.StudioInspectionPack().Cases {
		var a Artifact
		var err error
		var scenario any
		expected := map[string]any{strings.TrimPrefix(c.VerdictPath, "result."): c.Expected}
		switch e := c.Body.(type) {
		case model.WorkflowExample:
			a, err = x.Add("workflow", e.Workflow.Workflow)
			scenario = e.Scenario
		case model.DataExample:
			a, err = x.Add("data", e.Contract.Contract)
			scenario = e.Fixture
		case model.StateExample:
			a, err = x.Add("state", e.Machine.Machine)
			scenario = e.Scenario
		case model.DeliveryExample:
			a, err = x.Add("messages", e.Model.Model)
			scenario = e.Scenario
		case map[string]any:
			if b, ok := e["bundle"].(model.ComponentBundle); ok {
				for _, l := range b.Packages {
					v, e := x.Add("behavior", l.Package)
					if e != nil {
						return Project{}, e
					}
					if l.Package.ID == b.Root.PackageID {
						a = v
					}
				}
				scenario = map[string]any{"tests": true}
			} else if p, ok := e["project"].(model.Project); ok {
				a, err = x.Add("timing", p)
				scenario = map[string]any{}
			}
		}
		if err != nil {
			return Project{}, err
		}
		if a.Kind != "" {
			x.Case(c.Name, a, scenario, expected)
		}
	}
	m := system.M1Example()
	if _, err := x.Add("workflow", m.Workflow.Workflow); err != nil {
		return Project{}, err
	}
	if _, err := x.Add("messages", m.Model.Model); err != nil {
		return Project{}, err
	}
	for _, d := range m.Definitions {
		if _, err := x.Add("definition", d); err != nil {
			return Project{}, err
		}
	}
	a, err := x.Add("system", m.Design)
	if err != nil {
		return Project{}, err
	}
	p, err := assembly.Prepare(context.Background(), Library{x.Artifacts}, m.Design)
	if err != nil {
		return Project{}, err
	}
	scenarios, err := systemrun.M1Scenarios(p.Plan)
	if err != nil {
		return Project{}, err
	}
	for _, s := range scenarios {
		x.Case(s.Name, a, s.Scenario, map[string]any{"scenario.passed": true})
	}
	return x.Project()
}
func MigrateLegacy(text string) (Project, error) {
	r := source.Compile(text)
	if !r.Valid || r.Component == nil {
		return Project{}, fmt.Errorf("a valid legacy component source is required")
	}
	x := NewExporter()
	p := r.Component
	for _, pkg := range p.Packages {
		if _, e := x.Add("behavior", pkg); e != nil {
			return Project{}, e
		}
	}
	machine, err := x.Add("state", p.Machine.Machine)
	if err != nil {
		return Project{}, err
	}
	if r.Testbench != nil {
		for _, test := range r.Testbench.Cases {
			expect := map[string]any{"passed": true, "verified": true}
			for i, step := range test.Steps {
				prefix := fmt.Sprintf("steps[%d].", i)
				expect[prefix+"status"] = step.Status
				expect[prefix+"replyLost"] = step.ReplyLost
				if step.Outputs != nil {
					expect[prefix+"receipt.outputs"] = step.Outputs
				}
			}
			if len(test.Effects) > 0 {
				matches := []any{}
				for _, effect := range test.Effects {
					matches = append(matches, map[string]any{"value": map[string]any{"commandId": effect.CommandID, "effect": effect.Event, "payload": effect.Payload}, "count": 1})
				}
				expect["instances[0].effects"] = map[string]any{"$matches": matches}
			}
			x.Case(test.Scenario.Name, machine, test.Scenario, expect)
		}
	}

	if _, e := x.Add("definition", p.Definition); e != nil {
		return Project{}, e
	}
	if _, e := x.Add("system", r.Design); e != nil {
		return Project{}, e
	}
	return x.Project()
}
