package configuration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"software-studio/internal/assembly"
	"software-studio/internal/model"
	"software-studio/internal/system"
	"strconv"
	"strings"
)

type typeError struct{ Path, Message string }

func (e *typeError) Error() string { return e.Path + ": " + e.Message }
func typeShape(v any, t reflect.Type, path string) error {
	if t == reflect.TypeOf(json.RawMessage{}) || t.Kind() == reflect.Interface {
		return nil
	}
	if v == nil {
		switch t.Kind() {
		case reflect.Pointer, reflect.Slice, reflect.Map:
			return nil
		default:
			return &typeError{path, "null is not a " + t.String()}
		}
	}
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	fail := func(message string) error { return &typeError{path, message} }
	switch t.Kind() {
	case reflect.Struct:
		m, ok := v.(map[string]any)
		if !ok {
			return fail("expected an object")
		}
		fields := map[string]reflect.Type{}
		var collect func(reflect.Type)
		collect = func(t reflect.Type) {
			for i := 0; i < t.NumField(); i++ {
				f := t.Field(i)
				name := strings.Split(f.Tag.Get("json"), ",")[0]
				if name == "" && f.Anonymous && f.Type.Kind() == reflect.Struct {
					collect(f.Type)
				} else if name != "" && name != "-" {
					fields[name] = f.Type
				}
			}
		}
		collect(t)
		for _, k := range sortedKeys(m) {
			child, ok := fields[k]
			p := k
			if path != "" {
				p = path + "." + k
			}
			if !ok {
				return &typeError{p, "unknown field"}
			}
			if err := typeShape(m[k], child, p); err != nil {
				return err
			}
		}
	case reflect.Map:
		m, ok := v.(map[string]any)
		if !ok {
			return fail("expected a map")
		}
		for _, k := range sortedKeys(m) {
			if err := typeShape(m[k], t.Elem(), path+"."+k); err != nil {
				return err
			}
		}
	case reflect.Slice:
		values, ok := v.([]any)
		if !ok {
			return fail("expected an array")
		}
		for i, value := range values {
			if err := typeShape(value, t.Elem(), fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
	case reflect.String:
		if _, ok := v.(string); !ok {
			return fail("expected a string")
		}
	case reflect.Bool:
		if _, ok := v.(bool); !ok {
			return fail("expected a boolean")
		}
	case reflect.Int, reflect.Int64, reflect.Int32, reflect.Uint, reflect.Uint64, reflect.Float64, reflect.Float32:
		if _, ok := v.(json.Number); !ok {
			return fail("expected a number")
		}
	}
	return nil
}
func strictDecode(v any, out any) error {
	v = cloneJSON(v)
	if err := typeShape(v, reflect.TypeOf(out).Elem(), ""); err != nil {
		return err
	}
	b, e := json.Marshal(v)
	if e != nil {
		return e
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	return d.Decode(out)
}

// Only omitted, required collections get an empty default. Explicit null remains
// visible to model validation; optional collections retain their historic digest.
func collectionDefaults(v any, t reflect.Type) {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.Struct:
		m, ok := v.(map[string]any)
		if !ok {
			return
		}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			tag := strings.Split(f.Tag.Get("json"), ",")
			if tag[0] == "" || tag[0] == "-" {
				continue
			}
			child, exists := m[tag[0]]
			if !exists && !strings.Contains(f.Tag.Get("json"), "omitempty") {
				switch f.Type.Kind() {
				case reflect.Slice:
					m[tag[0]] = []any{}
				case reflect.Map:
					m[tag[0]] = map[string]any{}
				}
			}
			if exists {
				collectionDefaults(child, f.Type)
			}
		}
	case reflect.Slice:
		a, ok := v.([]any)
		if ok {
			for _, child := range a {
				collectionDefaults(child, t.Elem())
			}
		}
	case reflect.Map:
		m, ok := v.(map[string]any)
		if ok {
			for _, child := range m {
				collectionDefaults(child, t.Elem())
			}
		}
	}
}
func artifactModel(kind string) any {
	switch kind {
	case "data":
		return &model.DataContract{}
	case "behavior":
		return &model.ComponentPackage{}
	case "state":
		return &model.StateMachine{}
	case "messages":
		return &model.DeliveryModel{}
	case "workflow":
		return &model.WorkflowDefinition{}
	case "definition":
		return &system.ComponentDefinition{}
	case "system":
		return &system.SystemDesign{}
	case "timing":
		return &model.Project{}
	}
	return nil
}
func validateArtifact(kind string, body map[string]any) (a Artifact, issues []model.ValidationIssue, err error) {
	value := artifactModel(kind)
	if value == nil {
		return a, nil, fmt.Errorf("unknown artifact kind %s", kind)
	}
	collectionDefaults(body, reflect.TypeOf(value))
	if err = strictDecode(body, value); err != nil {
		return
	}
	a.Kind = kind
	switch v := value.(type) {
	case *model.DataContract:
		a.ID, a.Version = v.ID, v.Version
		issues = model.ValidateDataContract(*v)
		a.Digest, err = model.DataContractDigest(*v)
	case *model.ComponentPackage:
		a.ID, a.Version = v.ID, v.Version
		issues = model.ValidateComponentPackage(*v)
		a.Digest, err = model.ComponentDigest(*v)
	case *model.StateMachine:
		a.ID, a.Version = v.ID, v.Version
		issues = model.ValidateStateMachine(*v)
		a.Digest, err = model.StateMachineDigest(*v)
	case *model.DeliveryModel:
		a.ID, a.Version = v.ID, v.Version
		issues = model.ValidateDeliveryModel(*v)
		a.Digest, err = model.DeliveryModelDigest(*v)
	case *model.WorkflowDefinition:
		a.ID, a.Version = v.ID, v.Version
		issues = model.ValidateWorkflowDefinition(*v)
		a.Digest, err = model.WorkflowDigest(*v)
	case *system.ComponentDefinition:
		a.ID, a.Version = v.ID, v.Version
		issues = system.ValidateDefinition(*v).Issues
		a.Digest, err = system.DefinitionDigest(*v)
	case *system.SystemDesign:
		a.ID, a.Version = v.ID, v.Revision
		issues = system.Validate(*v).Issues
		a.Digest = Hash(*v)
	case *model.Project:
		a.ID = v.ID
		issues = model.Validate(*v)
		a.Digest = Hash(*v)
	}
	a.Body, _ = json.Marshal(value)
	return
}
func pathParts(path string) []string {
	return strings.FieldsFunc(path, func(r rune) bool { return r == '.' || r == '[' || r == ']' })
}
func locateExpr(e *Expr, path string) Span {
	at := e.Span
	for _, key := range pathParts(path) {
		var next *Expr
		if e.Kind == "object" {
			for _, f := range e.Fields {
				if f.Name == key {
					next = f.Value
					break
				}
			}
		} else if e.Kind == "array" {
			i, err := strconv.Atoi(key)
			if err == nil && i >= 0 && i < len(e.Items) {
				next = e.Items[i]
			}
		}
		if next == nil {
			return at
		}
		e = next
		at = e.Span
	}
	return at
}
func lookupPath(v any, path string) (any, bool) {
	for _, key := range pathParts(path) {
		switch x := v.(type) {
		case map[string]any:
			var ok bool
			v, ok = x[key]
			if !ok {
				return nil, false
			}
		case []any:
			i, e := strconv.Atoi(key)
			if e != nil || i < 0 || i >= len(x) {
				return nil, false
			}
			v = x[i]
		default:
			return nil, false
		}
	}
	return v, true
}
func Bundle(artifacts []Artifact, root Artifact) (model.ComponentBundle, error) {
	var p model.ComponentPackage
	if root.Kind != "behavior" {
		return model.ComponentBundle{}, fmt.Errorf("behavior root required")
	}
	if e := json.Unmarshal(root.Body, &p); e != nil {
		return model.ComponentBundle{}, e
	}
	if p.Behavior == nil {
		return model.ComponentBundle{}, assembly.ErrMissing
	}
	locks := []model.PackageLock{}
	seen := map[string]bool{root.ID + "@" + root.Version: true}
	var visit func(model.ComponentPackage) error
	visit = func(p model.ComponentPackage) error {
		if p.Behavior == nil {
			return nil
		}
		for _, node := range p.Behavior.Nodes {
			if node.Component == nil {
				continue
			}
			ref := *node.Component
			key := ref.PackageID + "@" + ref.Version
			if seen[key] {
				continue
			}
			seen[key] = true
			found := false
			for _, a := range artifacts {
				if a.Kind == "behavior" && a.ID == ref.PackageID && a.Version == ref.Version && a.Digest == ref.Digest {
					var child model.ComponentPackage
					_ = json.Unmarshal(a.Body, &child)
					locks = append(locks, model.PackageLock{Package: child, Digest: a.Digest})
					if e := visit(child); e != nil {
						return e
					}
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("missing exact behavior dependency %s", key)
			}
		}
		return nil
	}
	if e := visit(p); e != nil {
		return model.ComponentBundle{}, e
	}
	b, issues := model.PrepareComponentBundle(p, locks)
	if len(issues) > 0 {
		return b, fmt.Errorf("%s: %s", issues[0].Path, issues[0].Message)
	}
	return b, nil
}

// Library implements the same resolver used by installed catalog designs, without a database.
type Library struct{ Artifacts []Artifact }

func (l Library) find(kind, id, v, h string) (Artifact, error) {
	for _, a := range l.Artifacts {
		if a.Kind == kind && a.ID == id && a.Version == v && (h == "" || a.Digest == h) {
			return a, nil
		}
	}
	return Artifact{}, fmt.Errorf("%w: %s %s@%s", assembly.ErrMissing, kind, id, v)
}
func (l Library) Definitions(context.Context) ([]assembly.DefinitionRecord, error) {
	out := []assembly.DefinitionRecord{}
	for _, a := range l.Artifacts {
		if a.Kind == "definition" {
			var d system.ComponentDefinition
			_ = json.Unmarshal(a.Body, &d)
			out = append(out, assembly.DefinitionRecord{Definition: d})
		}
	}
	return out, nil
}
func (l Library) ComponentPackage(_ context.Context, id, v string) (p model.ComponentPackage, e error) {
	a, e := l.find("behavior", id, v, "")
	if e == nil {
		e = json.Unmarshal(a.Body, &p)
	}
	return
}
func (l Library) ComponentBundle(_ context.Context, r model.ComponentRef) (model.ComponentBundle, error) {
	a, e := l.find("behavior", r.PackageID, r.Version, r.Digest)
	if e != nil {
		return model.ComponentBundle{}, e
	}
	return Bundle(l.Artifacts, a)
}
func (l Library) StateMachine(_ context.Context, id, v, h string) (r model.StateMachineLock, e error) {
	a, e := l.find("state", id, v, h)
	if e == nil {
		r.Digest = a.Digest
		e = json.Unmarshal(a.Body, &r.Machine)
	}
	return
}
func (l Library) DeliveryModel(_ context.Context, id, v, h string) (r model.DeliveryModelLock, e error) {
	a, e := l.find("messages", id, v, h)
	if e == nil {
		r.Digest = a.Digest
		e = json.Unmarshal(a.Body, &r.Model)
	}
	return
}
func (l Library) WorkflowPackage(_ context.Context, id, v, h string) (r model.WorkflowLock, e error) {
	a, e := l.find("workflow", id, v, h)
	if e == nil {
		r.Digest = a.Digest
		e = json.Unmarshal(a.Body, &r.Workflow)
	}
	return
}
func (l Library) Catalog() []system.CatalogEntry {
	out := []system.CatalogEntry{}
	for _, a := range l.Artifacts {
		kind := a.Kind
		if kind == "behavior" {
			kind = "component"
		}
		entry := system.CatalogEntry{Kind: kind, ID: a.ID, Version: a.Version, Name: a.ID, Digest: a.Digest, Source: "configuration", Ready: true}
		if kind == "component" {
			var p model.ComponentPackage
			_ = json.Unmarshal(a.Body, &p)
			entry.Kernel = p.Kernel
		}
		if kind == "definition" {
			var d system.ComponentDefinition
			_ = json.Unmarshal(a.Body, &d)
			entry.Aspects = d.Aspects
		}
		out = append(out, entry)
	}
	return out
}
func (l Library) ResolveSystem(ctx context.Context, d system.SystemDesign) (system.Resolution, error) {
	return assembly.Resolve(ctx, l, d, l.Catalog())
}
func (c *compiler) validateClosure() {
	l := Library{c.result.Artifacts}
	exported := NewExporter()
	for _, a := range c.result.Artifacts {
		if _, err := exported.Add(a.Kind, decodeAny(a.Body)); err != nil {
			c.result.Diagnostics = append(c.result.Diagnostics, Diagnostic{Span: a.Origin, Code: "closure.conflict", Message: err.Error()})
		}
	}
	for _, a := range exported.Artifacts {
		if _, err := l.find(a.Kind, a.ID, a.Version, a.Digest); err != nil {
			c.result.Diagnostics = append(c.result.Diagnostics, Diagnostic{Span: Span{File: c.project.Entry, Line: 1, Column: 1}, Code: "closure.missing", Message: "Declare/import every embedded dependency: " + err.Error()})
		}
	}
	for _, a := range c.result.Artifacts {
		c.attempt(func() {
			switch a.Kind {
			case "behavior":
				var p model.ComponentPackage
				_ = json.Unmarshal(a.Body, &p)
				if p.Behavior != nil {
					if _, e := Bundle(l.Artifacts, a); e != nil {
						c.fail(a.Origin, "closure.behavior", e.Error())
					}
				}
			case "definition":
				var d system.ComponentDefinition
				_ = json.Unmarshal(a.Body, &d)
				r, e := assembly.CheckDefinition(context.Background(), l, d, l.Catalog())
				if e != nil {
					c.fail(a.Origin, "closure.definition", e.Error())
				}
				for _, i := range r.Issues {
					c.result.Diagnostics = append(c.result.Diagnostics, Diagnostic{Span: a.Origin, Code: "closure.definition", Path: i.Path, Message: i.Message})
				}
			case "system":
				var d system.SystemDesign
				_ = json.Unmarshal(a.Body, &d)
				r, e := l.ResolveSystem(context.Background(), d)
				if e != nil {
					c.fail(a.Origin, "closure.system", e.Error())
				}
				for _, i := range r.Issues {
					c.result.Diagnostics = append(c.result.Diagnostics, Diagnostic{Span: a.Origin, Code: "closure.system", Path: i.Path, Message: i.Message})
				}
			}
		})
	}
	for _, e := range c.result.Experiments {
		c.attempt(func() {
			if _, err := l.find(e.Target.Kind, e.Target.ID, e.Target.Version, e.Target.Digest); err != nil {
				c.fail(e.Origin, "experiment.target", err.Error())
			}
			if e.Target.Kind == "definition" {
				c.fail(e.Origin, "experiment.target", "Test an executable aspect or a composed system, not a manifest")
			}
			if len(e.Scenario) == 0 {
				c.fail(e.Origin, "experiment.scenario", "Scenario is required (use {} for timing models)")
			}
		})
	}
}
