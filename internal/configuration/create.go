package configuration

import (
	"encoding/json"
	"fmt"
	"path"
	"path/filepath"
	"software-studio/internal/model"
	"strconv"
	"strings"
)

func componentDestination(p Project, dir, name string) (string, string, error) {
	if !identifier.MatchString(name) || len(name) > 48 {
		return "", "", fmt.Errorf("use a component symbol of 1–48 letters, digits or underscores, starting with a letter")
	}
	file := path.Join(dir, "components", name, "component.lce")
	// A folder belongs to one component; never overwrite even an empty file.
	for f := range p.Files {
		if strings.HasPrefix(f, path.Dir(file)+"/") {
			return "", "", fmt.Errorf("component folder already exists: %s", path.Dir(file))
		}
	}
	return file, "local.component_" + Hash(file)[:12], nil
}

func moduleHeader(module string) string {
	return "language \"1.0\";\nmodule " + literal(module) + " version \"1.0.0\";\n"
}

func relativeImport(from, to, alias string) string {
	rel, _ := filepath.Rel(path.Dir(from), to)
	return "\nimport " + literal(filepath.ToSlash(rel)) + " as " + alias + ";\n"
}

// NewComponent is an explicit user action, not a seed for new projects. The
// small identity scaffold compiles; its independently authored test is TODO.
func NewComponent(r EditRequest) (OrganizedProject, error) {
	if _, err := editorBase(r); err != nil {
		return OrganizedProject{}, err
	}
	if r.ValueType != "boolean" && r.ValueType != "integer" && r.ValueType != "string" {
		return OrganizedProject{}, fmt.Errorf("choose a boolean, integer or string starter interface")
	}
	file, module, err := componentDestination(r.Project, "", r.Name)
	if err != nil {
		return OrganizedProject{}, err
	}
	p := applyEdits(r.Project, nil)
	p.Files[file] = moduleHeader(module) + `
-- Starter: replace this identity operation with your component's behavior.
behavior ` + r.Name + ` {
  behavior = {
    inputs = [{ name = "value"; type = ` + literal(r.ValueType) + `; }];
    outputs = [{ name = "result"; type = ` + literal(r.ValueType) + `; }];
    nodes = [{ id = "evaluate"; op = "identity"; inputs = { value = { input = "value"; }; }; }];
    bindings = { result = { node = "evaluate"; port = "result"; }; };
  };
}
`
	p.Files[p.Entry] += relativeImport(p.Entry, file, "component_"+Hash(file)[:12])
	return componentWithTestbench(p, file, r.Name)
}

func componentWithTestbench(p Project, file, name string) (OrganizedProject, error) {
	c := Compile(p)
	if !c.Valid {
		return OrganizedProject{}, fmt.Errorf("cannot create component: %s", c.Diagnostics[0].Message)
	}
	r, err := NewTestbench(EditRequest{Project: p, ExpectedHash: c.SourceHash, Name: file + "#" + name})
	if err != nil {
		return OrganizedProject{}, err
	}
	r.Paths["component"] = file
	return r, nil
}

