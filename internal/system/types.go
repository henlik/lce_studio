// Package system connects independently versioned studio artifacts and evaluates
// bounded, observable conformance claims. It does not execute arbitrary code.
package system

type SystemDesign struct {
	SchemaVersion   int                 `json:"schemaVersion"`
	ID              string              `json:"id"`
	Name            string              `json:"name"`
	Revision        string              `json:"revision"`
	Components      []ComponentInstance `json:"components"`
	Connections     []Connection        `json:"connections"`
	DeploymentPlans []DeploymentPlan    `json:"deploymentPlans"`
	Contract        Contract            `json:"contract"`
}
type DefinitionRef struct {
	Kind    string `json:"kind,omitempty"`
	ID      string `json:"id"`
	Version string `json:"version"`
}
type ArtifactRef struct {
	Kind    string `json:"kind"`
	ID      string `json:"id"`
	Version string `json:"version"`
}
type Binding struct {
	Requirement string `json:"requirement"`
	ResourceID  string `json:"resourceId"`
}
type ComponentInstance struct {
	ID         string        `json:"id"`
	Name       string        `json:"name"`
	Definition DefinitionRef `json:"definition"`
	Artifacts  []ArtifactRef `json:"artifacts"`
	Bindings   []Binding     `json:"bindings"`
}
type Connection struct {
	ID         string          `json:"id"`
	Source     string          `json:"source"`
	Target     string          `json:"target"`
	Kind       string          `json:"kind"`
	SourcePort *AspectEndpoint `json:"sourcePort,omitempty"`
	TargetPort *AspectEndpoint `json:"targetPort,omitempty"`
}
type Resource struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
}
type Placement struct {
	ComponentID string `json:"componentId"`
	ResourceID  string `json:"resourceId"`
}
type DeploymentPlan struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	Resources  []Resource  `json:"resources"`
	Placements []Placement `json:"placements"`
}
type Contract struct {
	ID          string       `json:"id"`
	Revision    string       `json:"revision"`
	Claims      []Claim      `json:"claims"`
	Assumptions []Assumption `json:"assumptions"`
}
type Claim struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	Description string `json:"description"`
}

// Assumptions are declared preconditions, not automatically established facts.
type Assumption struct {
	ID          string `json:"id"`
	Description string `json:"description"`
}

// Event sequence numbers establish order within a complete, bounded run. They
// are not distributed wall-clock timestamps. Repeated persisted observations of
// the same encounter do not constitute duplicate records.
type Event struct {
	Sequence    int    `json:"sequence"`
	Kind        string `json:"kind"`
	RequestID   string `json:"requestId"`
	EncounterID string `json:"encounterId,omitempty"`
	Detail      string `json:"detail,omitempty"`
}
type Verdict struct {
	ClaimID           string `json:"claimId"`
	Status            string `json:"status"`
	Message           string `json:"message"`
	EvidenceSequences []int  `json:"evidenceSequences"`
}
