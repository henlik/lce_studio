package server

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"software-studio/internal/model"
	"software-studio/internal/store"
)

func TestComponentCatalogHTTP(t *testing.T) {
	s := testServer(t)
	if w := req(t, s, "GET", "/api/components", nil); w.Code != 200 || strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatalf("empty catalog: %d %s", w.Code, w.Body.String())
	}
	w := req(t, s, "GET", "/api/components/examples", nil)
	var examples []model.PackageLock
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &examples) != nil || len(examples) < 3 {
		t.Fatalf("bundled components: %d %s", w.Code, w.Body.String())
	}
	for _, example := range examples {
		digest, err := model.ComponentDigest(example.Package)
		if err != nil || digest != example.Digest || len(model.ValidateComponentPackage(example.Package)) != 0 {
			t.Fatalf("invalid bundled package: %#v %v", example, err)
		}
	}
	p := examples[0].Package
	w = req(t, s, "POST", "/api/components", p)
	var first store.ComponentRecord
	if w.Code != 201 || json.Unmarshal(w.Body.Bytes(), &first) != nil || first.Digest != examples[0].Digest || first.InstalledAt == "" {
		t.Fatalf("install: %d %s", w.Code, w.Body.String())
	}
	w = req(t, s, "POST", "/api/components", p)
	var again store.ComponentRecord
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &again) != nil || !reflect.DeepEqual(first, again) {
		t.Fatalf("identical reinstall: %d %s", w.Code, w.Body.String())
	}
	p.Description += " Edited"
	if w := req(t, s, "POST", "/api/components", p); w.Code != 409 {
		t.Fatalf("conflicting version: %d %s", w.Code, w.Body.String())
	}
	p.Version = "99.0.0"
	if w := req(t, s, "POST", "/api/components", p); w.Code != 201 {
		t.Fatalf("new version: %d %s", w.Code, w.Body.String())
	}
	w = req(t, s, "GET", "/api/components", nil)
	var list []store.ComponentRecord
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &list) != nil || len(list) != 2 || !reflect.DeepEqual(list[0], first) {
		t.Fatalf("listing: %d %s", w.Code, w.Body.String())
	}
	p.Kernel = "arbitrary-code"
	if w := req(t, s, "POST", "/api/components", p); w.Code != 422 || !strings.Contains(w.Body.String(), `"issues"`) {
		t.Fatalf("unsupported kernel: %d %s", w.Code, w.Body.String())
	}
	for _, path := range []string{"/api/components", "/api/components/examples"} {
		if w := req(t, s, "DELETE", path, nil); w.Code != 405 {
			t.Fatalf("unsupported removal %s: %d", path, w.Code)
		}
	}
}

func TestComponentCatalogHTTPBoundaries(t *testing.T) {
	s := testServer(t)
	cases := []struct {
		name, body, contentType, origin string
		want                            int
	}{
		{"unknown field", `{"script":"run me"}`, "application/json", "", 400},
		{"trailing JSON", `{} {}`, "application/json", "", 400},
		{"unsupported media", `{}`, "text/plain", "", 415},
		{"cross-origin", `{}`, "application/json", "https://attacker.example", 403},
		{"too large", `{"description":"` + strings.Repeat("x", MaxBodyBytes) + `"}`, "application/json", "", 413},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "/api/components", bytes.NewBufferString(tc.body))
			r.Header.Set("Content-Type", tc.contentType)
			if tc.origin != "" {
				r.Header.Set("Origin", tc.origin)
			}
			w := httptest.NewRecorder()
			s.ServeHTTP(w, r)
			if w.Code != tc.want {
				t.Fatalf("got %d: %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestBranchingExampleIsPortable(t *testing.T) {
	s := testServer(t)
	w := req(t, s, "GET", "/api/example?template=branching", nil)
	var project model.Project
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &project) != nil || project.SchemaVersion != 3 || len(project.Packages) == 0 {
		t.Fatalf("branching example: %d %s", w.Code, w.Body.String())
	}
	if w := req(t, s, "POST", "/api/validate", project); w.Code != 200 || !strings.Contains(w.Body.String(), `"valid":true`) {
		t.Fatalf("portable validation without installed packages: %d %s", w.Code, w.Body.String())
	}
	if w := req(t, s, "POST", "/api/projects", project); w.Code != 200 {
		t.Fatalf("portable save: %d %s", w.Code, w.Body.String())
	}
	w = req(t, s, "GET", "/api/projects/"+project.ID, nil)
	var saved model.Project
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &saved) != nil || !reflect.DeepEqual(saved, project) {
		t.Fatalf("portable round trip: %d %s", w.Code, w.Body.String())
	}
	if w := req(t, s, "GET", "/api/components", nil); strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatal("project import silently installed catalog packages")
	}
}
