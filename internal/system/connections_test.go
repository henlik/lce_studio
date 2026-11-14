package system

import (
	"software-studio/internal/model"
	"testing"
)

func TestSystemConnectionsScopedTypedAndLegacy(t *testing.T) {
	out := AspectEndpoint{Slot: "behavior", Interface: "outputs", Port: "encounter"}
	in := AspectEndpoint{Slot: "behavior", Interface: "inputs", Port: "encounter"}
	ref := model.DataTypeRef{ContractID: "health.encounter", Version: "1.0.0", Digest: "abc", Record: "Encounter"}
	ports := []InstancePort{{ComponentID: "reception", AspectPort: AspectPort{AspectEndpoint: out, Direction: "output", Type: "record", RecordRef: &ref}}, {ComponentID: "consultation", AspectPort: AspectPort{AspectEndpoint: in, Direction: "input", Type: "record", RecordRef: &ref}}}
	edge := Connection{ID: "handoff", Source: "reception", Target: "consultation", Kind: "data", SourcePort: &out, TargetPort: &in}
	checks, issues := CheckConnections([]Connection{edge}, ports)
	if len(issues) > 0 || checks[0].Status != "checked" {
		t.Fatal(checks, issues)
	}
	for _, mutate := range []func(*Connection){func(c *Connection) { c.Source = "consultation" }, func(c *Connection) { c.TargetPort = &out }, func(c *Connection) { c.TargetPort = nil }} {
		bad := edge
		mutate(&bad)
		_, issues = CheckConnections([]Connection{bad}, ports)
		if len(issues) == 0 {
			t.Fatal("invalid scoped endpoint accepted")
		}
	}
	other := ref
	other.Version = "2.0.0"
	ports[1].RecordRef = &other
	if _, issues = CheckConnections([]Connection{edge}, ports); len(issues) == 0 {
		t.Fatal("record version mismatch accepted")
	}
	ports[1].RecordRef = &ref
	ports[1].Bound = true
	if _, issues = CheckConnections([]Connection{edge}, ports); len(issues) == 0 {
		t.Fatal("internally bound input accepted")
	}
	ports[1].Bound = false
	if _, issues = CheckConnections([]Connection{edge, edge}, ports); len(issues) == 0 {
		t.Fatal("multiple producers accepted")
	}
	edge.SourcePort = nil
	edge.TargetPort = nil
	checks, issues = CheckConnections([]Connection{edge}, ports)
	if len(issues) > 0 || checks[0].Status != "untyped" {
		t.Fatal("legacy connection misrepresented", checks, issues)
	}
}
