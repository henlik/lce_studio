package system_test

import (
	"software-studio/internal/system"
	"testing"
)

func TestReferenceResolutionIsExactScopedAndReserved(t *testing.T) {
	d := system.ReceptionExample()
	baseline := system.Resolve(d, nil)
	if !baseline.Valid || len(baseline.References) != 2 {
		t.Fatalf("builtins: %+v", baseline)
	}
	d.Components[0].Definition = system.DefinitionRef{ID: "example.client", Version: "1.0.0"}
	entries := []system.CatalogEntry{{Kind: "component", ID: "example.client", Version: "2.0.0", Ready: true, Digest: "newer"}}
	if system.Resolve(d, entries).Valid {
		t.Fatal("substituted newer version")
	}
	entries[0].Version = "1.0.0"
	entries[0].Kind = "data"
	if system.Resolve(d, entries).Valid {
		t.Fatal("substituted wrong artifact kind")
	}
	entries[0].Kind = "component"
	entries[0].Ready = false
	entries[0].Issue = "Content digest mismatch"
	if system.Resolve(d, entries).Valid {
		t.Fatal("accepted corrupt definition")
	}
	entries[0].Ready = true
	if !system.Resolve(d, entries).Valid {
		t.Fatal("rejected installed exact definition")
	}
	if system.Resolve(d, append(entries, entries[0])).Valid {
		t.Fatal("accepted ambiguous catalog identity")
	}
	d = system.ReceptionExample()
	evil := system.CatalogEntry{Kind: "component", ID: d.Components[0].Definition.ID, Version: "1.0.0", Ready: true, Digest: "shadow"}
	r := system.Resolve(d, []system.CatalogEntry{evil})
	if !r.Valid || r.References[0].Digest == "shadow" || r.References[0].Source != "builtin-adapter" {
		t.Fatal("catalog shadowed reserved adapter")
	}
}

func TestResolutionPointsToMissingAspect(t *testing.T) {
	d := system.ReceptionExample()
	d.Components[1].Artifacts = []system.ArtifactRef{{Kind: "state", ID: "example.state", Version: "1.0.0"}}
	r := system.Resolve(d, nil)
	if r.Valid || len(r.Issues) != 1 || r.Issues[0].Path != "components[1].artifacts[0]" {
		t.Fatalf("missing path: %+v", r)
	}
	if len(r.References) != 2 {
		t.Fatal("lost valid direct references")
	}
}
