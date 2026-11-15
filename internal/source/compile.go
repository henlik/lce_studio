// Package source implements the experimental LCE Assembly 0.1 frontend.
// It is VHDL-inspired syntax, not an IEEE VHDL parser or execution engine.
package source

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"software-studio/internal/system"
)

const Version = "lce-assembly-0.1"
const MaxBytes = 64 << 10

type Position struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}
type Diagnostic struct {
	Position
	Path    string `json:"path"`
	Message string `json:"message"`
}
type Result struct {
	Language    string               `json:"language"`
	Valid       bool                 `json:"valid"`
	Design      *system.SystemDesign `json:"design,omitempty"`
	Diagnostics []Diagnostic         `json:"diagnostics"`
	Locations   map[string]Position  `json:"locations"`
}
type token struct {
	kind, value string
	Position
}
type syntaxError struct{ Diagnostic }
type parser struct {
	tokens    []token
	index     int
	locations map[string]Position
}

func Compile(input string) (result Result) {
	result = Result{Language: Version, Diagnostics: []Diagnostic{}, Locations: map[string]Position{}}
	defer func() {
		if failure := recover(); failure != nil {
			if e, ok := failure.(syntaxError); ok {
				result.Design = nil
				result.Valid = false
				result.Diagnostics = append(result.Diagnostics, e.Diagnostic)
			} else {
				panic(failure)
			}
		}
	}()
	if len(input) > MaxBytes || !utf8.ValidString(input) {
		result.Diagnostics = append(result.Diagnostics, Diagnostic{Position: Position{1, 1}, Message: "Source must be valid UTF-8 and at most 64 KiB"})
		return
	}
	p := parser{tokens: lex(input), locations: result.Locations}
	p.mark("")
	p.expect("system")
	d := system.SystemDesign{SchemaVersion: 1, ID: p.text(), Components: []system.ComponentInstance{}, Connections: []system.Connection{}, DeploymentPlans: []system.DeploymentPlan{}}
	p.expect("version")
	d.Revision = p.text()
	d.Name = d.ID
	d.Contract = system.Contract{ID: d.ID + ".contract", Revision: d.Revision, Claims: []system.Claim{}, Assumptions: []system.Assumption{}}
	p.expect("is")
	named := false
	for p.peek().kind != "begin" {
		switch p.peek().kind {
		case "name":
			if named {
				p.fail("Duplicate system name")
			}
			named = true
			p.expect("name")
			d.Name = p.text()
			p.expect(";")
		case "provider":
			p.mark(fmt.Sprintf("executionProviders[%d]", len(d.ExecutionProviders)))
			p.expect("provider")
			provider := system.ExecutionProvider{ID: p.text()}
			p.expect(":")
			provider.Kind = p.text()
			p.expect("version")
			provider.Version = p.text()
			p.expect(";")
			d.ExecutionProviders = append(d.ExecutionProviders, provider)
		default:
			p.fail("Expected name, provider or begin")
		}
	}
	p.expect("begin")
	for p.peek().kind != "end" {
		switch p.peek().kind {
		case "string":
			path := fmt.Sprintf("components[%d]", len(d.Components))
			p.mark(path)
			c := system.ComponentInstance{ID: p.text(), Artifacts: []system.ArtifactRef{}, Bindings: []system.Binding{}}
			c.Name = c.ID
			p.expect(":")
			p.expect("component")
			p.mark(path + ".definition")
			c.Definition = system.DefinitionRef{Kind: "definition", ID: p.text()}
			p.expect("version")
			c.Definition.Version = p.text()
			p.expect("is")
			p.component(&c, path)
			d.Components = append(d.Components, c)
		case "connect":
			p.mark(fmt.Sprintf("connections[%d]", len(d.Connections)))
			p.expect("connect")
			c := system.Connection{ID: p.text(), Kind: "data"}
			p.expect(":")
			c.Source, c.SourcePort = p.endpoint()
			p.expect("=>")
			c.Target, c.TargetPort = p.endpoint()
			p.expect(";")
			d.Connections = append(d.Connections, c)
		case "claim":
			p.mark(fmt.Sprintf("contract.claims[%d]", len(d.Contract.Claims)))
			p.expect("claim")
			c := system.Claim{ID: p.text()}
			p.expect(":")
			c.Kind = p.text()
			p.expect("description")
			c.Description = p.text()
			p.expect(";")
			d.Contract.Claims = append(d.Contract.Claims, c)
		case "assume":
			p.mark(fmt.Sprintf("contract.assumptions[%d]", len(d.Contract.Assumptions)))
			p.expect("assume")
			a := system.Assumption{ID: p.text()}
			p.expect(":")
			a.Description = p.text()
			p.expect(";")
			d.Contract.Assumptions = append(d.Contract.Assumptions, a)
		default:
			p.fail("Expected a component instance, connect, claim, assume or end")
		}
	}
	p.expect("end")
	p.expect("system")
	p.expect(";")
	p.expect("eof")
	checked := system.Validate(d)
	for _, issue := range checked.Issues {
		result.Diagnostics = append(result.Diagnostics, Diagnostic{Position: Locate(result.Locations, issue.Path), Path: issue.Path, Message: issue.Message})
	}
	result.Valid = checked.Valid
	if result.Valid {
		result.Design = &d
	}
	return
}

