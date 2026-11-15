package configuration

import (
	"encoding/json"
	"fmt"
	"path"
	"path/filepath"
	"software-studio/internal/model"
	"software-studio/internal/system"
	"sort"
	"strings"
)

// Workspace describes composition, not filesystem ownership. Reused artifacts
// remain single identities and may be reached through several parents.
type WorkspaceLink struct {
	Symbol   string `json:"symbol"`
	Relation string `json:"relation"`
}
type WorkspaceNode struct {
	Symbol   string          `json:"symbol"`
	Children []WorkspaceLink `json:"children"`
}
type Workspace struct {
	Roots []string        `json:"roots"`
	Nodes []WorkspaceNode `json:"nodes"`
}

func Composition(c Compilation) Workspace {
	w := Workspace{Roots: []string{}, Nodes: []WorkspaceNode{}}
	parents := map[string]bool{}
	for _, a := range c.Artifacts {
		n := WorkspaceNode{Symbol: a.Symbol, Children: []WorkspaceLink{}}
		seen := map[string]bool{}
		add := func(kind, id, version, digest, relation string) {
			if kind == "component" {
				kind = "behavior"
			}
			for _, b := range c.Artifacts {
				if b.Kind == kind && b.ID == id && b.Version == version && (digest == "" || b.Digest == digest) && b.Symbol != a.Symbol && !seen[b.Symbol] {
					n.Children = append(n.Children, WorkspaceLink{b.Symbol, relation})
					seen[b.Symbol] = true
					parents[b.Symbol] = true
					break
				}
			}
		}
		behavior := func(r model.ComponentRef) { add("behavior", r.PackageID, r.Version, r.Digest, "subcomponent") }
		data := func(locks []model.DataContractLock) {
			for _, l := range locks {
				add("data", l.Contract.ID, l.Contract.Version, l.Digest, "aspect")
			}
		}
		machines := func(locks []model.StateMachineLock) {
			for _, l := range locks {
				add("state", l.Machine.ID, l.Machine.Version, l.Digest, "aspect")
			}
		}
		switch a.Kind {
		case "behavior":
			var v model.ComponentPackage
			_ = json.Unmarshal(a.Body, &v)
			data(v.DataContracts)
			if v.Behavior != nil {
				for _, node := range v.Behavior.Nodes {
					if node.Component != nil {
						behavior(*node.Component)
					}
				}
			}
		case "definition":
			var v system.ComponentDefinition
			_ = json.Unmarshal(a.Body, &v)
			for _, r := range v.Aspects {
				add(r.Kind, r.ID, r.Version, r.Digest, "aspect")
			}
		case "system":
			var v system.SystemDesign
			_ = json.Unmarshal(a.Body, &v)
			for _, r := range v.Components {
				kind := r.Definition.Kind
				if kind == "" {
					kind = "behavior"
				}
				add(kind, r.Definition.ID, r.Definition.Version, "", "subcomponent")
				for _, a := range r.Artifacts {
					add(a.Kind, a.ID, a.Version, "", "aspect")
				}
			}
		case "state":
			var v model.StateMachine
			_ = json.Unmarshal(a.Body, &v)
			data(v.DataContracts)
			for _, cmd := range v.Commands {
				behavior(cmd.Decision.Root)
			}
		case "messages":
			var v model.DeliveryModel
			_ = json.Unmarshal(a.Body, &v)
			machines(v.Machines)
		case "workflow":
			var v model.WorkflowDefinition
			_ = json.Unmarshal(a.Body, &v)
			data(v.DataContracts)
			machines(v.Machines)
			for _, d := range v.Decisions {
				behavior(d.Root)
			}
		}
		sort.Slice(n.Children, func(i, j int) bool { return n.Children[i].Symbol < n.Children[j].Symbol })
		w.Nodes = append(w.Nodes, n)
	}
	for _, a := range c.Artifacts {
		if !parents[a.Symbol] {
			w.Roots = append(w.Roots, a.Symbol)
		}
	}
	// Include disconnected cycles without recursively expanding them forever.
	reached := map[string]bool{}
	var visit func(string)
	visit = func(s string) {
		if reached[s] {
			return
		}
		reached[s] = true
		for _, n := range w.Nodes {
			if n.Symbol == s {
				for _, l := range n.Children {
					visit(l.Symbol)
				}
			}
		}
	}
	for _, s := range w.Roots {
		visit(s)
	}
	for _, a := range c.Artifacts {
		if !reached[a.Symbol] {
			w.Roots = append(w.Roots, a.Symbol)
			visit(a.Symbol)
		}
	}
	return w
}
func targetOf(a Artifact) Target { return Target{a.Kind, a.ID, a.Version, a.Digest} }
func selectedTargets(c Compilation, scope *TestScope) (map[Target]bool, error) {
	out := map[Target]bool{}
	if scope == nil {
		return out, nil
	}
	var selected *Artifact
	for i := range c.Artifacts {
		if targetOf(c.Artifacts[i]) == scope.Target {
			selected = &c.Artifacts[i]
			break
		}
	}
	if selected == nil {
		return nil, fmt.Errorf("selected component does not match the current source")
	}
	symbols := map[string]bool{}
	w := Composition(c)
	var visit func(string)
	visit = func(s string) {
		if symbols[s] {
			return
		}
		symbols[s] = true
		if scope.Descendants {
			for _, n := range w.Nodes {
				if n.Symbol == s {
					for _, child := range n.Children {
						visit(child.Symbol)
					}
				}
			}
		}
	}
	visit(selected.Symbol)
	for _, a := range c.Artifacts {
		if symbols[a.Symbol] {
			out[targetOf(a)] = true
		}
	}
	return out, nil
}

