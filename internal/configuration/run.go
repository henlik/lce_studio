package configuration

import (
	"context"
	"encoding/json"
	"fmt"
	"software-studio/internal/assembly"
	"software-studio/internal/behavior"
	"software-studio/internal/delivery"
	"software-studio/internal/model"
	"software-studio/internal/simulation"
	"software-studio/internal/state"
	"software-studio/internal/system"
	"software-studio/internal/systemrun"
	"software-studio/internal/workflow"
)

type Assertion struct {
	Path     string `json:"path"`
	Expected any    `json:"expected"`
	Actual   any    `json:"actual"`
	Observed bool   `json:"observed"`
	Passed   bool   `json:"passed"`
}
type CaseResult struct {
	Name         string          `json:"name"`
	Target       Target          `json:"target"`
	ScenarioHash string          `json:"scenarioHash"`
	Passed       bool            `json:"passed"`
	Error        string          `json:"error,omitempty"`
	Assertions   []Assertion     `json:"assertions"`
	Result       json.RawMessage `json:"result,omitempty"`
}
type Report struct {
	Compiler   string       `json:"compiler"`
	SourceHash string       `json:"sourceHash"`
	Lock       Lockfile     `json:"lock"`
	Passed     bool         `json:"passed"`
	Cases      []CaseResult `json:"cases"`
}

func Run(ctx context.Context, p Project) (Report, error) {
	c := Compile(p)
	r := Report{Compiler: CompilerVersion, SourceHash: c.SourceHash, Lock: c.Lock, Cases: []CaseResult{}}
	if !c.Valid {
		return r, fmt.Errorf("configuration has %d diagnostic(s): %s", len(c.Diagnostics), c.Diagnostics[0].Message)
	}
	if len(c.Experiments) == 0 {
		return r, fmt.Errorf("at least one experiment with independent expectations is required")
	}
	r.Passed = true
	totalBytes := 0
	for _, e := range c.Experiments {
		if err := ctx.Err(); err != nil {
			return Report{}, err
		}
		cr := CaseResult{Name: e.Name, Target: e.Target, ScenarioHash: Hash(e.Scenario), Assertions: []Assertion{}}
		result, err := execute(ctx, c.Artifacts, e)
		if err != nil {
			cr.Error = err.Error()
			r.Passed = false
			r.Cases = append(r.Cases, cr)
			continue
		}
		cr.Result, _ = json.Marshal(result)
		totalBytes += len(cr.Result)
		if len(cr.Result) > 8<<20 || totalBytes > 24<<20 {
			return Report{}, fmt.Errorf("experiment result exceeds 8 MiB")
		}
		actual := decodeAny(cr.Result)
		cr.Passed = true
		for _, path := range sortedKeys(e.Expect) {
			v, observed := lookupPath(actual, path)
			pass := observed && expectationMatches(v, e.Expect[path])
			cr.Assertions = append(cr.Assertions, Assertion{path, e.Expect[path], v, observed, pass})
			cr.Passed = cr.Passed && pass
		}
		r.Passed = r.Passed && cr.Passed
		r.Cases = append(r.Cases, cr)
	}
	return r, nil
}
func execute(ctx context.Context, artifacts []Artifact, e Experiment) (any, error) {
	l := Library{artifacts}
	a, err := l.find(e.Target.Kind, e.Target.ID, e.Target.Version, e.Target.Digest)
	if err != nil {
		return nil, err
	}
	decode := func(out any) error { return strictDecode(decodeAny(e.Scenario), out) }
	switch a.Kind {
	case "data":
		var d model.DataContract
		_ = json.Unmarshal(a.Body, &d)
		var s model.DataFixture
		if err = decode(&s); err != nil {
			return nil, err
		}
		return model.CheckDataFixture(model.DataContractLock{Contract: d, Digest: a.Digest}, s)
	case "behavior":
		b, err := Bundle(artifacts, a)
		if err != nil {
			return nil, err
		}
		var s struct {
			Tests  bool                        `json:"tests,omitempty"`
			Inputs map[string]model.TypedValue `json:"inputs,omitempty"`
		}
		if err = decode(&s); err != nil {
			return nil, err
		}
		if s.Tests {
			if s.Inputs != nil {
				return nil, fmt.Errorf("tests and inputs are mutually exclusive")
			}
			return behavior.Test(ctx, b)
		}
		return behavior.Evaluate(ctx, b, s.Inputs)
	case "state":
		lock, _ := l.StateMachine(ctx, a.ID, a.Version, a.Digest)
		var s model.StateScenario
		if err = decode(&s); err != nil {
			return nil, err
		}
		return state.Run(ctx, lock, s)
	case "messages":
		lock, _ := l.DeliveryModel(ctx, a.ID, a.Version, a.Digest)
		var s model.DeliveryScenario
		if err = decode(&s); err != nil {
			return nil, err
		}
		return delivery.Run(ctx, lock, s)
	case "workflow":
		lock, _ := l.WorkflowPackage(ctx, a.ID, a.Version, a.Digest)
		var s model.WorkflowScenario
		if err = decode(&s); err != nil {
			return nil, err
		}
		return workflow.Run(ctx, lock, s)
	case "system":
		var d system.SystemDesign
		_ = json.Unmarshal(a.Body, &d)
		p, err := assembly.Prepare(ctx, l, d)
		if err != nil {
			return nil, err
		}
		var s systemrun.Scenario
		if err = decode(&s); err != nil {
			return nil, err
		}
		return systemrun.RunScenario(ctx, p.Plan, s)
	case "timing":
		var empty struct{}
		if err = decode(&empty); err != nil {
			return nil, fmt.Errorf("timing scenario must be {}; configure workload/faults on the timing artifact: %w", err)
		}
		var d model.Project
		_ = json.Unmarshal(a.Body, &d)
		r, err := simulation.Run(ctx, d)
		r.ID = ""
		r.CreatedAt = ""
		return r, err
	}
	return nil, fmt.Errorf("artifact kind %s has no execution engine", a.Kind)
}

// $matches counts array entries containing exact selected fields. Missing fields
// never count as null, and additional entries are constrained by explicit counts.
func expectationMatches(actual, expected any) bool {
	m, ok := expected.(map[string]any)
	if !ok || m["$matches"] == nil {
		return Hash(actual) == Hash(expected)
	}
	if len(m) != 1 {
		return false
	}
	checks, ok := m["$matches"].([]any)
	if !ok || len(checks) == 0 || len(checks) > 64 {
		return false
	}
	items, ok := actual.([]any)
	if !ok {
		return false
	}
	for _, v := range checks {
		check, ok := v.(map[string]any)
		if !ok || len(check) != 2 {
			return false
		}
		fields, ok := check["value"].(map[string]any)
		if !ok || len(fields) == 0 {
			return false
		}
		count := 0
		for _, item := range items {
			obj, ok := item.(map[string]any)
			if !ok {
				continue
			}
			matches := true
			for key, value := range fields {
				a, exists := obj[key]
				matches = matches && exists && Hash(a) == Hash(value)
			}
			if matches {
				count++
			}
		}
		if Hash(count) != Hash(check["count"]) {
			return false
		}
	}
	return true
}
