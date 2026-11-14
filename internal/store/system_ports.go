package store

import (
	"context"
	"software-studio/internal/model"
	"software-studio/internal/system"
)

type SystemPortInspection struct {
	Ports  []system.InstancePort   `json:"ports"`
	Issues []model.ValidationIssue `json:"issues"`
}

// InspectSystemPorts inspects only component interfaces. It deliberately does
// not return a system compatibility verdict. This lets incomplete diagrams
// discover ports before claims, deployments or both connection ends are filled.
func (s *Store) InspectSystemPorts(ctx context.Context, d system.SystemDesign) (SystemPortInspection, error) {
	projection := system.SystemDesign{SchemaVersion: 1, ID: "interface-inspection", Name: "Interface inspection", Revision: "1", Components: append([]system.ComponentInstance{}, d.Components...), Connections: []system.Connection{}, Contract: system.Contract{ID: "interface-inspection", Revision: "1", Claims: []system.Claim{{ID: "not-evaluated", Kind: "required_evidence"}}}}
	for i := range projection.Components {
		projection.Components[i].Bindings = nil
	}
	result, err := s.ResolveSystem(ctx, projection)
	return SystemPortInspection{Ports: result.Ports, Issues: result.Issues}, err
}