// ExtractPrimitive replaces one inline scalar primitive with a call that has
// the exact same inputs and result port. Parent bindings, comments, fixtures,
// expectations and node identity remain authored source. An extra component
// invocation changes trace paths and consumes the usual depth/invocation budget.
func ExtractPrimitive(r EditRequest) (OrganizedProject, error) {
	c, err := editorBase(r)
	if err != nil {
		return OrganizedProject{}, err
	}
	doc, d, ok := findDecl(c, r.File, r.Declaration)
	if !ok || d.Kind != "behavior" {
		return OrganizedProject{}, fmt.Errorf("select an executable behavior component")
	}
	for _, declaration := range doc.Declarations {
		if declaration.Kind == "parameter" {
			return OrganizedProject{}, fmt.Errorf("extract from an unparameterized module; parameterized modules require an explicit reusable interface")
		}
	}
	parts := pathParts(r.Path)
	if len(parts) != 3 || parts[0] != "behavior" || parts[1] != "nodes" {
		return OrganizedProject{}, fmt.Errorf("select a primitive node")
	}
	index, err := strconv.Atoi(parts[2])
	if err != nil || index < 0 {
		return OrganizedProject{}, fmt.Errorf("invalid node index")
	}
	expr, ok := exactExpr(d.Value, parts)
	if !ok || expr.Kind != "object" {
		return OrganizedProject{}, fmt.Errorf("this node is supplied by a fixture; open its source before extracting it")
	}
	var parent Artifact
	for _, a := range c.Artifacts {
		if a.Symbol == r.File+"#"+r.Declaration {
			parent = a
		}
	}
	var pkg model.ComponentPackage
	_ = json.Unmarshal(parent.Body, &pkg)
	if pkg.Behavior == nil || index >= len(pkg.Behavior.Nodes) {
		return OrganizedProject{}, fmt.Errorf("node does not belong to the selected component")
	}
	node := pkg.Behavior.Nodes[index]
	scalar := false
	for _, spec := range model.PrimitiveSpecs() {
		scalar = scalar || node.Op == spec.Op
	}
	if node.Component != nil || !scalar {
		return OrganizedProject{}, fmt.Errorf("extract supports scalar primitives; existing subcomponents already have their own source")
	}
	inputs, output, err := primitiveInterface(c, parent, node.ID)
	if err != nil {
		return OrganizedProject{}, err
	}
	if output.Type != "boolean" && output.Type != "integer" && output.Type != "string" {
		return OrganizedProject{}, fmt.Errorf("structured values need an explicitly authored data-contract interface")
	}
	ports := []model.BehaviorPort{}
	inner := node
	inner.ID = "evaluate"
	inner.Position = model.Position{}
	inner.Inputs = map[string]model.BehaviorBinding{}
	for _, name := range sortedKeys(inputs) {
		typ := inputs[name]
		if typ.Type != "boolean" && typ.Type != "integer" && typ.Type != "string" {
			return OrganizedProject{}, fmt.Errorf("structured values need an explicitly authored data-contract interface")
		}
		ports = append(ports, model.BehaviorPort{Name: name, Type: typ.Type})
		inner.Inputs[name] = model.BehaviorBinding{Input: name}
	}
	file, module, err := componentDestination(r.Project, path.Dir(r.File), r.Name)
	if err != nil {
		return OrganizedProject{}, err
	}
	alias := "component_" + Hash(file)[:12]
	edits := []textEdit{}
	for _, f := range expr.Fields {
		if f.Name == "op" {
			if f.Value.Kind != "literal" {
				return OrganizedProject{}, fmt.Errorf("operation is supplied by a fixture; author the subcomponent explicitly")
			}
			edits = append(edits, textEdit{r.File, f.Span.Start, f.Span.End, "component"}, textEdit{r.File, f.Value.Start, f.Value.End, "ref(" + alias + "." + r.Name + ")"})
		}
	}
	if len(edits) == 0 {
		return OrganizedProject{}, fmt.Errorf("select an inline primitive operation")
	}
	p := applyEdits(r.Project, edits)
	p.Files[r.File] += relativeImport(r.File, file, alias)
	child := model.Behavior{
		Inputs: ports, Outputs: []model.BehaviorPort{{Name: "result", Type: output.Type}},
		Nodes: []model.BehaviorNode{inner}, Bindings: map[string]model.BehaviorBinding{"result": {Node: inner.ID, Port: "result"}},
	}
	p.Files[file] = moduleHeader(module) + "\nbehavior " + r.Name + " " + render(map[string]any{"schemaVersion": pkg.SchemaVersion, "behavior": cloneJSON(child)}, 0) + "\n"
	return componentWithTestbench(p, file, r.Name)
}

// Resolve types through the validated graph, including literals, upstream
// component calls and structured nodes. Only the selected scalar interface is
// extracted; preceding nodes need not be scalar themselves.
func primitiveInterface(c Compilation, a Artifact, selected string) (map[string]model.BehaviorValueType, model.BehaviorValueType, error) {
	bundle, err := Bundle(c.Artifacts, a)
	if err != nil {
		return nil, model.BehaviorValueType{}, err
	}
	contracts, _ := model.BundleDataContracts(bundle)
	var pkg model.ComponentPackage
	_ = json.Unmarshal(a.Body, &pkg)
	order, err := model.BehaviorNodeOrder(*pkg.Behavior)
	if err != nil {
		return nil, model.BehaviorValueType{}, err
	}
	inputs := map[string]model.BehaviorValueType{}
	for _, p := range pkg.Behavior.Inputs {
		inputs[p.Name] = model.PortBehaviorType(p)
	}
	values := map[string]map[string]model.BehaviorValueType{}
	for _, node := range order {
		args := map[string]model.BehaviorValueType{}
		for name, binding := range node.Inputs {
			switch {
			case binding.Literal != nil:
				args[name] = model.ValueBehaviorType(*binding.Literal)
			case binding.Input != "":
				args[name] = inputs[binding.Input]
			default:
				args[name] = values[binding.Node][binding.Port]
			}
		}
		outputs := map[string]model.BehaviorValueType{}
		if node.Component != nil {
			for _, lock := range bundle.Packages {
				if lock.Package.ID == node.Component.PackageID && lock.Package.Version == node.Component.Version && lock.Digest == node.Component.Digest {
					for _, p := range lock.Package.Behavior.Outputs {
						outputs[p.Name] = model.PortBehaviorType(p)
					}
				}
			}
		} else {
			typ, e := model.StructuredPrimitiveOutputType(node, args, contracts)
			if e != nil {
				return nil, model.BehaviorValueType{}, e
			}
			outputs["result"] = typ
		}
		if node.ID == selected {
			return args, outputs["result"], nil
		}
		values[node.ID] = outputs
	}
	return nil, model.BehaviorValueType{}, fmt.Errorf("primitive not found")
}
