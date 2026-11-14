package system

import (
	"software-studio/internal/model"
	"testing"
)

func TestAspectLinksRequireExactTypesAndSingleProducers(t *testing.T) {
	source := AspectEndpoint{Slot: "screening", Interface: "outputs", Port: "emergency"}
	target := AspectEndpoint{Slot: "intake", Interface: "inputs", Port: "emergency"}
	ports := []AspectPort{{AspectEndpoint: source, Direction: "output", Type: "boolean"}, {AspectEndpoint: target, Direction: "input", Type: "boolean"}}
	links := []AspectLink{{ID: "emergency", Source: source, Target: target}}
	if issues := CheckAspectLinks(links, ports); len(issues) > 0 {
		t.Fatal(issues)
	}
	for _, change := range []func([]AspectPort){
		func(p []AspectPort) { p[1].Type = "string" },
		func(p []AspectPort) { p[1].Direction = "output" },
		func(p []AspectPort) { p[0].Port = "missing" },
	} {
		copyPorts := append([]AspectPort{}, ports...)
		change(copyPorts)
		if len(CheckAspectLinks(links, copyPorts)) == 0 {
			t.Fatal("incompatible link accepted")
		}
	}
	if len(CheckAspectLinks(append(links, links[0]), ports)) == 0 {
		t.Fatal("multiple producers accepted")
	}
	ref := model.DataTypeRef{ContractID: "patient.data", Version: "1.0.0", Digest: "abc", Record: "Patient"}
	ports[0].Type = "record"
	ports[1].Type = "record"
	ports[0].RecordRef = &ref
	other := ref
	ports[1].RecordRef = &other
	if len(CheckAspectLinks(links, ports)) != 0 {
		t.Fatal("same nominal record rejected")
	}
	for _, change := range []func(*model.DataTypeRef){func(r *model.DataTypeRef) { r.Version = "2.0.0" }, func(r *model.DataTypeRef) { r.ContractID = "other.data" }, func(r *model.DataTypeRef) { r.Digest = "changed" }, func(r *model.DataTypeRef) { r.Record = "Other" }} {
		other = ref
		change(&other)
		if len(CheckAspectLinks(links, ports)) == 0 {
			t.Fatal("different nominal record accepted")
		}
	}
}
func TestDefinitionLinkShapeAndLegacyDigest(t *testing.T) {
	d := ComponentDefinition{SchemaVersion: 1, ID: "test.simple", Version: "1.0.0", Name: "Simple", Aspects: []AspectRef{{Slot: "test", Kind: "data", ID: "test.data", Version: "1.0.0", Digest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}}
	before, _ := DefinitionDigest(d)
	d.Links = []AspectLink{}
	after, _ := DefinitionDigest(d)
	if before != after {
		t.Fatal("empty optional links changed legacy digest")
	}
	d.Links = []AspectLink{{ID: "link", Source: AspectEndpoint{Slot: "absent"}, Target: AspectEndpoint{Slot: "test"}}}
	if ValidateDefinition(d).Valid {
		t.Fatal("malformed link accepted")
	}
}
