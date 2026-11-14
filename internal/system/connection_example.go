package system

import "software-studio/internal/model"

type ConnectionExamplePack struct {
	Design   SystemDesign             `json:"design"`
	Mismatch SystemDesign             `json:"mismatch"`
	Packages []model.ComponentPackage `json:"packages"`
}

// ConnectionExample is a synthetic contract exercise, not clinical software.
// Reception and consultation are reusable record pass-through boundaries.
func ConnectionExample() ConnectionExamplePack {
	makePackage := func(id, name, version, dataVersion string) model.ComponentPackage {
		contract := model.DataContract{SchemaVersion: 1, ID: "example.encounter-contract", Version: dataVersion, Name: "Synthetic encounter", Description: "Synthetic encounter boundary for static connection checks", Author: "LCE Studio", License: "MIT", Records: []model.DataRecord{{Name: "Encounter", Fields: []model.DataField{{Name: "encounterId", Type: "string"}, {Name: "patientId", Type: "string"}}}}, Entities: []model.DataEntity{}, Relationships: []model.DataRelationship{}}
		digest, _ := model.DataContractDigest(contract)
		ref := &model.DataTypeRef{ContractID: contract.ID, Version: contract.Version, Digest: digest, Record: "Encounter"}
		return model.ComponentPackage{SchemaVersion: 3, ID: id, Version: version, Name: name, Description: "Record pass-through boundary; no clinical decisions or joint execution", Author: "LCE Studio", License: "MIT", Kernel: "composite", Capabilities: []string{"design", "evaluate"}, DataContracts: []model.DataContractLock{{Contract: contract, Digest: digest}}, Behavior: &model.Behavior{Inputs: []model.BehaviorPort{{Name: "encounter", Type: "record", RecordRef: ref}}, Outputs: []model.BehaviorPort{{Name: "encounter", Type: "record", RecordRef: ref}}, Nodes: []model.BehaviorNode{}, Bindings: map[string]model.BehaviorBinding{"encounter": {Input: "encounter"}}}}
	}
	packages := []model.ComponentPackage{makePackage("example.reception-boundary", "Reception boundary", "1.0.0", "1.0.0"), makePackage("example.consultation-boundary", "Consultation boundary", "1.0.0", "1.0.0"), makePackage("example.consultation-boundary", "Consultation boundary v2", "2.0.0", "2.0.0")}
	d := ReceptionExample()
	d.ID = "reception-consultation-connections"
	d.Name = "Reception → Consultation · static compatibility"
	d.Revision = "1.0.0"
	d.DeploymentPlans = []DeploymentPlan{}
	d.Components = []ComponentInstance{{ID: "reception", Name: "Reception", Definition: DefinitionRef{ID: packages[0].ID, Version: "1.0.0"}, Artifacts: []ArtifactRef{}, Bindings: []Binding{}}, {ID: "consultation", Name: "Consultation", Definition: DefinitionRef{ID: packages[1].ID, Version: "1.0.0"}, Artifacts: []ArtifactRef{}, Bindings: []Binding{}}}
	d.Connections = []Connection{{ID: "encounter-handoff", Source: "reception", Target: "consultation", Kind: "data", SourcePort: &AspectEndpoint{Slot: "root", Interface: "outputs", Port: "encounter"}, TargetPort: &AspectEndpoint{Slot: "root", Interface: "inputs", Port: "encounter"}}}
	d.Contract = Contract{ID: "connection-exercise", Revision: "1", Claims: []Claim{{ID: "future-observation", Kind: "required_evidence", Description: "Placeholder for future execution; static compatibility does not evaluate this claim"}}, Assumptions: []Assumption{{ID: "static-only", Description: "This exercise checks declared port types only; it does not implement a clinical handoff."}}}
	mismatch := d
	mismatch.Components = append([]ComponentInstance{}, d.Components...)
	mismatch.ID += "-mismatch"
	mismatch.Name = "Reception → Consultation · contract mismatch"
	mismatch.Components[1].Definition.Version = "2.0.0"
	return ConnectionExamplePack{Design: d, Mismatch: mismatch, Packages: packages}
}
