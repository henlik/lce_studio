package source

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"software-studio/internal/model"
	"software-studio/internal/system"
)

// Component is a closed artifact pack. All decisions use existing primitives;
// the frontend introduces no evaluator, external effect or catalog lookup.
type Component struct {
	Data       model.DataContractLock     `json:"data"`
	Packages   []model.ComponentPackage   `json:"packages"`
	Machine    model.StateMachineLock     `json:"machine"`
	Definition system.ComponentDefinition `json:"definition"`
}

var localName = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{0,47}$`)

func (p *parser) name() string {
	t := p.peek()
	if !localName.MatchString(t.value) || (t.kind == "number" || t.kind == "quoted") {
		p.fail("Expected a local name: a letter followed by letters, digits or underscores (maximum 48 characters)")
	}
	p.index++
	return t.value
}
func (p *parser) scalar() string {
	t := p.peek().kind
	if t != "integer" && t != "boolean" && t != "string" {
		p.fail("Expected integer, boolean or string")
	}
	p.index++
	return t
}
func (p *parser) literal() model.TypedValue {
	t := p.peek()
	switch t.kind {
	case "true", "false":
		p.index++
		v := t.kind == "true"
		return model.TypedValue{Type: "boolean", Boolean: &v}
	case "number", "-":
		sign := int64(1)
		if t.kind == "-" {
			sign = -1
			p.index++
		}
		t = p.peek()
		p.expect("number")
		v, err := strconv.ParseInt(t.value, 10, 64)
		if err != nil || v > model.MaxBehaviorInteger {
			p.fail("Integer exceeds the exact supported range ±9007199254740991")
		}
		v *= sign
		return model.TypedValue{Type: "integer", Integer: &v}
	case "quoted":
		v := p.text()
		return model.TypedValue{Type: "string", String: &v}
	default:
		p.fail("Expected a scalar literal")
	}
	return model.TypedValue{}
}
func (p *parser) ports() []model.BehaviorPort {
	p.expect("(")
	ports := []model.BehaviorPort{}
	seen := map[string]bool{}
	for p.peek().kind != ")" {
		name := p.name()
		if seen[name] {
			p.fail("Duplicate port " + name)
		}
		seen[name] = true
		p.expect(":")
		ports = append(ports, model.BehaviorPort{Name: name, Type: p.scalar()})
		if len(ports) > 8 {
			p.fail("At most eight public ports are supported")
		}
		if !p.more() {
			break
		}
	}
	p.expect(")")
	return ports
}
func (p *parser) issues(path string, issues []model.ValidationIssue) {
	if len(issues) > 0 {
		at := path
		if issues[0].Path != "" {
			at += "." + issues[0].Path
		}
		panic(syntaxError{Diagnostic{Position: Locate(p.locations, at), Path: at, Message: issues[0].Message}})
	}
}

func (p *parser) compileComponent(r *Result) {
	p.mark("component")
	p.expect("component")
	id := p.text()
	p.expect("version")
	version := p.text()
	p.expect("is")
	m := model.StateMachine{SchemaVersion: 1, ID: id + ".state", Version: version, Name: id, Author: "LCE source author", License: "UNLICENSED", DataContracts: []model.DataContractLock{}, State: []model.StateSlot{}, Effects: []model.StateEffect{}, Commands: []model.StateCommand{}}
	pack := &Component{Packages: []model.ComponentPackage{}}
	names := map[string]bool{}
	for p.peek().kind != "end" {
		switch p.peek().kind {
		case "name", "description", "author", "license":
			if len(m.Commands) > 0 {
				p.fail("Declare metadata before commands")
			}
			key := p.peek().kind
			if names[key] {
				p.fail("Duplicate " + key)
			}
			names[key] = true
			p.index++
			value := p.text()
			p.expect(";")
			switch key {
			case "name":
				m.Name = value
			case "description":
				m.Description = value
			case "author":
				m.Author = value
			case "license":
				m.License = value
			}
		case "state":
			if len(m.Commands) > 0 {
				p.fail("Declare state and events before commands")
			}
			p.mark(fmt.Sprintf("component.machine.state[%d]", len(m.State)))
			p.expect("state")
			s := model.StateSlot{Name: p.name()}
			p.expect(":")
			s.Type = p.scalar()
			p.expect(":=")
			s.Initial = p.literal()
			if p.peek().kind != "durable" && p.peek().kind != "volatile" {
				p.fail("Declare durable or volatile state explicitly")
			}
			s.Durability = p.peek().kind
			p.index++
			p.expect(";")
			m.State = append(m.State, s)
			if len(m.State) > 8 {
				p.fail("At most eight state slots are supported")
			}
		case "event":
			if len(m.Commands) > 0 {
				p.fail("Declare state and events before commands")
			}
			p.mark(fmt.Sprintf("component.machine.effects[%d]", len(m.Effects)))
			p.expect("event")
			e := model.StateEffect{Name: p.name(), Payload: p.ports()}
			p.expect(";")
			m.Effects = append(m.Effects, e)
			if len(m.Effects) > 8 {
				p.fail("At most eight events are supported")
			}
		case "command":
			p.mark(fmt.Sprintf("component.machine.commands[%d]", len(m.Commands)))
			command, pkg := p.command(m, id)
			m.Commands = append(m.Commands, command)
			pack.Packages = append(pack.Packages, pkg)
			if len(m.Commands) > 8 {
				p.fail("At most eight commands are supported")
			}
		default:
			p.fail("Expected metadata, state, event, command or end component")
		}
	}
	p.expect("end")
	p.expect("component")
	p.expect(";")
	p.issues("component.machine", model.ValidateStateMachine(m))
	data := model.DataContract{SchemaVersion: 1, ID: id + ".data", Version: version, Name: m.Name, Description: "Generated scalar state, command and event schemas.", Author: m.Author, License: m.License, Records: []model.DataRecord{}, Entities: []model.DataEntity{}, Relationships: []model.DataRelationship{}}
	addRecord := func(name string, ports []model.BehaviorPort) {
		if len(ports) == 0 {
			return
		}
		fields := []model.DataField{}
		for _, port := range ports {
			fields = append(fields, model.DataField{Name: port.Name, Type: port.Type})
		}
		data.Records = append(data.Records, model.DataRecord{Name: name, Fields: fields})
	}
	statePorts := []model.BehaviorPort{}
	for _, s := range m.State {
		statePorts = append(statePorts, model.BehaviorPort{Name: s.Name, Type: s.Type})
	}
	addRecord("OwnedState", statePorts)
	for _, c := range m.Commands {
		addRecord(c.Name+"Inputs", c.Inputs)
		addRecord(c.Name+"Outputs", c.Outputs)
	}
	for _, e := range m.Effects {
		addRecord(e.Name+"Event", e.Payload)
	}
	p.issues("component.data", model.ValidateDataContract(data))
	dataDigest, err := model.DataContractDigest(data)
	if err != nil {
		p.fail(err.Error())
	}
	pack.Data = model.DataContractLock{Contract: data, Digest: dataDigest}
	m.DataContracts = []model.DataContractLock{pack.Data}
	p.issues("component.machine", model.ValidateStateMachine(m))
	digest, err := model.StateMachineDigest(m)
	if err != nil {
		p.fail(err.Error())
	}
	pack.Machine = model.StateMachineLock{Machine: m, Digest: digest}
	pack.Definition = system.ComponentDefinition{SchemaVersion: 1, ID: id, Version: version, Name: m.Name, Description: m.Description, Aspects: []system.AspectRef{
		{Slot: "data", Kind: "data", ID: data.ID, Version: version, Digest: dataDigest},
		{Slot: "owner", Kind: "state", ID: m.ID, Version: version, Digest: digest},
	}}
	p.issues("component.definition", system.ValidateDefinition(pack.Definition).Issues)
	d := system.SystemDesign{SchemaVersion: 1, ID: id + ".design", Name: m.Name, Revision: version, Connections: []system.Connection{}, DeploymentPlans: []system.DeploymentPlan{}, ExecutionProviders: []system.ExecutionProvider{{ID: "state-model", Kind: "modeled-state", Version: "1.0.0"}}, Contract: system.Contract{ID: id + ".contract", Revision: version, Claims: []system.Claim{{ID: "evidence", Kind: "required_evidence", Description: "Testbench assertions are checked separately; model runs do not evaluate system contract claims."}}, Assumptions: []system.Assumption{}}, Components: []system.ComponentInstance{{ID: "subject", Name: m.Name, Definition: system.DefinitionRef{Kind: "definition", ID: id, Version: version}, Artifacts: []system.ArtifactRef{}, Bindings: []system.Binding{{Requirement: "state/owner", ResourceID: "state-model"}}, StateEntries: []system.StateEntry{{Slot: "owner", Command: m.Commands[0].Name}}}}}
	p.issues("design", system.Validate(d).Issues)
	if p.peek().kind == "testbench" {
		r.Testbench = p.testbench(pack)
	}
	p.expect("eof")
	r.Component = pack
	r.Design = &d
	r.Valid = true
}

type expression struct {
	binding model.BehaviorBinding
	typ     string
}
type commandBuilder struct {
	p        *parser
	behavior model.Behavior
	vars     map[string]expression
	command  model.StateCommand
	depth    int
}

func (b *commandBuilder) node(op string, args map[string]expression) expression {
	types := map[string]string{}
	inputs := map[string]model.BehaviorBinding{}
	for k, v := range args {
		types[k] = v.typ
		inputs[k] = v.binding
	}
	typ, err := model.PrimitiveOutputType(op, types)
	if err != nil {
		b.p.fail(err.Error())
	}
	if len(b.behavior.Nodes) >= 32 {
		b.p.fail("A command may expand to at most 32 primitive operations")
	}
	id := fmt.Sprintf("n%d", len(b.behavior.Nodes)+1)
	b.behavior.Nodes = append(b.behavior.Nodes, model.BehaviorNode{ID: id, Op: op, Inputs: inputs, Position: model.Position{X: float64(len(b.behavior.Nodes)%4) * 260, Y: float64(len(b.behavior.Nodes)/4) * 120}})
	return expression{model.BehaviorBinding{Node: id, Port: "result"}, typ}
}
func (b *commandBuilder) output(name string, value expression) string {
	b.behavior.Outputs = append(b.behavior.Outputs, model.BehaviorPort{Name: name, Type: value.typ})
	b.behavior.Bindings[name] = value.binding
	return name
}
func (p *parser) command(m model.StateMachine, id string) (model.StateCommand, model.ComponentPackage) {
	p.expect("command")
	name := p.name()
	inputs := p.ports()
	p.expect("returns")
	outputs := p.ports()
	p.expect("is")
	b := commandBuilder{p: p, vars: map[string]expression{}, behavior: model.Behavior{Inputs: []model.BehaviorPort{}, Outputs: []model.BehaviorPort{}, Nodes: []model.BehaviorNode{}, Bindings: map[string]model.BehaviorBinding{}}, command: model.StateCommand{Name: name, Inputs: inputs, Outputs: append([]model.BehaviorPort{{Name: "accepted", Type: "boolean"}}, outputs...), Bindings: map[string]model.StateBinding{}, Writes: map[string]string{}, Results: map[string]string{}, Effects: []model.StateEffectBinding{}}}
	for _, s := range m.State {
		key := "state_" + s.Name
		b.behavior.Inputs = append(b.behavior.Inputs, model.BehaviorPort{Name: key, Type: s.Type})
		b.command.Bindings[key] = model.StateBinding{State: s.Name}
		b.vars["state."+s.Name] = expression{model.BehaviorBinding{Input: key}, s.Type}
	}
	for _, port := range inputs {
		if reservedValueName(port.Name) {
			p.fail("Input name is reserved: " + port.Name)
		}
		key := "input_" + port.Name
		b.behavior.Inputs = append(b.behavior.Inputs, model.BehaviorPort{Name: key, Type: port.Type})
		b.command.Bindings[key] = model.StateBinding{Input: port.Name}
		b.vars[port.Name] = expression{model.BehaviorBinding{Input: key}, port.Type}
	}
	p.expect("accept")
	p.expect("when")
	accepted := b.expr(0)
	p.expect(";")
	if accepted.typ != "boolean" {
		p.fail("Acceptance condition must be boolean")
	}
	b.vars["accepted"] = accepted
	b.command.AcceptedOutput = b.output("accepted", accepted)
	b.command.Results["accepted"] = "accepted"
	for p.peek().kind != "end" {
		switch p.peek().kind {
		case "let":
			p.index++
			key := p.name()
			if _, ok := b.vars[key]; ok || reservedValueName(key) {
				p.fail("Duplicate or reserved local name " + key)
			}
			p.expect(":=")
			b.vars[key] = b.expr(0)
			p.expect(";")
		case "set":
			p.index++
			key := p.name()
			old, ok := b.vars["state."+key]
			if !ok {
				p.fail("Unknown state slot " + key)
			}
			if _, ok := b.command.Writes[key]; ok {
				p.fail("State may be assigned only once per command")
			}
			p.expect(":=")
			value := b.expr(0)
			p.expect(";")
			if value.typ != old.typ {
				p.fail("State assignment type mismatch for " + key)
			}
			next := b.node("select", map[string]expression{"condition": accepted, "trueValue": value, "falseValue": old})
			b.vars["next."+key] = next
			b.command.Writes[key] = b.output("write_"+key, next)
		case "return":
			p.index++
			key := p.name()
			typ := ""
			for _, port := range outputs {
				if port.Name == key {
					typ = port.Type
				}
			}
			if typ == "" {
				p.fail("Unknown return port " + key)
			}
			if _, ok := b.command.Results[key]; ok {
				p.fail("Duplicate return " + key)
			}
			p.expect(":=")
			value := b.expr(0)
			p.expect(";")
			if value.typ != typ {
				p.fail("Return type mismatch for " + key)
			}
			b.command.Results[key] = b.output("result_"+key, value)
		case "emit":
			p.index++
			key := p.name()
			var ports []model.BehaviorPort
			for _, e := range m.Effects {
				if e.Name == key {
					ports = e.Payload
				}
			}
			if ports == nil {
				p.fail("Unknown event " + key)
			}
			for _, e := range b.command.Effects {
				if e.Effect == key {
					p.fail("Each event may be emitted once per command")
				}
			}
			values := b.arguments(ports)
			p.expect(";")
			payload := map[string]string{}
			for _, port := range ports {
				payload[port.Name] = b.output("event_"+strconv.Itoa(len(b.command.Effects))+"_"+port.Name, values[port.Name])
			}
			b.command.Effects = append(b.command.Effects, model.StateEffectBinding{Effect: key, WhenOutput: "accepted", Payload: payload})
		default:
			p.fail("Expected let, set, return, emit or end command")
		}
	}
	p.expect("end")
	p.expect("command")
	p.expect(";")
	for _, port := range outputs {
		if port.Name == "accepted" {
			p.fail("accepted is an automatically generated boolean output")
		}
		if _, ok := b.command.Results[port.Name]; !ok {
			p.fail("Missing return for " + port.Name)
		}
	}
	pkg := model.ComponentPackage{SchemaVersion: 3, ID: id + "." + name + ".decision", Version: m.Version, Name: name, Author: m.Author, License: m.License, Kernel: "composite", Capabilities: []string{"design", "evaluate"}, DataContracts: []model.DataContractLock{}, Behavior: &b.behavior}
	bundle, issues := model.PrepareComponentBundle(pkg, []model.PackageLock{})
	p.issues(fmt.Sprintf("component.machine.commands[%d]", len(m.Commands)), issues)
	b.command.Decision = bundle
	return b.command, pkg
}
func (b *commandBuilder) arguments(ports []model.BehaviorPort) map[string]expression {
	p := b.p
	values := map[string]expression{}
	p.expect("(")
	for p.peek().kind != ")" {
		key := p.name()
		if _, ok := values[key]; ok {
			p.fail("Duplicate argument " + key)
		}
		p.expect("=>")
		values[key] = b.expr(0)
		if !p.more() {
			break
		}
	}
	p.expect(")")
	if len(values) != len(ports) {
		p.fail("Supply every declared argument exactly once")
	}
	for _, port := range ports {
		v, ok := values[port.Name]
		if !ok || v.typ != port.Type {
			p.fail("Missing or incorrectly typed argument " + port.Name)
		}
	}
	return values
}
func precedence(op string) int {
	switch op {
	case "or":
		return 1
	case "and":
		return 2
	case "=", "<", "<=", ">", ">=":
		return 3
	case "+", "-":
		return 4
	}
	return -1
}
func (b *commandBuilder) expr(min int) expression {
	b.depth++
	defer func() { b.depth-- }()
	if b.depth > 32 {
		b.p.fail("Expression nesting exceeds 32")
	}
	p := b.p
	t := p.peek()
	var left expression
	switch t.kind {
	case "number", "-", "true", "false", "quoted":
		v := p.literal()
		left = expression{model.BehaviorBinding{Literal: &v}, v.Type}
	case "(":
		p.index++
		left = b.expr(0)
		p.expect(")")
	case "not":
		p.index++
		left = b.node("not", map[string]expression{"value": b.expr(5)})
	case "select":
		p.index++
		p.expect("(")
		condition := b.expr(0)
		p.expect(",")
		yes := b.expr(0)
		p.expect(",")
		no := b.expr(0)
		p.expect(")")
		left = b.node("select", map[string]expression{"condition": condition, "trueValue": yes, "falseValue": no})
	default:
		key := p.name()
		if key == "state" || key == "next" {
			p.expect(".")
			key += "." + p.name()
		}
		var ok bool
		left, ok = b.vars[key]
		if !ok {
			p.fail("Unknown or not-yet-defined value " + key)
		}
	}
	for precedence(p.peek().kind) >= min {
		op := p.peek().kind
		priority := precedence(op)
		p.index++
		right := b.expr(priority + 1)
		if op == ">" || op == ">=" {
			left, right = right, left
		}
		primitive := map[string]string{"+": "add", "-": "subtract", "=": "equal", "<": "less_than", "<=": "less_or_equal", ">": "less_than", ">=": "less_or_equal", "and": "and", "or": "or"}[op]
		left = b.node(primitive, map[string]expression{"left": left, "right": right})
	}
	return left
}

func reservedValueName(name string) bool {
	switch strings.ToLower(name) {
	case "accepted", "state", "next", "select", "not", "true", "false", "and", "or":
		return true
	}
	return false
}
