package system

import (
	"strings"
	"testing"
)

func TestDefinitionPinsAndValidation(t *testing.T) {
	d := ComponentDefinition{SchemaVersion: 1, ID: "demo.reception", Version: "1.0.0", Name: "Reception", Aspects: []AspectRef{{Slot: "patient", Kind: "data", ID: "demo.patient", Version: "1.0.0", Digest: strings.Repeat("a", 64)}}}
	catalog := []CatalogEntry{{Kind: "data", ID: "demo.patient", Version: "1.0.0", Digest: strings.Repeat("a", 64), Ready: true}}
	if v := ResolveDefinition(d, catalog); !v.Valid {
		t.Fatal(v)
	}
	for _, change := range []func(*ComponentDefinition){
		func(d *ComponentDefinition) { d.ID = "builtin.override" },
		func(d *ComponentDefinition) { d.Aspects = append(d.Aspects, d.Aspects[0]) },
		func(d *ComponentDefinition) { d.Aspects[0].Kind = "definition" },
		func(d *ComponentDefinition) { d.Aspects[0].Digest = "" },
		func(d *ComponentDefinition) { d.Aspects[0].Version = "latest" },
	} {
		bad := d
		bad.Aspects = append([]AspectRef{}, d.Aspects...)
		change(&bad)
		if ValidateDefinition(bad).Valid {
			t.Fatalf("invalid accepted: %+v", bad)
		}
	}
	if ResolveDefinition(d, nil).Valid {
		t.Fatal("missing pin accepted")
	}
	catalog[0].Digest = strings.Repeat("b", 64)
	if ResolveDefinition(d, catalog).Valid {
		t.Fatal("changed content accepted")
	}
	first, _ := DefinitionDigest(d)
	d.Aspects[0].Slot = "other"
	second, _ := DefinitionDigest(d)
	if first == second {
		t.Fatal("ownership not hashed")
	}
}
