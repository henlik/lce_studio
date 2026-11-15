package source

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"

	"software-studio/internal/model"
	"software-studio/internal/state"
)

type StepExpectation struct {
	Status    string                      `json:"status"`
	ReplyLost bool                        `json:"replyLost"`
	Outputs   map[string]model.TypedValue `json:"outputs,omitempty"`
}
type EffectExpectation struct {
	Event     string                      `json:"event"`
	CommandID string                      `json:"commandId"`
	Payload   map[string]model.TypedValue `json:"payload"`
}
type TestCase struct {
	Scenario model.StateScenario `json:"scenario"`
	Steps    []StepExpectation   `json:"steps"`
	Effects  []EffectExpectation `json:"effects"`
}
type Testbench struct {
	Name  string     `json:"name"`
	Cases []TestCase `json:"cases"`
}
type CaseReport struct {
	Name        string       `json:"name"`
	Passed      bool         `json:"passed"`
	Assertions  int          `json:"assertions"`
	Diagnostics []Diagnostic `json:"diagnostics"`
	Result      state.Result `json:"result"`
}
type TestReport struct {
	Language    string       `json:"language"`
	Source      string       `json:"source"`
	SourceHash  string       `json:"sourceHash"`
	Compilation Result       `json:"compilation"`
	Passed      bool         `json:"passed"`
	Cases       []CaseReport `json:"cases"`
}

func (p *parser) values(ports []model.BehaviorPort) map[string]model.TypedValue {
	p.expect("(")
	values := map[string]model.TypedValue{}
	for p.peek().kind != ")" {
		key := p.name()
		if _, ok := values[key]; ok {
			p.fail("Duplicate value " + key)
		}
		p.expect("=>")
		values[key] = p.literal()
		if !p.more() {
			break
		}
	}
	p.expect(")")
	if len(values) != len(ports) {
		p.fail("Supply every declared value exactly once")
	}
	for _, port := range ports {
		value, ok := values[port.Name]
		if !ok || value.Type != port.Type {
			p.fail("Missing or incorrectly typed value " + port.Name)
		}
	}
	return values
}
func (p *parser) natural() int64 {
	v := p.literal()
	if v.Type != "integer" || *v.Integer < 0 {
		p.fail("Expected a nonnegative integer")
	}
	return *v.Integer
}
func (p *parser) testbench(pack *Component) *Testbench {
	p.mark("testbench")
	p.expect("testbench")
	tb := &Testbench{Name: p.text(), Cases: []TestCase{}}
	p.expect("for")
	if p.text() != pack.Definition.ID {
		p.fail("Testbench must target this component")
	}
	p.expect("is")
	statePorts := []model.BehaviorPort{}
	for _, s := range pack.Machine.Machine.State {
		statePorts = append(statePorts, model.BehaviorPort{Name: s.Name, Type: s.Type})
	}
	commands := map[string]model.StateCommand{}
	for _, c := range pack.Machine.Machine.Commands {
		commands[c.Name] = c
	}
	events := map[string][]model.BehaviorPort{}
	for _, e := range pack.Machine.Machine.Effects {
		events[e.Name] = e.Payload
	}
	names := map[string]bool{}
	total := 0
	for p.peek().kind != "end" {
		if len(tb.Cases) >= 16 {
			p.fail("At most 16 test cases are supported")
		}
		path := fmt.Sprintf("testbench.cases[%d]", len(tb.Cases))
		p.mark(path)
		p.expect("case")
		name := p.text()
		if names[name] {
			p.fail("Duplicate case name")
		}
		names[name] = true
		p.expect("is")
		c := TestCase{Scenario: model.StateScenario{SchemaVersion: 1, Name: name, Instances: []model.StateInstance{{ID: "subject", Name: pack.Definition.Name, Initial: map[string]model.TypedValue{}}}, Steps: []model.StateStep{}, Expected: []model.StateExpectation{}}, Steps: []StepExpectation{}, Effects: []EffectExpectation{}}
		initial, final := false, false
		for p.peek().kind != "end" {
			stepPath := fmt.Sprintf("%s.steps[%d]", path, len(c.Steps))
			p.mark(stepPath)
			switch p.peek().kind {
			case "initial":
				if initial || len(c.Steps) > 0 || final {
					p.fail("Initial values may appear once, before all steps")
				}
				initial = true
				p.index++
				c.Scenario.Instances[0].Initial = p.values(statePorts)
				p.expect(";")
			case "call":
				if final {
					p.fail("Steps must precede final expectations")
				}
				p.index++
				name := p.name()
				command, ok := commands[name]
				if !ok {
					p.fail("Unknown command " + name)
				}
				step := model.StateStep{Kind: "command", InstanceID: "subject", Command: name, Inputs: p.values(command.Inputs)}
				p.expect("id")
				step.CommandID = p.text()
				p.expect("revision")
				revision := p.natural()
				step.ExpectedRevision = &revision
				if p.peek().kind == "fault" {
					p.index++
					step.Fault = p.name()
				}
				p.expect("expect")
				expected := StepExpectation{Status: p.name()}
				switch expected.Status {
				case "committed", "denied", "replayed", "id_conflict", "revision_conflict", "rolled_back", "evaluation_error":
				default:
					p.fail("Unknown expected command status")
				}
				if p.peek().kind == "reply_lost" {
					p.index++
					expected.ReplyLost = true
				}
				if p.peek().kind == "outputs" {
					p.index++
					expected.Outputs = p.values(command.Outputs)
				}
				p.expect(";")
				c.Scenario.Steps = append(c.Scenario.Steps, step)
				c.Steps = append(c.Steps, expected)
			case "restart":
				if final {
					p.fail("Steps must precede final expectations")
				}
				p.index++
				p.expect(";")
				c.Scenario.Steps = append(c.Scenario.Steps, model.StateStep{Kind: "restart", InstanceID: "subject"})
				c.Steps = append(c.Steps, StepExpectation{Status: "restarted"})
			case "expect":
				p.index++
				if p.peek().kind == "state" {
					if final {
						p.fail("Exactly one final state expectation is required")
					}
					final = true
					p.mark(path + ".final")
					p.index++
					e := model.StateExpectation{InstanceID: "subject", State: p.values(statePorts)}
					p.expect("revision")
					e.Revision = p.natural()
					p.expect("receipts")
					e.Receipts = int(p.natural())
					p.expect("effects")
					e.Effects = int(p.natural())
					p.expect(";")
					c.Scenario.Expected = append(c.Scenario.Expected, e)
				} else {
					p.mark(fmt.Sprintf("%s.effects[%d]", path, len(c.Effects)))
					p.expect("event")
					name := p.name()
					ports, ok := events[name]
					if !ok {
						p.fail("Unknown event " + name)
					}
					e := EffectExpectation{Event: name, Payload: p.values(ports)}
					p.expect("from")
					e.CommandID = p.text()
					p.expect(";")
					c.Effects = append(c.Effects, e)
					if len(c.Effects) > 64 {
						p.fail("At most 64 event expectations are supported")
					}
				}
			default:
				p.fail("Expected initial, call, restart, expect or end case")
			}
			if len(c.Steps) > 64 {
				p.fail("At most 64 steps per case are supported")
			}
		}
		p.expect("end")
		p.expect("case")
		p.expect(";")
		if !final {
			p.fail("Each case requires independent final state, revision, receipt and effect expectations")
		}
		total += len(c.Steps)
		if total > 256 {
			p.fail("At most 256 steps per testbench are supported")
		}
		p.issues(path, model.ValidateStateScenario(pack.Machine, c.Scenario))
		tb.Cases = append(tb.Cases, c)
	}
	p.expect("end")
	p.expect("testbench")
	p.expect(";")
	if len(tb.Cases) == 0 {
		p.fail("A testbench requires at least one case")
	}
	return tb
}

