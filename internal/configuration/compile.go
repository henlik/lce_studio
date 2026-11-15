package configuration

import (
	"encoding/json"
	"fmt"
	"math"
	"path"
	"sort"
	"strings"

	"software-studio/internal/model"
)

type semanticError struct{ Diagnostic }
type artifactValue struct{ artifact Artifact }
type moduleInstance struct {
	doc          *Document
	key          string
	parameters   map[string]any
	imports      map[string]*moduleInstance
	declarations map[string]*Declaration
	values       map[string]any
	active       map[string]bool
	failed       map[string]bool
}
type compiler struct {
	project    Project
	result     Compilation
	docs       map[string]*Document
	instances  map[string]*moduleInstance
	loading    map[string]bool
	identities map[string]Artifact
	work       int
	allocated  int
}

func validPath(file string) bool {
	return file != "" && len(file) < 240 && !strings.ContainsAny(file, "\\\x00") && !strings.HasPrefix(file, "/") && path.Clean(file) == file && file != ".." && !strings.HasPrefix(file, "../") && strings.HasSuffix(file, ".lce")
}
func Compile(project Project) (result Compilation) {
	c := compiler{project: project, docs: map[string]*Document{}, instances: map[string]*moduleInstance{}, loading: map[string]bool{}, identities: map[string]Artifact{}, result: Compilation{Language: Version, Diagnostics: []Diagnostic{}, Artifacts: []Artifact{}, Experiments: []Experiment{}, Documents: []Document{}}}
	c.result.SourceHash = SourceHash(project)
	add := func(message string) {
		c.result.Diagnostics = append(c.result.Diagnostics, Diagnostic{Span: Span{File: project.Entry, Line: 1, Column: 1}, Code: "project.invalid", Message: message})
	}
	if project.Language != Version {
		add("Project language must be 1.0")
	}
	if len(project.Files) < 1 || len(project.Files) > MaxFiles {
		add("A project requires 1–128 local .lce files")
	}
	if !validPath(project.Entry) {
		add("Entry must be a normalized relative .lce path")
	}
	if _, ok := project.Files[project.Entry]; !ok {
		add("Entry file is missing")
	}
	bytes := 0
	files := sortedKeys(project.Files)
	namespaces := map[string]string{}
	for _, file := range files {
		bytes += len(project.Files[file])
		if bytes > MaxProjectBytes {
			add("Combined source exceeds 4 MiB")
			break
		}
		if !validPath(file) {
			add("Invalid local file path " + file)
			continue
		}
		doc, diagnostics := Parse(file, project.Files[file])
		c.result.Diagnostics = append(c.result.Diagnostics, diagnostics...)
		if previous, exists := namespaces[doc.Module]; exists && len(diagnostics) == 0 {
			add("Duplicate module namespace " + doc.Module + " in " + previous + " and " + file)
		}
		namespaces[doc.Module] = file
		c.docs[file] = &doc
		c.result.Documents = append(c.result.Documents, doc)
	}
	if len(c.result.Diagnostics) > 0 {
		return c.result
	}
	c.attempt(func() { c.load(project.Entry, project.Parameters, Span{File: project.Entry, Line: 1, Column: 1}) })
	if len(c.result.Diagnostics) == 0 {
		c.validateClosure()
	}
	sort.Slice(c.result.Artifacts, func(i, j int) bool {
		a, b := c.result.Artifacts[i], c.result.Artifacts[j]
		return a.Kind+"/"+a.ID+"/"+a.Version < b.Kind+"/"+b.ID+"/"+b.Version
	})
	sort.Slice(c.result.Experiments, func(i, j int) bool { return c.result.Experiments[i].Name < c.result.Experiments[j].Name })
	lock := Lockfile{Compiler: CompilerVersion, SourceHash: c.result.SourceHash, Files: map[string]string{}, Artifacts: []Target{}}
	for _, file := range files {
		lock.Files[file] = Hash(project.Files[file])
	}
	for _, a := range c.result.Artifacts {
		lock.Artifacts = append(lock.Artifacts, Target{a.Kind, a.ID, a.Version, a.Digest})
	}
	c.result.Lock = lock
	if project.Lock != nil && Hash(*project.Lock) != Hash(lock) {
		add("Lock mismatch: source, parameters, compiler or artifact closure changed; review and regenerate the lock explicitly")
	}
	c.result.Valid = len(c.result.Diagnostics) == 0
	if !c.result.Valid {
		c.result.Artifacts = []Artifact{}
		c.result.Experiments = []Experiment{}
	}
	if c.result.Valid {
		w := Composition(c.result)
		c.result.Workspace = &w
	}
	return c.result
}
func sortedKeys[T any](m map[string]T) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
func (c *compiler) fail(at Span, code, message string) {
	panic(semanticError{Diagnostic{Span: at, Code: code, Message: message}})
}
func (c *compiler) attempt(fn func()) {
	defer func() {
		if e := recover(); e != nil {
			if d, ok := e.(semanticError); ok {
				if len(c.result.Diagnostics) < 200 {
					c.result.Diagnostics = append(c.result.Diagnostics, d.Diagnostic)
				}
			} else {
				panic(e)
			}
		}
	}()
	fn()
}
func (c *compiler) load(file string, args map[string]any, at Span) *moduleInstance {
	if len(c.instances) >= 256 {
		c.fail(at, "limit.modules", "At most 256 module specializations are supported")
	}
	if c.loading[file] {
		c.fail(at, "import.cycle", "Import cycle through "+file)
	}
	doc, ok := c.docs[file]
	if !ok {
		c.fail(at, "import.missing", "Missing local module "+file)
	}
	key := file + ":" + Hash(args)
	if m, ok := c.instances[key]; ok {
		return m
	}
	c.loading[file] = true
	defer delete(c.loading, file)
	m := &moduleInstance{doc: doc, key: key, parameters: map[string]any{}, imports: map[string]*moduleInstance{}, declarations: map[string]*Declaration{}, values: map[string]any{}, active: map[string]bool{}, failed: map[string]bool{}}
	for i := range doc.Declarations {
		d := &doc.Declarations[i]
		if _, ok := m.declarations[d.Name]; ok {
			c.fail(d.NameSpan, "symbol.duplicate", "Duplicate symbol "+d.Name)
		}
		m.declarations[d.Name] = d
	}
	for _, imp := range doc.Imports {
		if _, ok := m.declarations[imp.Alias]; ok {
			c.fail(imp.AliasSpan, "symbol.duplicate", "Import alias collides with a declaration")
		}
		if _, ok := m.imports[imp.Alias]; ok {
			c.fail(imp.AliasSpan, "symbol.duplicate", "Duplicate import alias")
		}
		m.imports[imp.Alias] = nil
	}
	for _, d := range doc.Declarations {
		if d.Kind != "parameter" {
			continue
		}
		var value any
		if v, ok := args[d.Name]; ok {
			value = cloneJSON(v)
		} else {
			value = c.eval(m, d.Value)
		}
		if !parameterMatches(d.Type, value) {
			c.fail(d.Value.Span, "parameter.type", "Parameter "+d.Name+" requires "+d.Type)
		}
		m.parameters[d.Name] = value
		m.values[d.Name] = value
	}
	for _, name := range sortedKeys(args) {
		d, ok := m.declarations[name]
		if !ok || d.Kind != "parameter" {
			c.fail(at, "parameter.unknown", "Unknown parameter "+name)
		}
	}
	key = file + ":" + Hash(m.parameters)
	m.key = key
	if prior, ok := c.instances[key]; ok {
		return prior
	}
	for _, imp := range doc.Imports {
		if strings.ContainsAny(imp.Path, "\\\x00") || strings.HasPrefix(imp.Path, "/") {
			c.fail(imp.Span, "import.path", "Imports must be local relative .lce paths")
		}
		child := path.Join(path.Dir(file), imp.Path)
		if !validPath(child) {
			c.fail(imp.Span, "import.path", "Import leaves the project or has an invalid path")
		}
		values := map[string]any{}
		if imp.Arguments != nil {
			values = c.eval(m, imp.Arguments).(map[string]any)
		}
		m.imports[imp.Alias] = c.load(child, values, imp.Span)
	}
	c.instances[key] = m
	for _, d := range doc.Declarations {
		if d.Kind != "parameter" {
			c.attempt(func() { c.resolve(m, d.Name, d.NameSpan) })
		}
	}
	return m
}
func parameterMatches(kind string, v any) bool {
	switch kind {
	case "string":
		_, ok := v.(string)
		return ok
	case "boolean":
		_, ok := v.(bool)
		return ok
	case "integer", "number":
		n, ok := v.(json.Number)
		if !ok {
			return false
		}
		f, err := n.Float64()
		if err != nil || math.IsInf(f, 0) || math.IsNaN(f) {
			return false
		}
		return kind == "number" || math.Trunc(f) == f && math.Abs(f) <= float64(model.MaxBehaviorInteger)
	}
	return false
}
func cloneJSON(v any) any {
	b, _ := json.Marshal(v)
	var copy any
	d := json.NewDecoder(strings.NewReader(string(b)))
	d.UseNumber()
	_ = d.Decode(&copy)
	return copy
}
func (c *compiler) resolve(m *moduleInstance, name string, at Span) any {
	if value, ok := m.values[name]; ok {
		return value
	}
	if m.failed[name] {
		c.fail(at, "symbol.invalid", "Referenced declaration "+name+" is invalid")
	}
	d, ok := m.declarations[name]
	if !ok {
		c.fail(at, "symbol.unknown", "Unknown symbol "+name)
	}
	if d.Kind == "parameter" {
		c.fail(at, "parameter.order", "Parameter defaults may refer only to earlier parameters")
	}
	if m.active[name] {
		c.fail(at, "symbol.cycle", "Cyclic definition involving "+name)
	}
	m.active[name] = true
	success := false
	defer func() {
		delete(m.active, name)
		if !success {
			m.failed[name] = true
		}
	}()
	value := c.eval(m, d.Value)
	c.bound(d.Value.Span, value)
	if d.Kind == "experiment" {
		var body struct {
			Target   Target          `json:"target"`
			Scenario json.RawMessage `json:"scenario"`
			Expect   map[string]any  `json:"expect"`
		}
		if err := strictDecode(value, &body); err != nil {
			c.fail(d.Value.Span, "experiment.shape", err.Error())
		}
		if len(body.Expect) == 0 || len(body.Expect) > 64 {
			c.fail(d.Value.Span, "experiment.expect", "Each experiment requires 1–64 independent expected result paths")
		}
		if len(c.result.Experiments) >= 128 {
			c.fail(d.Span, "limit.experiments", "At most 128 experiments are supported")
		}
		c.result.Experiments = append(c.result.Experiments, Experiment{Name: m.doc.Module + "." + name + specialization(m), Target: body.Target, Scenario: body.Scenario, Expect: body.Expect, Origin: d.Span})
	} else if d.Kind != "fixture" {
		object, ok := value.(map[string]any)
		if !ok {
			c.fail(d.Value.Span, "artifact.object", "Artifact body must evaluate to an object")
		}
		body := cloneJSON(object).(map[string]any)
		defaultField := func(k string, v any) {
			if _, ok := body[k]; !ok {
				body[k] = v
			}
		}
		defaultField("id", m.doc.Module+"."+name)
		if len(m.parameters) > 0 {
			if id, ok := body["id"].(string); ok {
				body["id"] = id + specialization(m)
			}
		}
		defaultField("name", name)
		if d.Kind == "system" {
			defaultField("revision", m.doc.Version)
		} else if d.Kind != "timing" {
			defaultField("version", m.doc.Version)
		}
		if d.Kind != "definition" && d.Kind != "system" && d.Kind != "timing" {
			defaultField("author", "LCE configuration author")
			defaultField("license", "UNLICENSED")
		}
		schema := 1
		if d.Kind == "behavior" || d.Kind == "timing" {
			schema = 3
		}
		defaultField("schemaVersion", schema)
		if d.Kind == "behavior" {
			defaultField("kernel", "composite")
			defaultField("capabilities", []any{"design", "evaluate"})
		}
		a, issues, err := validateArtifact(d.Kind, body)
		if err != nil && len(issues) == 0 {
			at := d.Value.Span
			if typed, ok := err.(*typeError); ok {
				at = locateExpr(d.Value, typed.Path)
			}
			c.fail(at, "artifact.type", err.Error())
		}
		if len(issues) > 0 {
			for _, issue := range issues {
				c.result.Diagnostics = append(c.result.Diagnostics, Diagnostic{Span: locateExpr(d.Value, issue.Path), Code: "model.invalid", Path: issue.Path, Message: issue.Message})
			}
			c.fail(d.Span, "artifact.invalid", "Invalid "+d.Kind+" declaration "+name)
		}
		a.Origin = d.Span
		a.Symbol = fileSymbol(m, name)
		if a.Version == "" {
			a.Version = m.doc.Version
		}
		key := a.Kind + "/" + a.ID + "/" + a.Version
		if prev, exists := c.identities[key]; exists {
			if prev.Digest != a.Digest {
				c.fail(d.Span, "artifact.conflict", "Different content has the same immutable identity "+key)
			}
		} else {
			if len(c.result.Artifacts) >= MaxArtifacts {
				c.fail(d.Span, "limit.artifacts", "At most 512 artifacts are supported")
			}
			c.identities[key] = a
			c.result.Artifacts = append(c.result.Artifacts, a)
		}
		value = artifactValue{a}
	}
	c.allocated += c.bound(d.Value.Span, value)
	if c.allocated > 32<<20 {
		c.fail(d.Span, "limit.memory", "Resolved declarations exceed the 32 MiB aggregate budget")
	}
	m.values[name] = value
	success = true
	return value
}
func specialization(m *moduleInstance) string {
	if len(m.parameters) == 0 {
		return ""
	}
	return ".p" + Hash(m.parameters)[:12]
}
func fileSymbol(m *moduleInstance, name string) string {
	return m.doc.File + "#" + name + specialization(m)
}
func (c *compiler) eval(m *moduleInstance, e *Expr) any {
	c.work++
	if c.work > 300000 {
		c.fail(e.Span, "limit.evaluation", "Configuration evaluation budget exceeded")
	}
	switch e.Kind {
	case "literal":
		return e.Value
	case "array":
		out := []any{}
		for _, child := range e.Items {
			out = append(out, c.eval(m, child))
		}
		c.bound(e.Span, out)
		return out
	case "object":
		out := map[string]any{}
		for _, f := range e.Fields {
			out[f.Name] = c.eval(m, f.Value)
		}
		c.bound(e.Span, out)
		return out
	case "reference":
		parts := strings.Split(e.Name, ".")
		owner := m
		if imported, ok := m.imports[parts[0]]; ok {
			if imported == nil {
				c.fail(e.Span, "import.order", "Import arguments cannot refer to an unresolved module")
			}
			if len(parts) < 2 {
				c.fail(e.Span, "import.symbol", "Select a symbol from the imported module")
			}
			owner = imported
			parts = parts[1:]
		}
		value := c.resolve(owner, parts[0], e.Span)
		for _, field := range parts[1:] {
			object, ok := value.(map[string]any)
			if !ok {
				c.fail(e.Span, "symbol.member", "Only fixture objects expose member paths; use constructors for artifacts")
			}
			value, ok = object[field]
			if !ok {
				c.fail(e.Span, "symbol.member", "Unknown fixture field "+field)
			}
		}
		return value
	case "call":
		args := []any{}
		for _, arg := range e.Items {
			args = append(args, c.eval(m, arg))
		}
		value := c.call(e, args)
		c.bound(e.Span, value)
		return value
	}
	c.fail(e.Span, "expression.kind", "Unsupported expression")
	return nil
}
func (c *compiler) call(e *Expr, args []any) any {
	fail := func(message string) { c.fail(e.Span, "constructor.invalid", message) }
	count := func(n int) {
		if len(args) != n {
			fail(fmt.Sprintf("%s expects %d arguments", e.Name, n))
		}
	}
	artifact := func(index int) Artifact {
		if index >= len(args) {
			fail("Missing artifact argument")
		}
		a, ok := args[index].(artifactValue)
		if !ok {
			fail("Expected an artifact symbol")
		}
		return a.artifact
	}
	switch e.Name {
	case "body":
		count(1)
		a := artifact(0)
		return decodeAny(a.Body)
	case "target":
		count(1)
		a := artifact(0)
		return cloneJSON(Target{a.Kind, a.ID, a.Version, a.Digest})
	case "lock":
		count(1)
		a := artifact(0)
		key := map[string]string{"behavior": "package", "data": "contract", "state": "machine", "messages": "model", "workflow": "workflow"}[a.Kind]
		if key == "" {
			fail("This artifact kind has no embedded lock")
		}
		return map[string]any{key: decodeAny(a.Body), "digest": a.Digest}
	case "ref":
		count(1)
		a := artifact(0)
		key := map[string]string{"behavior": "packageId", "state": "machineId"}[a.Kind]
		if key == "" {
			fail("ref requires a behavior or state artifact")
		}
		return map[string]any{key: a.ID, "version": a.Version, "digest": a.Digest}
	case "use":
		count(1)
		a := artifact(0)
		kind := a.Kind
		if kind == "behavior" {
			kind = "component"
		}
		if kind != "component" && kind != "definition" {
			fail("use requires a behavior or component definition")
		}
		return map[string]any{"kind": kind, "id": a.ID, "version": a.Version}
	case "aspect":
		count(2)
		slot, ok := args[0].(string)
		if !ok {
			fail("Aspect slot must be a string")
		}
		a := artifact(1)
		kind := a.Kind
		if kind == "behavior" {
			kind = "component"
		}
		return map[string]any{"slot": slot, "kind": kind, "id": a.ID, "version": a.Version, "digest": a.Digest}
	case "bundle":
		count(1)
		a := artifact(0)
		if a.Kind != "behavior" {
			fail("bundle requires a behavior artifact")
		}
		bundle, err := Bundle(c.result.Artifacts, a)
		if err != nil {
			fail(err.Error())
		}
		return cloneJSON(bundle)
	case "record":
		count(2)
		a := artifact(0)
		name, ok := args[1].(string)
		if a.Kind != "data" || !ok {
			fail("record requires a data artifact and record name")
		}
		var contract model.DataContract
		_ = json.Unmarshal(a.Body, &contract)
		found := false
		for _, r := range contract.Records {
			found = found || r.Name == name
		}
		if !found {
			fail("Unknown data record " + name)
		}
		return map[string]any{"contractId": a.ID, "version": a.Version, "digest": a.Digest, "record": name}
	case "integer", "number", "string", "boolean":
		count(1)
		if !parameterMatches(e.Name, args[0]) {
			fail("Incorrect scalar value for " + e.Name)
		}
		if e.Name == "number" {
			fail("Runtime values support integer, string and boolean; use number parameters only for configuration fields")
		}
		return map[string]any{"type": e.Name, e.Name: args[0]}
	case "value":
		count(2)
		ref, ok := args[0].(map[string]any)
		if !ok {
			fail("value requires a record type reference")
		}
		var r model.DataTypeRef
		if err := strictDecode(ref, &r); err != nil {
			fail(err.Error())
		}
		if issues := model.ValidateDataTypeRef(r); len(issues) > 0 {
			fail(issues[0].Message)
		}
		switch value := args[1].(type) {
		case map[string]any:
			return map[string]any{"type": "record", "recordRef": ref, "record": value}
		case []any:
			return map[string]any{"type": "collection", "recordRef": ref, "collection": value}
		default:
			fail("value requires a record field map or record collection")
		}
	case "merge":
		count(2)
		a, ok := args[0].(map[string]any)
		if !ok {
			fail("merge expects objects")
		}
		b, ok := args[1].(map[string]any)
		if !ok {
			fail("merge expects objects")
		}
		return mergeObjects(a, b)
	case "concat":
		if len(args) < 1 || len(args) > 32 {
			fail("concat accepts 1–32 strings")
		}
		var b strings.Builder
		for _, arg := range args {
			s, ok := arg.(string)
			if !ok {
				fail("concat arguments must be strings")
			}
			b.WriteString(s)
		}
		if b.Len() > 65536 {
			fail("Concatenated text exceeds 64 KiB")
		}
		return b.String()
	case "get":
		count(2)
		key, ok := args[1].(string)
		if !ok {
			fail("get requires a dotted field path")
		}
		value, ok := lookupPath(args[0], key)
		if !ok {
			fail("Unknown object path " + key)
		}
		return value
	default:
		fail("Unknown constructor " + e.Name)
	}
	return nil
}
func mergeObjects(a, b map[string]any) map[string]any {
	out := cloneJSON(a).(map[string]any)
	for key, value := range b {
		left, lok := out[key].(map[string]any)
		right, rok := value.(map[string]any)
		if lok && rok {
			out[key] = mergeObjects(left, right)
		} else {
			out[key] = cloneJSON(value)
		}
	}
	return out
}
func decodeAny(data []byte) any {
	var v any
	d := json.NewDecoder(strings.NewReader(string(data)))
	d.UseNumber()
	_ = d.Decode(&v)
	return v
}

// Count expanded values before any JSON clone. Shared fixtures may otherwise
// describe an exponentially large tree with a very small source file.
func (c *compiler) bound(at Span, value any) int {
	remaining := MaxProjectBytes
	var count func(any, int)
	count = func(v any, depth int) {
		remaining -= 8
		if remaining < 0 || depth > 80 {
			c.fail(at, "limit.expansion", "Expanded configuration value exceeds 4 MiB or 80 levels")
		}
		switch x := v.(type) {
		case string:
			remaining -= len(x)
		case map[string]any:
			for k, child := range x {
				remaining -= len(k)
				count(child, depth+1)
			}
		case []any:
			for _, child := range x {
				count(child, depth+1)
			}
		case artifactValue:
			remaining -= len(x.artifact.Body)
		}
	}
	count(value, 0)
	if remaining < 0 {
		c.fail(at, "limit.expansion", "Expanded configuration value exceeds 4 MiB")
	}
	return MaxProjectBytes - remaining
}
