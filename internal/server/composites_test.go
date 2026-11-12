package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"software-studio/internal/behavior"
	"software-studio/internal/model"
	"software-studio/internal/store"
)

func compositeExample(t *testing.T) (model.ComponentBundle, model.ComponentPackage, map[string]model.TypedValue) {
	t.Helper()
	examples := model.ExampleComponentBundles()
	if len(examples) == 0 {
		t.Fatal("no composite examples")
	}
	bundle := examples[0]
	for _, lock := range bundle.Packages {
		if lock.Package.ID == bundle.Root.PackageID && lock.Package.Version == bundle.Root.Version {
			if len(lock.Package.Tests) == 0 {
				t.Fatal("composite example has no tests")
			}
			return bundle, lock.Package, lock.Package.Tests[0].Inputs
		}
	}
	t.Fatal("composite example root is missing")
	return model.ComponentBundle{}, model.ComponentPackage{}, nil
}

func componentDependencies(bundle model.ComponentBundle) []model.PackageLock {
	dependencies := make([]model.PackageLock, 0)
	for _, lock := range bundle.Packages {
		if lock.Package.ID != bundle.Root.PackageID || lock.Package.Version != bundle.Root.Version {
			dependencies = append(dependencies, lock)
		}
	}
	return dependencies
}

func TestCompositeHTTPWorkbenchAndEvidence(t *testing.T) {
	s := testServer(t)
	w := req(t, s, "GET", "/api/components/primitives", nil)
	var primitives []model.PrimitiveSpec
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &primitives) != nil || len(primitives) != 11 {
		t.Fatalf("primitive catalog: %d %s", w.Code, w.Body.String())
	}
	w = req(t, s, "GET", "/api/components/composite-examples", nil)
	var examples []model.ComponentBundle
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &examples) != nil || len(examples) < 2 {
		t.Fatalf("composite examples: %d %s", w.Code, w.Body.String())
	}
	for _, bundle := range examples {
		if issues := model.ValidateComponentBundle(bundle); len(issues) != 0 {
			t.Fatalf("invalid bundled composite: %#v", issues)
		}
	}
	bundle, root, inputs := compositeExample(t)
	w = req(t, s, "POST", "/api/components/prepare", map[string]any{"package": root, "dependencies": componentDependencies(bundle)})
	var prepared model.ComponentBundle
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &prepared) != nil || prepared.Root != bundle.Root || len(model.ValidateComponentBundle(prepared)) != 0 {
		t.Fatalf("prepare portable closure: %d %s", w.Code, w.Body.String())
	}
	if w := req(t, s, "POST", "/api/components/validate-bundle", prepared); w.Code != 200 || !strings.Contains(w.Body.String(), `"valid":true`) {
		t.Fatalf("validate bundle: %d %s", w.Code, w.Body.String())
	}
	if w := req(t, s, "GET", "/api/components/evaluations", nil); w.Code != 200 || strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatalf("empty evaluation history: %d %s", w.Code, w.Body.String())
	}
	w = req(t, s, "POST", "/api/components/evaluate", map[string]any{"bundle": prepared, "inputs": inputs})
	var first store.EvaluationRecord
	if w.Code != 201 || json.Unmarshal(w.Body.Bytes(), &first) != nil || first.ID == "" || first.CreatedAt == "" || first.Result.EngineVersion != behavior.Version || len(first.Result.Events) == 0 {
		t.Fatalf("portable evaluation: %d %s", w.Code, w.Body.String())
	}
	if !reflect.DeepEqual(first.Result.Outputs, root.Tests[0].Expected) || first.Bundle.Root != prepared.Root || !reflect.DeepEqual(first.Inputs, inputs) {
		t.Fatalf("evaluation values: %#v", first)
	}
	if w := req(t, s, "GET", "/api/components", nil); strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatal("portable prepare/evaluate installed catalog packages")
	}
	w = req(t, s, "GET", "/api/components/evaluations/"+first.ID, nil)
	var detail store.EvaluationRecord
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &detail) != nil || !reflect.DeepEqual(first, detail) {
		t.Fatalf("immutable evidence detail: %d %s", w.Code, w.Body.String())
	}
	w = req(t, s, "GET", "/api/components/evaluations", nil)
	var history []store.EvaluationSummary
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &history) != nil || len(history) != 1 || history[0].ID != first.ID || history[0].Operations != first.Result.Operations || strings.Contains(w.Body.String(), `"events"`) {
		t.Fatalf("summary history: %d %s", w.Code, w.Body.String())
	}
	w = req(t, s, "POST", "/api/components/test", map[string]any{"bundle": bundle})
	var tests behavior.TestResult
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &tests) != nil || !tests.Passed || len(tests.Cases) != len(root.Tests) {
		t.Fatalf("root test suite: %d %s", w.Code, w.Body.String())
	}
	w = req(t, s, "POST", "/api/components/bundles", bundle)
	var installed struct {
		Bundle    model.ComponentBundle   `json:"bundle"`
		Installed []store.ComponentRecord `json:"installed"`
	}
	if w.Code != 201 || json.Unmarshal(w.Body.Bytes(), &installed) != nil || len(installed.Installed) != len(bundle.Packages) {
		t.Fatalf("install full closure: %d %s", w.Code, w.Body.String())
	}
	if w := req(t, s, "POST", "/api/components/bundles", bundle); w.Code != 200 {
		t.Fatalf("identical bundle reinstall: %d %s", w.Code, w.Body.String())
	}
	query := url.Values{"id": {bundle.Root.PackageID}, "version": {bundle.Root.Version}, "digest": {bundle.Root.Digest}}
	w = req(t, s, "GET", "/api/components/bundle?"+query.Encode(), nil)
	var exported model.ComponentBundle
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &exported) != nil || exported.Root != bundle.Root || len(exported.Packages) != len(bundle.Packages) || len(model.ValidateComponentBundle(exported)) != 0 {
		t.Fatalf("exact closure export: %d %s", w.Code, w.Body.String())
	}
	if w := req(t, s, "POST", "/api/components", root); w.Code != 422 || !strings.Contains(w.Body.String(), "/api/components/bundles") {
		t.Fatalf("raw schema2 install must direct closure install: %d %s", w.Code, w.Body.String())
	}
	// A portable edited definition remains evaluable even when that exact version
	// conflicts with local catalog content; simulation never substitutes it.
	root.Description += " Changed only in this portable bundle"
	changed, issues := model.PrepareComponentBundle(root, componentDependencies(bundle))
	if len(issues) != 0 {
		t.Fatal(issues)
	}
	if w := req(t, s, "POST", "/api/components/bundles", changed); w.Code != 409 {
		t.Fatalf("immutable version conflict: %d %s", w.Code, w.Body.String())
	}
	w = req(t, s, "POST", "/api/components/evaluate", map[string]any{"bundle": changed, "inputs": inputs})
	var portable store.EvaluationRecord
	if w.Code != 201 || json.Unmarshal(w.Body.Bytes(), &portable) != nil || portable.Result.BundleHash == first.Result.BundleHash || portable.Bundle.Root.Digest == first.Bundle.Root.Digest {
		t.Fatalf("portable definition replaced by catalog: %d %s", w.Code, w.Body.String())
	}
	w = req(t, s, "GET", "/api/components/evaluations/"+first.ID, nil)
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &detail) != nil || !reflect.DeepEqual(first, detail) {
		t.Fatalf("earlier evidence changed after editing: %d %s", w.Code, w.Body.String())
	}
}

