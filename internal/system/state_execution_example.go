package system

import "software-studio/internal/model"

type StateExecutionExamplePack struct {
	Design     SystemDesign             `json:"design"`
	Machine    model.StateMachineLock   `json:"machine"`
	Packages   []model.ComponentPackage `json:"packages"`
	Definition ComponentDefinition      `json:"definition"`
}

// StateExecutionExample is a synthetic stock model, not pharmacy software.
func StateExecutionExample() StateExecutionExamplePack {
	m := model.ExampleStateScenarios()[0].Machine.Machine
	m.ID = "studio.example-stock-owner"
	m.Name = "Synthetic stock owner"
	five := int64(5)
	for i := range m.State {
		if m.State[i].Name == "available" {
			m.State[i].Initial = model.TypedValue{Type: "integer", Integer: &five}
		}
	}
	digest, err := model.StateMachineDigest(m)
	if err != nil {
		panic(err)
	}
	pass := func(id, name string, port model.BehaviorPort) model.ComponentPackage {
		return model.ComponentPackage{SchemaVersion: 3, ID: id, Version: "1.0.0", Name: name, Description: "Synthetic pass-through for the connected state execution exercise", Author: "LCE Studio", License: "MIT", Kernel: "composite", Capabilities: []string{"design", "evaluate"}, DataContracts: m.DataContracts, Behavior: &model.Behavior{Inputs: []model.BehaviorPort{port}, Outputs: []model.BehaviorPort{port}, Nodes: []model.BehaviorNode{}, Bindings: map[string]model.BehaviorBinding{port.Name: {Input: port.Name}}}}
	}
	packages := []model.ComponentPackage{pass("studio.example-reservation-request", "Reservation request", m.Commands[0].Inputs[0]), pass("studio.example-stock-summary", "Stock summary", model.BehaviorPort{Name: "remaining", Type: "integer"})}
	def := ComponentDefinition{SchemaVersion: 1, ID: "studio.example-stock", Version: "1.0.0", Name: "Stock component", Aspects: []AspectRef{{Slot: "stock", Kind: "state", ID: m.ID, Version: m.Version, Digest: digest}}}
	d := SystemDesign{SchemaVersion: 1, ID: "state-handoff-example", Name: "Request → stock owner → summary", Revision: "1.0.0", DeploymentPlans: []DeploymentPlan{}, ExecutionProviders: []ExecutionProvider{{ID: "stock-model", Kind: "modeled-state", Version: "1.0.0"}}, Contract: Contract{ID: "state-handoff", Revision: "1", Claims: []Claim{{ID: "evidence", Kind: "required_evidence", Description: "Declared only; model runner does not evaluate claims"}}, Assumptions: []Assumption{}}, Components: []ComponentInstance{
		{ID: "request", Name: "Request", Definition: DefinitionRef{ID: packages[0].ID, Version: "1.0.0"}, Artifacts: []ArtifactRef{}, Bindings: []Binding{}},
		{ID: "stock", Name: "Stock owner", Definition: DefinitionRef{Kind: "definition", ID: def.ID, Version: def.Version}, Artifacts: []ArtifactRef{}, Bindings: []Binding{{Requirement: "state/stock", ResourceID: "stock-model"}}},
		{ID: "summary", Name: "Summary", Definition: DefinitionRef{ID: packages[1].ID, Version: "1.0.0"}, Artifacts: []ArtifactRef{}, Bindings: []Binding{}},
	}, Connections: []Connection{
		{ID: "request-stock", Source: "request", Target: "stock", Kind: "data", SourcePort: &AspectEndpoint{Slot: "root", Interface: "outputs", Port: "request"}, TargetPort: &AspectEndpoint{Slot: "stock", Interface: "command/reserve/inputs", Port: "request"}},
		{ID: "stock-summary", Source: "stock", Target: "summary", Kind: "data", SourcePort: &AspectEndpoint{Slot: "stock", Interface: "command/reserve/outputs", Port: "remaining"}, TargetPort: &AspectEndpoint{Slot: "root", Interface: "inputs", Port: "remaining"}},
	}}
	return StateExecutionExamplePack{Design: d, Machine: model.StateMachineLock{Machine: m, Digest: digest}, Packages: packages, Definition: def}
}