// RunTestbench recompiles the source and runs each case in a fresh existing
// state engine. Expectations are supplied by the author, never learned from a run.
// It performs no I/O and does not install artifacts or modify a canvas.
func RunTestbench(ctx context.Context, input string) (TestReport, error) {
	if err := ctx.Err(); err != nil {
		return TestReport{}, err
	}
	compiled := Compile(input)
	if !compiled.Valid || compiled.Component == nil || compiled.Testbench == nil {
		return TestReport{}, fmt.Errorf("a valid complete component and testbench are required")
	}
	hash := sha256.Sum256([]byte(input))
	report := TestReport{Language: Version, Source: input, SourceHash: hex.EncodeToString(hash[:]), Compilation: compiled, Passed: true, Cases: []CaseReport{}}
	bytes := 0
	for i, c := range compiled.Testbench.Cases {
		result, err := state.Run(ctx, compiled.Component.Machine, c.Scenario)
		if err != nil {
			return TestReport{}, err
		}
		cr := CaseReport{Name: c.Scenario.Name, Passed: result.Verified && result.Passed, Result: result, Diagnostics: []Diagnostic{}}
		path := fmt.Sprintf("testbench.cases[%d]", i)
		check := func(ok bool, at, message string) {
			cr.Assertions++
			if !ok {
				cr.Passed = false
				cr.Diagnostics = append(cr.Diagnostics, Diagnostic{Position: Locate(compiled.Locations, at), Path: at, Message: message})
			}
		}
		for j, expected := range c.Steps {
			actual := result.Steps[j]
			at := fmt.Sprintf("%s.steps[%d]", path, j)
			check(actual.Status == expected.Status, at, fmt.Sprintf("Expected status %s, got %s", expected.Status, actual.Status))
			check(actual.ReplyLost == expected.ReplyLost, at, fmt.Sprintf("Expected reply_lost=%t, got %t", expected.ReplyLost, actual.ReplyLost))
			if expected.Outputs != nil {
				check(actual.Receipt != nil && reflect.DeepEqual(expected.Outputs, actual.Receipt.Outputs), at, "Command receipt outputs differ from the independent expectation")
			}
		}
		for j, expected := range c.Effects {
			matches := 0
			for _, effect := range result.Instances[0].Effects {
				if effect.CommandID == expected.CommandID && effect.Effect == expected.Event && reflect.DeepEqual(effect.Payload, expected.Payload) {
					matches++
				}
			}
			check(matches == 1, fmt.Sprintf("%s.effects[%d]", path, j), "Expected exactly one matching committed event intent and payload")
		}
		for _, e := range result.Expectations {
			check(e.Passed, path+".final", "Final state, revision, receipts or effects differ from the independent expectation")
		}
		report.Passed = report.Passed && cr.Passed
		report.Cases = append(report.Cases, cr)
		encoded, err := json.Marshal(cr)
		if err != nil {
			return TestReport{}, err
		}
		bytes += len(encoded)
		if bytes > 8<<20 {
			return TestReport{}, fmt.Errorf("testbench evidence exceeds 8 MiB")
		}
	}
	return report, ctx.Err()
}