func TestCompositeHTTPFailuresDoNotSaveEvidence(t *testing.T) {
	s := testServer(t)
	bundle, root, inputs := compositeExample(t)
	if w := req(t, s, "POST", "/api/components/prepare", map[string]any{"package": root, "dependencies": []model.PackageLock{}}); w.Code != 422 {
		t.Fatalf("missing dependencies accepted: %d %s", w.Code, w.Body.String())
	}
	if w := req(t, s, "POST", "/api/components/evaluate", map[string]any{"bundle": bundle, "inputs": map[string]model.TypedValue{}}); w.Code != 422 {
		t.Fatalf("missing typed inputs accepted: %d %s", w.Code, w.Body.String())
	}
	tampered := bundle
	tampered.Root.Digest = strings.Repeat("0", 64)
	if w := req(t, s, "POST", "/api/components/validate-bundle", tampered); w.Code != 200 || !strings.Contains(w.Body.String(), `"valid":false`) {
		t.Fatalf("tampered validation: %d %s", w.Code, w.Body.String())
	}
	if w := req(t, s, "POST", "/api/components/evaluate", map[string]any{"bundle": tampered, "inputs": inputs}); w.Code != 422 {
		t.Fatalf("tampered execution: %d %s", w.Code, w.Body.String())
	}
	if w := req(t, s, "GET", "/api/components/bundle?id=bad&version=latest&digest=no", nil); w.Code != 422 {
		t.Fatalf("invalid exact reference: %d %s", w.Code, w.Body.String())
	}
	query := url.Values{"id": {bundle.Root.PackageID}, "version": {bundle.Root.Version}, "digest": {bundle.Root.Digest}}
	if w := req(t, s, "GET", "/api/components/bundle?"+query.Encode(), nil); w.Code != 404 {
		t.Fatalf("missing installed closure: %d %s", w.Code, w.Body.String())
	}
	for _, path := range []string{"/api/components/evaluations/missing", "/api/components/evaluations/a/b"} {
		if w := req(t, s, "GET", path, nil); w.Code != 404 {
			t.Fatalf("missing evidence %s: %d %s", path, w.Code, w.Body.String())
		}
	}
	for _, path := range []string{"/api/components/evaluate", "/api/components/test"} {
		body := map[string]any{"bundle": bundle}
		if strings.HasSuffix(path, "/evaluate") {
			body["inputs"] = inputs
		}
		// Occupy both existing simulation slots, proving evaluation and tests do
		// not create an independent unlimited execution queue.
		s.runs <- struct{}{}
		s.runs <- struct{}{}
		w := req(t, s, "POST", path, body)
		<-s.runs
		<-s.runs
		if w.Code != 429 {
			t.Fatalf("busy %s: %d %s", path, w.Code, w.Body.String())
		}
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		r := httptest.NewRequest("POST", path, bytes.NewReader(raw)).WithContext(ctx)
		r.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		s.ServeHTTP(w, r)
		if w.Code != 408 || len(s.runs) != 0 {
			t.Fatalf("cancelled %s: %d, slots %d, %s", path, w.Code, len(s.runs), w.Body.String())
		}
	}
	if w := req(t, s, "GET", "/api/components/evaluations", nil); strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatalf("failed computation saved success evidence: %s", w.Body.String())
	}
}