type OrganizedProject struct {
	EditResult
	Paths map[string]string `json:"paths"`
}

// Organize moves whole modules, preserving comments, identities and independent
// expectations. Shared modules have one location; imports are rewritten by span.
func Organize(r EditRequest) (OrganizedProject, error) {
	c, err := editorBase(r)
	if err != nil {
		return OrganizedProject{}, err
	}
	w := Composition(c)
	artifacts := map[string]Artifact{}
	byFile := map[string][]Artifact{}
	parents := map[string]map[string]bool{}
	modules := map[string]string{}
	for _, d := range c.Documents {
		modules[d.File] = d.Module
	}
	for _, a := range c.Artifacts {
		artifacts[a.Symbol] = a
		byFile[a.Origin.File] = append(byFile[a.Origin.File], a)
	}
	for _, n := range w.Nodes {
		for _, l := range n.Children {
			from, to := artifacts[n.Symbol].Origin.File, artifacts[l.Symbol].Origin.File
			if from != to {
				if parents[to] == nil {
					parents[to] = map[string]bool{}
				}
				parents[to][from] = true
			}
		}
	}
	dirs := map[string]string{}
	active := map[string]bool{}
	var folder func(string) string
	slug := func(f string) string {
		m := modules[f]
		parts := strings.Split(m, ".")
		s := parts[len(parts)-1]
		if len(s) > 24 {
			s = s[:24]
		}
		return s + "-" + Hash(m)[:6]
	}
	folder = func(f string) string {
		if d := dirs[f]; d != "" {
			return d
		}
		if active[f] {
			return "shared/" + slug(f)
		}
		active[f] = true
		d := "components/" + slug(f)
		if len(parents[f]) > 1 {
			d = "shared/" + slug(f)
		} else if len(parents[f]) == 1 {
			for p := range parents[f] {
				if p != r.Project.Entry {
					d = folder(p) + "/components/" + slug(f)
				}
			}
		}
		if len(d) > 190 {
			d = "shared/" + slug(f)
		}
		active[f] = false
		dirs[f] = d
		return d
	}
	paths := map[string]string{r.Project.Entry: r.Project.Entry}
	for _, f := range sortedKeys(r.Project.Files) {
		if f == r.Project.Entry {
			continue
		}
		if len(byFile[f]) > 0 {
			paths[f] = folder(f) + "/component.lce"
		}
	}
	for _, f := range sortedKeys(r.Project.Files) {
		if paths[f] != "" {
			continue
		}
		owners := map[string]bool{}
		count := 0
		for _, e := range c.Experiments {
			if e.Origin.File == f {
				count++
				for _, a := range c.Artifacts {
					if targetOf(a) == e.Target {
						owners[a.Origin.File] = true
					}
				}
			}
		}
		if count > 0 && len(owners) == 1 {
			for owner := range owners {
				d := folder(owner)
				paths[f] = d + "/testbench/" + slug(f) + ".lce"
			}
		} else {
			paths[f] = "modules/" + slug(f) + ".lce"
		}
	}
	edits := []textEdit{}
	for _, doc := range c.Documents {
		for _, imp := range doc.Imports {
			target := path.Join(path.Dir(doc.File), imp.Path)
			dest := paths[target]
			if dest == "" {
				return OrganizedProject{}, fmt.Errorf("unresolved import %s", target)
			}
			rel, e := filepath.Rel(path.Dir(paths[doc.File]), dest)
			if e != nil {
				return OrganizedProject{}, e
			}
			tokens, _ := tokenize(doc.File, r.Project.Files[doc.File][imp.Start:imp.End])
			for _, t := range tokens {
				if t.Kind == "string" {
					edits = append(edits, textEdit{doc.File, imp.Start + t.Start, imp.Start + t.End, literal(filepath.ToSlash(rel))})
					break
				}
			}
		}
	}
	p := applyEdits(r.Project, edits)
	files := map[string]string{}
	for old, text := range p.Files {
		next := paths[old]
		if _, exists := files[next]; exists {
			return OrganizedProject{}, fmt.Errorf("folder collision: %s", next)
		}
		files[next] = text
	}
	p.Files = files
	p.Entry = paths[p.Entry]
	next := Compile(p)
	if !sameArtifacts(c, next) {
		return OrganizedProject{}, fmt.Errorf("folder organization semantic guard rejected the change")
	}
	return OrganizedProject{EditResult{p, next}, paths}, nil
}

