package configuration

import (
	"fmt"
	"path"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

type EditRequest struct {
	Project      Project `json:"project"`
	Kind         string  `json:"kind,omitempty"`
	ExpectedHash string  `json:"expectedHash"`
	File         string  `json:"file"`
	Declaration  string  `json:"declaration"`
	Path         string  `json:"path"`
	Value        any     `json:"value"`
	Name         string  `json:"name"`
	Offset       int     `json:"offset"`
}
type EditResult struct {
	Project     Project     `json:"project"`
	Compilation Compilation `json:"compilation"`
}
type textEdit struct {
	file       string
	start, end int
	text       string
}

func applyEdits(p Project, edits []textEdit) Project {
	out := p
	out.Lock = nil
	out.Files = map[string]string{}
	for k, v := range p.Files {
		out.Files[k] = v
	}
	sort.Slice(edits, func(i, j int) bool {
		if edits[i].file != edits[j].file {
			return edits[i].file < edits[j].file
		}
		return edits[i].start > edits[j].start
	})
	for _, e := range edits {
		s := out.Files[e.file]
		out.Files[e.file] = s[:e.start] + e.text + s[e.end:]
	}
	return out
}
func editorBase(r EditRequest) (Compilation, error) {
	if SourceHash(r.Project) != r.ExpectedHash {
		return Compilation{}, fmt.Errorf("source conflict: this edit was prepared for an older project")
	}
	c := Compile(r.Project)
	if !c.Valid {
		return c, fmt.Errorf("compile the current source before applying a structural edit")
	}
	return c, nil
}
func findDecl(c Compilation, file, name string) (Document, Declaration, bool) {
	for _, d := range c.Documents {
		if d.File == file {
			for _, decl := range d.Declarations {
				if decl.Name == name {
					return d, decl, true
				}
			}
		}
	}
	return Document{}, Declaration{}, false
}
func exactExpr(e *Expr, parts []string) (*Expr, bool) {
	if len(parts) == 0 {
		return e, true
	}
	k := parts[0]
	if e.Kind == "object" {
		for _, f := range e.Fields {
			if f.Name == k {
				return exactExpr(f.Value, parts[1:])
			}
		}
	}
	if e.Kind == "array" {
		for i, child := range e.Items {
			if fmt.Sprint(i) == k {
				return exactExpr(child, parts[1:])
			}
		}
	}
	return nil, false
}

// Patch edits one property in the original text. A referenced fixture remains a
// reference: users navigate to that fixture rather than silently expanding it.
func Patch(r EditRequest) (EditResult, error) {
	c, e := editorBase(r)
	if e != nil {
		return EditResult{}, e
	}
	_, d, ok := findDecl(c, r.File, r.Declaration)
	if !ok {
		return EditResult{}, fmt.Errorf("declaration not found")
	}
	parts := pathParts(r.Path)
	if len(parts) == 0 {
		return EditResult{}, fmt.Errorf("select a property")
	}
	target, ok := exactExpr(d.Value, parts)
	edit := textEdit{file: r.File}
	if ok {
		for _, doc := range c.Documents {
			if doc.File == r.File {
				for _, comment := range doc.Comments {
					if comment.Start >= target.Start && comment.End <= target.End {
						return EditResult{}, fmt.Errorf("this value contains comments; edit its fields individually or in source")
					}
				}
			}
		}
		edit.start, edit.end = target.Start, target.End
		edit.text = render(cloneJSON(r.Value), 0)
	} else {
		parent, ok := exactExpr(d.Value, parts[:len(parts)-1])
		if !ok || parent.Kind != "object" {
			return EditResult{}, fmt.Errorf("property is supplied by a fixture or constructor; navigate to its source definition")
		}
		edit.start, edit.end = parent.End-1, parent.End-1
		edit.text = "\n  " + literal(parts[len(parts)-1]) + " = " + render(cloneJSON(r.Value), 1) + ";\n"
	}
	p := applyEdits(r.Project, []textEdit{edit})
	next := Compile(p)
	return EditResult{p, next}, nil
}
func sameArtifacts(a, b Compilation) bool {
	if !a.Valid || !b.Valid || len(a.Artifacts) != len(b.Artifacts) || len(a.Experiments) != len(b.Experiments) {
		return false
	}
	for i := range a.Artifacts {
		if a.Artifacts[i].Kind != b.Artifacts[i].Kind || a.Artifacts[i].Digest != b.Artifacts[i].Digest {
			return false
		}
	}
	for i := range a.Experiments {
		left, right := a.Experiments[i], b.Experiments[i]
		left.Origin, right.Origin = Span{}, Span{}
		if Hash(left) != Hash(right) {
			return false
		}
	}
	return true
}
func Format(r EditRequest) (EditResult, error) {
	c, e := editorBase(r)
	if e != nil {
		return EditResult{}, e
	}
	files := sortedKeys(r.Project.Files)
	if r.File != "" {
		if _, ok := r.Project.Files[r.File]; !ok {
			return EditResult{}, fmt.Errorf("file missing")
		}
		files = []string{r.File}
	}
	edits := []textEdit{}
	for _, f := range files {
		edits = append(edits, textEdit{f, 0, len(r.Project.Files[f]), formatText(r.Project.Files[f])})
	}
	p := applyEdits(r.Project, edits)
	next := Compile(p)
	if !sameArtifacts(c, next) {
		return EditResult{}, fmt.Errorf("formatter semantic guard rejected the change")
	}
	return EditResult{p, next}, nil
}
func formatText(s string) string {
	tokens, _ := tokenize("", s)
	var b strings.Builder
	indent := 0
	lineStart := true
	previous := ""
	stack := []string{}
	write := func(s string) {
		if lineStart {
			b.WriteString(strings.Repeat("  ", indent))
			lineStart = false
		}
		b.WriteString(s)
	}
	nl := func() {
		if !lineStart {
			b.WriteByte('\n')
			lineStart = true
		}
		previous = ""
	}
	for _, t := range tokens {
		switch t.Kind {
		case "eof":
			continue
		case "comment":
			if !lineStart {
				write(" ")
			}
			write(t.Text)
			nl()
		case "{", "[":
			if previous != "" && previous != "." && previous != "(" {
				write(" ")
			}
			write(t.Text)
			stack = append(stack, t.Kind)
			indent++
			nl()
		case "}", "]":
			nl()
			if indent > 0 {
				indent--
			}
			write(t.Text)
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		case ";":
			write(";")
			nl()
		case ",":
			write(",")
			if len(stack) > 0 && stack[len(stack)-1] == "[" {
				nl()
			} else {
				write(" ")
			}
		case ".":
			write(".")
		case "(":
			write("(")
			stack = append(stack, "(")
		case ")":
			write(")")
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		case "=", ":":
			write(" " + t.Text + " ")
		default:
			if previous != "" && previous != "." && previous != "(" && previous != "=" && previous != ":" && previous != "," {
				write(" ")
			}
			write(t.Text)
		}
		if !lineStart {
			previous = t.Kind
		}
	}
	nl()
	return b.String()
}
func walkExpr(e *Expr, fn func(*Expr)) {
	fn(e)
	for _, v := range e.Items {
		walkExpr(v, fn)
	}
	for _, f := range e.Fields {
		walkExpr(f.Value, fn)
	}
}
func Rename(r EditRequest) (EditResult, error) {
	c, e := editorBase(r)
	if e != nil {
		return EditResult{}, e
	}
	if !identifier.MatchString(r.Name) || r.Name == "true" || r.Name == "false" || r.Name == "null" {
		return EditResult{}, fmt.Errorf("choose a safe, non-reserved identifier")
	}
	doc, d, ok := findDecl(c, r.File, r.Declaration)
	if !ok {
		return EditResult{}, fmt.Errorf("declaration not found")
	}
	if d.Kind == "parameter" || d.Kind == "experiment" {
		return EditResult{}, fmt.Errorf("parameters and experiment names identify specializations/evidence; change them explicitly in source")
	}
	for _, decl := range doc.Declarations {
		if decl.Name == r.Name {
			return EditResult{}, fmt.Errorf("name already declared")
		}
	}
	for _, imp := range doc.Imports {
		if imp.Alias == r.Name {
			return EditResult{}, fmt.Errorf("name already imported")
		}
	}
	edits := []textEdit{{r.File, d.NameSpan.Start, d.NameSpan.End, r.Name}}
	for _, document := range c.Documents {
		aliases := map[string]bool{}
		for _, imp := range document.Imports {
			if path.Join(path.Dir(document.File), imp.Path) == r.File {
				aliases[imp.Alias] = true
			}
		}
		rename := func(expr *Expr) {
			if expr.Kind != "reference" {
				return
			}
			parts := strings.Split(expr.Name, ".")
			n := -1
			if document.File == r.File && parts[0] == r.Declaration {
				n = 0
			} else if len(parts) > 1 && aliases[parts[0]] && parts[1] == r.Declaration {
				n = 1
			}
			if n >= 0 {
				parts[n] = r.Name
				edits = append(edits, textEdit{document.File, expr.Start, expr.End, strings.Join(parts, ".")})
			}
		}
		for _, decl := range document.Declarations {
			walkExpr(decl.Value, rename)
		}
		for _, imp := range document.Imports {
			if imp.Arguments != nil {
				walkExpr(imp.Arguments, rename)
			}
		}
	}
	if artifactKinds[d.Kind] {
		fields := map[string]bool{}
		if d.Value.Kind == "object" {
			for _, f := range d.Value.Fields {
				fields[f.Name] = true
			}
		}
		insert := ""
		if !fields["id"] {
			insert += "id = " + literal(doc.Module+"."+d.Name) + "; "
		}
		if !fields["name"] {
			insert += "name = " + literal(d.Name) + "; "
		}
		if insert != "" {
			if d.Value.Kind != "object" {
				return EditResult{}, fmt.Errorf("give this constructor-based artifact explicit id and name fields before renaming")
			}
			edits = append(edits, textEdit{r.File, d.Value.Start + 1, d.Value.Start + 1, "\n  " + insert + "\n"})
		}
	}
	p := applyEdits(r.Project, edits)
	next := Compile(p)
	if !sameArtifacts(c, next) {
		return EditResult{}, fmt.Errorf("rename would change immutable content or test evidence; edit source explicitly")
	}
	return EditResult{p, next}, nil
}

type Completion struct {
	Label    string `json:"label"`
	Kind     string `json:"kind"`
	Type     string `json:"type,omitempty"`
	Required bool   `json:"required,omitempty"`
	Detail   string `json:"detail,omitempty"`
}

func Complete(p Project, file, decl, fieldPath string, fallbackKind ...string) []Completion {
	out := []Completion{}
	doc, _ := Parse(file, p.Files[file])
	for _, d := range doc.Declarations {
		out = append(out, Completion{Label: d.Name, Kind: d.Kind})
	}
	for _, imp := range doc.Imports {
		child := path.Join(path.Dir(file), imp.Path)
		d, _ := Parse(child, p.Files[child])
		for _, s := range d.Declarations {
			if s.Kind != "parameter" {
				out = append(out, Completion{Label: imp.Alias + "." + s.Name, Kind: s.Kind})
			}
		}
	}
	for _, n := range []string{"lock", "ref", "bundle", "aspect", "use", "target", "record", "value", "body", "integer", "string", "boolean", "merge", "concat", "get"} {
		out = append(out, Completion{Label: n + "()", Kind: "constructor"})
	}
	declarations := doc.Declarations
	foundDeclaration := false
	for _, d := range declarations {
		if d.Name == decl {
			foundDeclaration = true
		}
	}
	if !foundDeclaration && len(fallbackKind) > 0 && artifactKinds[fallbackKind[0]] {
		declarations = append(declarations, Declaration{Name: decl, Kind: fallbackKind[0]})
	}
	for _, d := range declarations {
		if d.Name != decl {
			continue
		}
		v := artifactModel(d.Kind)
		if v == nil {
			continue
		}
		t := reflect.TypeOf(v).Elem()
		for _, key := range pathParts(fieldPath) {
			if _, e := strconv.Atoi(key); e == nil {
				for t.Kind() == reflect.Pointer {
					t = t.Elem()
				}
				if t.Kind() == reflect.Slice {
					t = t.Elem()
					continue
				}
			}
			for t.Kind() == reflect.Pointer || t.Kind() == reflect.Slice || t.Kind() == reflect.Map {
				t = t.Elem()
			}
			if t.Kind() != reflect.Struct {
				break
			}
			found := false
			for i := 0; i < t.NumField(); i++ {
				f := t.Field(i)
				if strings.Split(f.Tag.Get("json"), ",")[0] == key {
					t = f.Type
					found = true
					break
				}
			}
			if !found {
				break
			}
		}
		for t.Kind() == reflect.Pointer || t.Kind() == reflect.Slice || t.Kind() == reflect.Map {
			t = t.Elem()
		}
		if t.Kind() == reflect.Struct {
			for i := 0; i < t.NumField(); i++ {
				f := t.Field(i)
				name := strings.Split(f.Tag.Get("json"), ",")[0]
				if name != "" && name != "-" {
					out = append(out, Completion{Label: name, Kind: "field", Type: f.Type.String(), Required: !strings.Contains(f.Tag.Get("json"), "omitempty")})
				}
			}
		}
	}
	return out
}
func Definition(p Project, file string, offset int) (Span, error) {
	doc, ds := Parse(file, p.Files[file])
	if len(ds) > 0 {
		return Span{}, fmt.Errorf("parse current file first")
	}
	var selected *Expr
	for _, d := range doc.Declarations {
		walkExpr(d.Value, func(e *Expr) {
			if e.Kind == "reference" && offset >= e.Start && offset < e.End {
				selected = e
			}
		})
	}
	if selected == nil {
		return Span{}, fmt.Errorf("place the cursor on a symbol reference")
	}
	parts := strings.Split(selected.Name, ".")
	if len(parts) > 1 {
		for _, i := range doc.Imports {
			if parts[0] == i.Alias {
				file = path.Join(path.Dir(file), i.Path)
				doc, _ = Parse(file, p.Files[file])
				parts = parts[1:]
				break
			}
		}
	}
	for _, d := range doc.Declarations {
		if d.Name == parts[0] {
			return d.NameSpan, nil
		}
	}
	return Span{}, fmt.Errorf("symbol not found")
}
