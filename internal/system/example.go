package system

// ReceptionExample is the pinned topology supported by the first real
// implementation adapter. These built-in references identify adapter semantics;
// they do not imply every studio catalog artifact has an implementation adapter.
func ReceptionExample() SystemDesign {
	return SystemDesign{
		SchemaVersion: 1, ID: "reception-intake", Name: "Durable reception intake", Revision: "1",
		Components: []ComponentInstance{
			{ID: "client", Name: "Intake client", Definition: DefinitionRef{ID: "builtin.intake-client", Version: "1.0.0"}, Artifacts: []ArtifactRef{}, Bindings: []Binding{}},
			{ID: "intake", Name: "Durable intake", Definition: DefinitionRef{ID: "builtin.durable-intake", Version: "1.0.0"}, Artifacts: []ArtifactRef{}, Bindings: []Binding{{Requirement: "encounter-storage", ResourceID: "encounters-db"}}},
		},
		Connections:     []Connection{{ID: "submit-intake", Source: "client", Target: "intake", Kind: "request"}},
		DeploymentPlans: []DeploymentPlan{{ID: "local-sandbox", Name: "Local conformance sandbox", Resources: []Resource{{ID: "local-host", Kind: "compute"}, {ID: "encounters-db", Kind: "postgresql"}}, Placements: []Placement{{ComponentID: "client", ResourceID: "local-host"}, {ComponentID: "intake", ResourceID: "local-host"}}}},
		Contract: Contract{ID: "durable-intake-contract", Revision: "1", Assumptions: []Assumption{{ID: "bounded-trace", Description: "The adapter records all semantic boundary events for the bounded run and reports missing observations as evidence_gap."}, {ID: "durable-store", Description: "Persistence events denote completed PostgreSQL commits; storage durability outside the sandbox is not established by this experiment."}}, Claims: []Claim{
			{ID: "one-encounter", Kind: "exactly_one_record", Description: "A logical request creates at most one encounter; a successful request has exactly one."},
			{ID: "stable-response", Kind: "response_matches_record", Description: "Every successful response identifies the persisted encounter for that request."},
			{ID: "honest-success", Kind: "no_false_success", Description: "Success is reported only after matching persistence evidence."},
			{ID: "complete-evidence", Kind: "required_evidence", Description: "Each request has received and terminal response evidence, without declared gaps."},
		}},
	}
}