func TestCompositeHTTPBoundaryPolicies(t *testing.T) {
	s := testServer(t)
	for _, path := range []string{"/api/components/prepare", "/api/components/validate-bundle", "/api/components/bundles", "/api/components/evaluate", "/api/components/test"} {
		for _, tc := range []struct {
			body, media, origin string
			want                int
		}{
			{`{"unknown":true}`, "application/json", "", 400},
			{`{} {}`, "application/json", "", 400},
			{`{}`, "text/plain", "", 415},
			{`{}`, "application/json", "https://attacker.example", 403},
		} {
			r := httptest.NewRequest("POST", path, strings.NewReader(tc.body))
			r.Header.Set("Content-Type", tc.media)
			if tc.origin != "" {
				r.Header.Set("Origin", tc.origin)
			}
			w := httptest.NewRecorder()
			s.ServeHTTP(w, r)
			if w.Code != tc.want {
				t.Fatalf("%s: want %d got %d: %s", path, tc.want, w.Code, w.Body.String())
			}
		}
		if w := req(t, s, "GET", path, nil); w.Code != http.StatusMethodNotAllowed {
			t.Fatalf("GET mutation path %s: %d", path, w.Code)
		}
	}
	for _, path := range []string{"/api/components/primitives", "/api/components/composite-examples", "/api/components/bundle", "/api/components/evaluations", "/api/components/evaluations/evaluation-1"} {
		if w := req(t, s, "DELETE", path, nil); w.Code != http.StatusMethodNotAllowed {
			t.Fatalf("evidence deletion supported at %s: %d", path, w.Code)
		}
	}
}

func TestCompositeValidationReturnsCanonicalTypedBundle(t *testing.T) {
	bundle, _, _ := compositeExample(t)
	original, err := json.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}
	aliased := strings.NewReplacer(`"behavior":`, `"Behavior":`, `"inputs":`, `"Inputs":`, `"nodes":`, `"Nodes":`).Replace(string(original))
	s := &Server{}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/components/validate-bundle", strings.NewReader(aliased))
	r.Header.Set("Content-Type", "application/json")
	s.componentValidateBundle(w, r)
	var response struct {
		Valid  bool            `json:"valid"`
		Bundle json.RawMessage `json:"bundle"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &response) != nil || !response.Valid {
		t.Fatalf("aliased typed fields: %d %s", w.Code, w.Body.String())
	}
	if !bytes.Equal(response.Bundle, original) {
		t.Fatal("valid response did not preserve the canonical typed bundle and case-sensitive map keys")
	}
	bundle.Root.Digest = strings.Repeat("0", 64)
	invalid, _ := json.Marshal(bundle)
	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodPost, "/api/components/validate-bundle", bytes.NewReader(invalid))
	r.Header.Set("Content-Type", "application/json")
	s.componentValidateBundle(w, r)
	response.Bundle = nil
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &response) != nil || response.Valid || len(response.Bundle) != 0 {
		t.Fatalf("invalid bundle returned as canonical: %d %s", w.Code, w.Body.String())
	}
}