func (p *parser) component(c *system.ComponentInstance, path string) {
	named, ports := false, false
	for p.peek().kind != "end" {
		switch p.peek().kind {
		case "name":
			if named {
				p.fail("Duplicate component name")
			}
			named = true
			p.expect("name")
			c.Name = p.text()
			p.expect(";")
		case "port":
			if ports {
				p.fail("Only one port map per component")
			}
			ports = true
			p.expect("port")
			p.expect("map")
			p.expect("(")
			seen := map[string]bool{}
			for p.peek().kind != ")" {
				p.mark(fmt.Sprintf("%s.bindings[%d]", path, len(c.Bindings)))
				requirement := p.text()
				if seen[requirement] {
					p.fail("Duplicate provider requirement")
				}
				seen[requirement] = true
				p.expect("=>")
				c.Bindings = append(c.Bindings, system.Binding{Requirement: requirement, ResourceID: p.text()})
				if !p.more() {
					break
				}
			}
			p.expect(")")
			p.expect(";")
		case "entry":
			p.mark(fmt.Sprintf("%s.stateEntries[%d]", path, len(c.StateEntries)))
			p.expect("entry")
			e := system.StateEntry{Slot: p.text()}
			p.expect("=>")
			e.Command = p.text()
			p.expect(";")
			c.StateEntries = append(c.StateEntries, e)
		case "messages":
			p.mark(fmt.Sprintf("%s.messageOwners[%d]", path, len(c.MessageOwners)))
			p.expect("messages")
			o := system.MessageOwner{Slot: p.text()}
			p.expect("owner")
			o.InstanceID = p.text()
			p.expect("=>")
			o.ComponentID = p.text()
			p.expect(".")
			o.StateSlot = p.text()
			p.expect(";")
			c.MessageOwners = append(c.MessageOwners, o)
		case "workflow":
			pos := p.peek().Position
			p.expect("workflow")
			slot := p.text()
			switch p.peek().kind {
			case "owner":
				p.locations[fmt.Sprintf("%s.workflowOwners[%d]", path, len(c.WorkflowOwners))] = pos
				p.expect("owner")
				o := system.WorkflowOwnerBinding{Slot: slot, OwnerID: p.text()}
				p.expect("=>")
				o.ComponentID = p.text()
				p.expect(".")
				o.StateSlot = p.text()
				p.expect(";")
				c.WorkflowOwners = append(c.WorkflowOwners, o)
			case "handoff":
				p.locations[fmt.Sprintf("%s.workflowMessages[%d]", path, len(c.WorkflowMessages))] = pos
				p.expect("handoff")
				r := system.WorkflowMessageBinding{Slot: slot, NodeID: p.text(), Response: map[string]string{}}
				p.expect("from")
				r.SourceNodeID = p.text()
				p.expect("via")
				r.ChannelComponentID = p.text()
				p.expect(".")
				r.ChannelSlot = p.text()
				p.expect(".")
				r.RouteID = p.text()
				p.expect("response")
				p.expect("map")
				p.expect("(")
				for p.peek().kind != ")" {
					key := p.text()
					if _, ok := r.Response[key]; ok {
						p.fail("Duplicate response field")
					}
					p.expect("=>")
					r.Response[key] = p.text()
					if !p.more() {
						break
					}
				}
				p.expect(")")
				p.expect(";")
				c.WorkflowMessages = append(c.WorkflowMessages, r)
			default:
				p.fail("Expected workflow owner or handoff")
			}
		default:
			p.fail("Expected name, port map, entry, messages owner, workflow owner/handoff, or end; behavior bodies and generics are not supported")
		}
	}
	p.expect("end")
	p.expect("component")
	p.expect(";")
}
func (p *parser) endpoint() (string, *system.AspectEndpoint) {
	id := p.text()
	p.expect(".")
	e := &system.AspectEndpoint{Slot: p.text()}
	p.expect(".")
	e.Interface = p.text()
	p.expect(".")
	e.Port = p.text()
	return id, e
}
func (p *parser) more() bool {
	if p.peek().kind == "," {
		p.expect(",")
		if p.peek().kind == ")" {
			p.fail("Trailing comma is not supported")
		}
		return true
	}
	return false
}
func (p *parser) peek() token {
	if p.index >= len(p.tokens) {
		return p.tokens[len(p.tokens)-1]
	}
	return p.tokens[p.index]
}
func (p *parser) expect(kind string) {
	if p.peek().kind != kind {
		p.fail("Expected " + kind + ", found " + p.peek().kind)
	}
	p.index++
}
func (p *parser) text() string     { t := p.peek(); p.expect("string"); return t.value }
func (p *parser) mark(path string) { p.locations[path] = p.peek().Position }
func (p *parser) fail(message string) {
	panic(syntaxError{Diagnostic{Position: p.peek().Position, Message: message}})
}