// NewTestbench never derives an oracle from the implementation. Its TODO
// assertion deliberately fails until the engineer supplies an expected result.
func NewTestbench(r EditRequest) (OrganizedProject, error) {
	c, err := editorBase(r)
	if err != nil {
		return OrganizedProject{}, err
	}
	var a *Artifact
	for i := range c.Artifacts {
		if c.Artifacts[i].Symbol == r.Name {
			a = &c.Artifacts[i]
			break
		}
	}
	if a == nil {
		return OrganizedProject{}, fmt.Errorf("select a component")
	}
	if a.Kind == "definition" {
		return OrganizedProject{}, fmt.Errorf("a definition groups aspects; create testbenches on its executable aspects")
	}
	name := strings.SplitN(a.Symbol, "#", 2)[1]
	if strings.Contains(name, ".") {
		return OrganizedProject{}, fmt.Errorf("specialized modules need explicit import parameters; add a testbench module with the same parameters")
	}
	file := path.Join(path.Dir(a.Origin.File), "testbench.lce")
	if file == r.Project.Entry || r.Project.Files[file] != "" {
		file = path.Join(path.Dir(a.Origin.File), name+"_testbench.lce")
	}
	if r.Project.Files[file] != "" {
		return OrganizedProject{}, fmt.Errorf("testbench already exists; open it in Files")
	}
	module := "tests.component_" + Hash(a.Symbol)[:16]
	for _, d := range c.Documents {
		if d.Module == module {
			return OrganizedProject{}, fmt.Errorf("testbench module already exists")
		}
	}
	text := "language \"1.0\";\nmodule " + literal(module) + " version \"1.0.0\";\nimport " + literal(path.Base(a.Origin.File)) + " as subject;\n\n-- TODO: supply this component's scenario inputs.\n-- TODO: replace the missing observation below with an independent expectation.\n-- This scaffold intentionally does not pass.\nexperiment " + name + "Test {\n  target = target(subject." + name + ");\n  scenario = {};\n  expect = { \"TODO.expected.observation\" = true; };\n}\n"
	p := applyEdits(r.Project, nil)
	p.Files[file] = text
	rel, _ := filepath.Rel(path.Dir(p.Entry), file)
	if a.Origin.File == p.Entry {
		entry := "test-suite.lce"
		if p.Files[entry] != "" {
			return OrganizedProject{}, fmt.Errorf("test-suite.lce already exists; use a dedicated component module")
		}
		p.Files[entry] = "language \"1.0\";\nmodule " + literal("tests.suite_"+Hash(a.Symbol)[:16]) + " version \"1.0.0\";\nimport " + literal(p.Entry) + " as project;\nimport " + literal(file) + " as tests;\n"
		p.Entry = entry
	} else {
		p.Files[p.Entry] += "\nimport " + literal(filepath.ToSlash(rel)) + " as test_" + Hash(module)[:12] + ";\n"
	}
	next := Compile(p)
	if !next.Valid {
		return OrganizedProject{}, fmt.Errorf("cannot add testbench: %s", next.Diagnostics[0].Message)
	}
	return OrganizedProject{EditResult{p, next}, map[string]string{"testbench": file}}, nil
}
