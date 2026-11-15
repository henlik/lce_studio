package configuration

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

type token struct {
	Span
	Kind, Text string
}
type parseError struct{ Diagnostic }

var identifier = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{0,63}$`)
var moduleID = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_.-]{0,79}$`)
var versionID = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)
var artifactKinds = map[string]bool{"data": true, "behavior": true, "state": true, "messages": true, "workflow": true, "definition": true, "system": true, "timing": true}

type syntaxParser struct {
	tokens       []token
	index, depth int
	doc          Document
}

func tokenize(file, input string) (out []token, diagnostics []Diagnostic) {
	line, column := 1, 1
	for i := 0; i < len(input); {
		start := i
		pos := Span{File: file, Start: i, Line: line, Column: column}
		r, n := utf8.DecodeRuneInString(input[i:])
		if unicode.IsSpace(r) {
			i += n
			if r == '\n' {
				line++
				column = 1
			} else {
				column++
			}
			continue
		}
		advance := func(end int) {
			for _, c := range input[i:end] {
				if c == '\n' {
					line++
					column = 1
				} else {
					column++
				}
			}
			i = end
		}
		kind := ""
		end := i + n
		switch {
		case strings.HasPrefix(input[i:], "--"):
			kind = "comment"
			for end < len(input) && input[end] != '\n' {
				end++
			}
		case r == '"':
			kind = "string"
			escaped, closed := false, false
			for end < len(input) {
				c := input[end]
				end++
				if escaped {
					escaped = false
					continue
				}
				if c == '\\' {
					escaped = true
					continue
				}
				if c == '"' {
					closed = true
					break
				}
			}
			var value string
			if !closed || json.Unmarshal([]byte(input[i:end]), &value) != nil {
				pos.End = end
				diagnostics = append(diagnostics, Diagnostic{Span: pos, Code: "syntax.string", Message: "Use a closed JSON-escaped quoted string"})
				return
			}
		case r >= '0' && r <= '9' || r == '-' && end < len(input) && input[end] >= '0' && input[end] <= '9':
			kind = "number"
			for end < len(input) && strings.ContainsRune("0123456789.eE+-", rune(input[end])) {
				end++
			}
			if !json.Valid([]byte(input[i:end])) {
				pos.End = end
				diagnostics = append(diagnostics, Diagnostic{Span: pos, Code: "syntax.number", Message: "Invalid decimal number"})
				return
			}
		case r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z':
			kind = "name"
			for end < len(input) {
				c := input[end]
				if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_') {
					break
				}
				end++
			}
		case strings.ContainsRune("{}[](),;=:.+", r):
			kind = string(r)
		default:
			pos.End = end
			diagnostics = append(diagnostics, Diagnostic{Span: pos, Code: "syntax.character", Message: fmt.Sprintf("Unexpected character %q", r)})
			return
		}
		advance(end)
		pos.End = i
		out = append(out, token{Span: pos, Kind: kind, Text: input[start:i]})
		if len(out) > 100000 {
			diagnostics = append(diagnostics, Diagnostic{Span: pos, Code: "limit.tokens", Message: "A file supports at most 100000 tokens"})
			return
		}
	}
	out = append(out, token{Span: Span{File: file, Start: len(input), End: len(input), Line: line, Column: column}, Kind: "eof"})
	return
}
func Parse(file, input string) (doc Document, diagnostics []Diagnostic) {
	doc = Document{File: file, Imports: []Import{}, Declarations: []Declaration{}, Comments: []Span{}}
	if len(input) > MaxFileBytes || !utf8.ValidString(input) {
		return doc, []Diagnostic{{Span: Span{File: file, Line: 1, Column: 1}, Code: "limit.source", Message: "Each file must be UTF-8 and at most 512 KiB"}}
	}
	tokens, diags := tokenize(file, input)
	if len(diags) > 0 {
		return doc, diags
	}
	p := syntaxParser{doc: doc}
	for _, t := range tokens {
		if t.Kind == "comment" {
			p.doc.Comments = append(p.doc.Comments, t.Span)
		} else {
			p.tokens = append(p.tokens, t)
		}
	}
	defer func() {
		if failure := recover(); failure != nil {
			if e, ok := failure.(parseError); ok {
				diagnostics = append(diagnostics, e.Diagnostic)
				doc = p.doc
			} else {
				panic(failure)
			}
		}
	}()
	p.word("language")
	if p.string() != Version {
		p.fail("version.unsupported", "Supported language version is \"1.0\"")
	}
	p.take(";")
	p.word("module")
	p.doc.Module = p.string()
	if !moduleID.MatchString(p.doc.Module) || !strings.Contains(p.doc.Module, ".") {
		p.fail("module.identity", "Use a namespaced module ID of at most 80 characters")
	}
	p.word("version")
	p.doc.Version = p.string()
	if !versionID.MatchString(p.doc.Version) || len(p.doc.Version) > 32 {
		p.fail("module.version", "Use an exact major.minor.patch module version")
	}
	p.take(";")
	for p.peek().Kind != "eof" {
		before := p.index
		func() {
			defer func() {
				if failure := recover(); failure != nil {
					if e, ok := failure.(parseError); ok {
						diagnostics = append(diagnostics, e.Diagnostic)
						p.recoverDeclaration(before)
					} else {
						panic(failure)
					}
				}
			}()
			p.declaration()
		}()
		if len(diagnostics) >= 50 {
			break
		}
	}
	return p.doc, diagnostics
}
func (p *syntaxParser) recoverDeclaration(start int) {
	p.depth = 0
	depth := 0
	p.index = start
	for p.peek().Kind != "eof" {
		t := p.peek()
		p.index++
		switch t.Kind {
		case "{", "[", "(":
			depth++
		case "}", "]", ")":
			depth--
		case ";":
			if depth <= 0 {
				return
			}
		}
		if depth == 0 && t.Kind == "}" {
			if p.peek().Kind == ";" {
				p.index++
			}
			return
		}
	}
}
func (p *syntaxParser) declaration() {
	t := p.peek()
	kind := strings.ToLower(p.name().Text)
	if kind == "import" {
		item := Import{Span: t.Span, Path: p.string()}
		p.word("as")
		alias := p.name()
		item.Alias = alias.Text
		item.AliasSpan = alias.Span
		if p.isWord("with") {
			p.index++
			item.Arguments = p.expression()
			if item.Arguments.Kind != "object" {
				p.fail("import.parameters", "Import arguments must be an object")
			}
		}
		item.End = p.take(";").End
		p.doc.Imports = append(p.doc.Imports, item)
		return
	}
	if kind != "parameter" && kind != "fixture" && kind != "experiment" && !artifactKinds[kind] {
		p.fail("declaration.kind", "Expected import, parameter, fixture, data, behavior, state, messages, workflow, definition, system, timing or experiment")
	}
	name := p.name()
	d := Declaration{Span: t.Span, Kind: kind, Name: name.Text, NameSpan: name.Span}
	if kind == "parameter" {
		p.take(":")
		d.Type = strings.ToLower(p.name().Text)
		if d.Type != "integer" && d.Type != "number" && d.Type != "boolean" && d.Type != "string" {
			p.fail("parameter.type", "Parameter type must be integer, number, boolean or string")
		}
	}
	if kind == "parameter" || kind == "fixture" {
		p.take("=")
	}
	d.Value = p.expression()
	d.End = d.Value.End
	if p.peek().Kind == ";" {
		d.End = p.take(";").End
	} else if kind == "parameter" || kind == "fixture" {
		p.take(";")
	}
	if artifactKinds[kind] || kind == "experiment" {
		if d.Value.Kind != "object" && d.Value.Kind != "call" {
			p.fail("declaration.body", "Artifact and experiment bodies must be objects or object-producing calls")
		}
	}
	p.doc.Declarations = append(p.doc.Declarations, d)
}
func (p *syntaxParser) expression() *Expr {
	p.depth++
	defer func() { p.depth-- }()
	if p.depth > 64 {
		p.fail("limit.depth", "Expression nesting exceeds 64")
	}
	t := p.peek()
	e := &Expr{Span: t.Span}
	switch t.Kind {
	case "string":
		e.Kind = "literal"
		e.Value = p.string()
	case "number":
		p.index++
		e.Kind = "literal"
		e.Value = json.Number(t.Text)
	case "name":
		p.index++
		switch strings.ToLower(t.Text) {
		case "true":
			e.Kind = "literal"
			e.Value = true
		case "false":
			e.Kind = "literal"
			e.Value = false
		case "null":
			e.Kind = "literal"
		default:
			e.Kind = "reference"
			e.Name = t.Text
			for p.peek().Kind == "." {
				p.index++
				e.Name += "." + p.name().Text
				e.End = p.tokens[p.index-1].End
			}
			if p.peek().Kind == "(" {
				e.Kind = "call"
				p.index++
				for p.peek().Kind != ")" {
					e.Items = append(e.Items, p.expression())
					if p.peek().Kind != "," {
						break
					}
					p.index++
				}
				e.End = p.take(")").End
			}
		}
	case "{":
		p.index++
		e.Kind = "object"
		e.Fields = []Field{}
		names := map[string]bool{}
		for p.peek().Kind != "}" {
			key := p.peek()
			var name string
			if key.Kind == "string" {
				name = p.string()
			} else {
				name = p.name().Text
			}
			if names[name] {
				p.fail("object.duplicate", "Duplicate field "+name)
			}
			names[name] = true
			p.take("=")
			value := p.expression()
			e.Fields = append(e.Fields, Field{Name: name, Span: key.Span, Value: value})
			p.take(";")
		}
		e.End = p.take("}").End
	case "[":
		p.index++
		e.Kind = "array"
		e.Items = []*Expr{}
		for p.peek().Kind != "]" {
			e.Items = append(e.Items, p.expression())
			if p.peek().Kind != "," {
				break
			}
			p.index++
		}
		e.End = p.take("]").End
	default:
		p.fail("expression.expected", "Expected a literal, object, array, symbol or constructor")
	}
	if e.End == t.End && p.index > 0 {
		e.End = p.tokens[p.index-1].End
	}
	return e
}
func (p *syntaxParser) peek() token {
	if p.index >= len(p.tokens) {
		return p.tokens[len(p.tokens)-1]
	}
	return p.tokens[p.index]
}
func (p *syntaxParser) take(kind string) token {
	t := p.peek()
	if t.Kind != kind {
		p.fail("syntax.expected", "Expected "+kind+", found "+t.Kind)
	}
	p.index++
	return t
}
func (p *syntaxParser) name() token {
	t := p.take("name")
	if !identifier.MatchString(t.Text) {
		p.fail("name.invalid", "Identifiers contain 1–64 letters, digits or underscores, starting with a letter")
	}
	return t
}
func (p *syntaxParser) isWord(word string) bool {
	return p.peek().Kind == "name" && strings.EqualFold(p.peek().Text, word)
}
func (p *syntaxParser) word(word string) {
	if !p.isWord(word) {
		p.fail("syntax.expected", "Expected "+word)
	}
	p.index++
}
func (p *syntaxParser) string() string {
	t := p.take("string")
	var v string
	_ = json.Unmarshal([]byte(t.Text), &v)
	return v
}
func (p *syntaxParser) fail(code, message string) {
	panic(parseError{Diagnostic{Span: p.peek().Span, Code: code, Message: message}})
}