// Locate maps model issue paths to the nearest source declaration. Aspect-internal
// issues without a matching declaration fall back to the system header.
func Locate(locations map[string]Position, path string) Position {
	best := locations[""]
	if best.Line == 0 {
		best = Position{1, 1}
	}
	length := -1
	for key, pos := range locations {
		if (path == key || strings.HasPrefix(path, key+".") || strings.HasPrefix(path, key+"[")) && len(key) > length {
			best = pos
			length = len(key)
		}
	}
	return best
}
func lex(input string) []token {
	runes := []rune(input)
	out := []token{}
	line, col := 1, 1
	advance := func(r rune) {
		if r == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}
	for i := 0; i < len(runes); {
		r := runes[i]
		if unicode.IsSpace(r) {
			advance(r)
			i++
			continue
		}
		if r == '-' && i+1 < len(runes) && runes[i+1] == '-' {
			for i < len(runes) && runes[i] != '\n' {
				advance(runes[i])
				i++
			}
			continue
		}
		pos := Position{line, col}
		fail := func(message string) { panic(syntaxError{Diagnostic{Position: pos, Message: message}}) }
		if len(out) >= 8192 {
			fail("Source exceeds 8192 tokens")
		}
		switch {
		case r == '"':
			start := i
			advance(r)
			i++
			closed := false
			for i < len(runes) {
				v := runes[i]
				advance(v)
				i++
				if v == '\\' {
					if i < len(runes) {
						advance(runes[i])
						i++
					}
					continue
				}
				if v == '"' {
					closed = true
					break
				}
			}
			if !closed {
				fail("Unterminated quoted string")
			}
			var value string
			if err := json.Unmarshal([]byte(string(runes[start:i])), &value); err != nil {
				fail("Quoted names use JSON string escapes")
			}
			out = append(out, token{"string", value, pos})
		case r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r == '_':
			start := i
			for i < len(runes) && ((runes[i] >= 'a' && runes[i] <= 'z') || (runes[i] >= 'A' && runes[i] <= 'Z') || runes[i] == '_') {
				advance(runes[i])
				i++
			}
			value := strings.ToLower(string(runes[start:i]))
			out = append(out, token{value, value, pos})
		case r == '=' && i+1 < len(runes) && runes[i+1] == '>':
			advance(r)
			advance('>')
			i += 2
			out = append(out, token{"=>", "=>", pos})
		case strings.ContainsRune(":;().,", r):
			advance(r)
			i++
			out = append(out, token{string(r), string(r), pos})
		default:
			fail("Unexpected character " + fmt.Sprintf("%q", r))
		}
	}
	return append(out, token{"eof", "", Position{line, col}})
}
