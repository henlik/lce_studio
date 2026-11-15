package server

import (
	"context"
	"encoding/json"
	sourceexamples "software-studio/examples/source"
	"software-studio/internal/configuration"
	"testing"
)

func TestConfigurationProjectAPI(t *testing.T) {
	s := New(nil, "")
	w := req(t, s, "POST", "/api/configuration/migrate", map[string]any{"source": sourceexamples.StockReservation})
	var p configuration.Project
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &p) != nil {
		t.Fatal(w.Code, w.Body.String())
	}
	w = req(t, s, "POST", "/api/configuration/compile", map[string]any{"project": p})
	var c configuration.Compilation
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &c) != nil || !c.Valid {
		t.Fatal(w.Code, w.Body.String())
	}
	w = req(t, s, "POST", "/api/configuration/format", map[string]any{"project": p, "expectedHash": "stale"})
	if w.Code != 409 {
		t.Fatal(w.Code, w.Body.String())
	}
	w = req(t, s, "POST", "/api/configuration/run", map[string]any{"project": p})
	var r configuration.Report
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &r) != nil || !r.Passed {
		t.Fatal(w.Code, w.Body.String())
	}
	for _, path := range []string{"compile", "run", "pack"} {
		if w = req(t, s, "POST", "/api/configuration/"+path, map[string]any{"project": p, "executeArbitrary": true}); w.Code != 400 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	pkg, e := configuration.Pack(context.Background(), "test.api", "1.0.0", p, true)
	if e != nil {
		t.Fatal(e)
	}
	pkg.Report.Cases[0].Passed = false
	pkg.Digest = configuration.PackageDigest(pkg)
	w = req(t, s, "POST", "/api/configuration/replay", map[string]any{"package": pkg})
	if w.Code != 422 {
		t.Fatal(w.Code, w.Body.String())
	}
}
func TestConfigurationInstallationAPI(t *testing.T) {
	s := testServer(t)
	p, e := configuration.MigrateLegacy(sourceexamples.StockReservation)
	if e != nil {
		t.Fatal(e)
	}
	pkg, e := configuration.Pack(context.Background(), "test.api-install", "1.0.0", p, true)
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 2; i++ {
		w := req(t, s, "POST", "/api/configuration/install", map[string]any{"package": pkg})
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	w := req(t, s, "GET", "/api/configuration/package?id=test.api-install&version=1.0.0", nil)
	var got configuration.Package
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &got) != nil || got.Digest != pkg.Digest {
		t.Fatal(w.Code, w.Body.String())
	}
	w = req(t, s, "GET", "/api/configuration/packages", nil)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
}
