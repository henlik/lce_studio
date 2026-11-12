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

	"software-studio/internal/model"
	"software-studio/internal/store"
)

func dataHTTPExample(t *testing.T) (model.DataContractLock, model.DataFixture) {
	t.Helper()
	c := model.DataContract{SchemaVersion: 1, ID: "test.people", Version: "1.0.0", Name: "People", Description: "Closed person records", Author: "Test engineer", License: "MIT",
		Records:  []model.DataRecord{{Name: "person", Fields: []model.DataField{{Name: "id", Type: "integer"}, {Name: "display", Type: "string"}}}},
		Entities: []model.DataEntity{{Name: "people", Record: "person", PrimaryKey: []string{"id"}, UniqueKeys: [][]string{}}}, Relationships: []model.DataRelationship{}}
	if issues := model.ValidateDataContract(c); len(issues) != 0 {
		t.Fatal(issues)
	}
	digest, err := model.DataContractDigest(c)
	if err != nil {
		t.Fatal(err)
	}
	id, display := int64(1), "Ada"
	f := model.DataFixture{Name: "People snapshot", Rows: map[string][]map[string]model.TypedValue{"people": {{"id": {Type: "integer", Integer: &id}, "display": {Type: "string", String: &display}}}}}
	return model.DataContractLock{Contract: c, Digest: digest}, f
}

func rawDataRequest(s *Server, path, raw string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("POST", path, strings.NewReader(raw))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	return w
}

func TestDataHTTPPrepareCatalogAndCanonicalLock(t *testing.T) {
	s := testServer(t)
	lock, _ := dataHTTPExample(t)
	w := req(t, s, "GET", "/api/data/examples", nil)
	var examples []model.DataExample
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &examples) != nil || len(examples) < 2 {
		t.Fatalf("examples: %d %s", w.Code, w.Body.String())
	}
	for _, example := range examples {
		if result, err := model.CheckDataFixture(example.Contract, example.Fixture); err != nil || !result.Valid {
			t.Fatalf("invalid example: %#v %v", result, err)
		}
	}
	if w := req(t, s, "GET", "/api/data/contracts", nil); w.Code != 200 || strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatalf("empty catalog: %d %s", w.Code, w.Body.String())
	}
	w = req(t, s, "POST", "/api/data/prepare", lock.Contract)
	var prepared model.DataContractLock
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &prepared) != nil || !reflect.DeepEqual(prepared, lock) {
		t.Fatalf("prepare: %d %s", w.Code, w.Body.String())
	}
	raw, err := json.Marshal(lock)
	if err != nil {
		t.Fatal(err)
	}
	w = rawDataRequest(s, "/api/data/validate-lock", strings.ReplaceAll(strings.ReplaceAll(string(raw), `"contract":`, `"CONTRACT":`), `"schemaVersion":`, `"SCHEMAVERSION":`))
	var validated struct {
		Valid  bool                    `json:"valid"`
		Issues []model.ValidationIssue `json:"issues"`
		Lock   model.DataContractLock  `json:"lock"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &validated) != nil || !validated.Valid || len(validated.Issues) != 0 || !reflect.DeepEqual(validated.Lock, lock) || strings.Contains(w.Body.String(), "SCHEMAVERSION") {
		t.Fatalf("canonical lock: %d %s", w.Code, w.Body.String())
	}
	tampered := lock
	tampered.Digest = strings.Repeat("0", 64)
	if w := req(t, s, "POST", "/api/data/validate-lock", tampered); w.Code != 200 || !strings.Contains(w.Body.String(), `"valid":false`) || strings.Contains(w.Body.String(), `"lock":`) {
		t.Fatalf("invalid lock: %d %s", w.Code, w.Body.String())
	}
	w = req(t, s, "POST", "/api/data/contracts", lock)
	var installed store.DataContractRecord
	if w.Code != 201 || json.Unmarshal(w.Body.Bytes(), &installed) != nil || installed.Digest != lock.Digest || installed.InstalledAt == "" {
		t.Fatalf("install: %d %s", w.Code, w.Body.String())
	}
	w = req(t, s, "POST", "/api/data/contracts", lock)
	var again store.DataContractRecord
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &again) != nil || !reflect.DeepEqual(again, installed) {
		t.Fatalf("reinstall: %d %s", w.Code, w.Body.String())
	}
	changed := lock
	changed.Contract.Description = "Changed metadata"
	changed.Digest, err = model.DataContractDigest(changed.Contract)
	if err != nil {
		t.Fatal(err)
	}
	if w := req(t, s, "POST", "/api/data/contracts", changed); w.Code != 409 {
		t.Fatalf("conflict: %d %s", w.Code, w.Body.String())
	}
	query := url.Values{"id": {lock.Contract.ID}, "version": {lock.Contract.Version}, "digest": {lock.Digest}}
	w = req(t, s, "GET", "/api/data/contract?"+query.Encode(), nil)
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &again) != nil || !reflect.DeepEqual(again, installed) {
		t.Fatalf("exact retrieval: %d %s", w.Code, w.Body.String())
	}
	query.Set("digest", strings.Repeat("0", 64))
	if w := req(t, s, "GET", "/api/data/contract?"+query.Encode(), nil); w.Code != 404 {
		t.Fatalf("wrong digest: %d %s", w.Code, w.Body.String())
	}
	if w := req(t, s, "GET", "/api/data/contract?id=bad&version=latest&digest=no", nil); w.Code != 422 {
		t.Fatalf("bad reference: %d %s", w.Code, w.Body.String())
	}
	if w := req(t, s, "GET", "/api/components", nil); strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatalf("data install modified components: %s", w.Body.String())
	}
}

func TestDataHTTPChecksAndImmutableEvidence(t *testing.T) {
	s := testServer(t)
	lock, fixture := dataHTTPExample(t)
	body := dataCheckRequest{Contract: lock, Fixture: fixture}
	w := req(t, s, "POST", "/api/data/check", body)
	var checked struct {
		Result  model.DataCheckResult `json:"result"`
		Fixture model.DataFixture     `json:"fixture"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &checked) != nil || !checked.Result.Valid || checked.Result.EngineVersion != "0.5.0" || checked.Result.RowCount != 1 || !reflect.DeepEqual(checked.Fixture, fixture) {
		t.Fatalf("check: %d %s", w.Code, w.Body.String())
	}
	if w := req(t, s, "GET", "/api/data/validations", nil); strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatalf("check saved evidence: %s", w.Body.String())
	}
	w = req(t, s, "POST", "/api/data/validate", body)
	var first store.DataValidationRecord
	if w.Code != 201 || json.Unmarshal(w.Body.Bytes(), &first) != nil || first.ID == "" || first.CreatedAt == "" || !first.Result.Valid || !reflect.DeepEqual(first.Contract, lock) || !reflect.DeepEqual(first.Fixture, fixture) {
		t.Fatalf("save evidence: %d %s", w.Code, w.Body.String())
	}
	body.Fixture.Rows["people"] = append(body.Fixture.Rows["people"], body.Fixture.Rows["people"][0])
	w = req(t, s, "POST", "/api/data/check", body)
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &checked) != nil || checked.Result.Valid || checked.Result.RowCount != 2 || len(checked.Result.Issues) == 0 {
		t.Fatalf("invalid rows must be verdicts: %d %s", w.Code, w.Body.String())
	}
	w = req(t, s, "POST", "/api/data/validate", body)
	var invalid store.DataValidationRecord
	if w.Code != 201 || json.Unmarshal(w.Body.Bytes(), &invalid) != nil || invalid.Result.Valid || invalid.ID == first.ID {
		t.Fatalf("invalid verdict evidence: %d %s", w.Code, w.Body.String())
	}
	body.Contract.Contract.Name = "Edited draft"
	body.Fixture.Name = "Edited fixture"
	w = req(t, s, "GET", "/api/data/validations/"+first.ID, nil)
	var detail store.DataValidationRecord
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &detail) != nil || !reflect.DeepEqual(first, detail) {
		t.Fatalf("snapshot changed: %d %s", w.Code, w.Body.String())
	}
	w = req(t, s, "GET", "/api/data/validations", nil)
	var history []store.DataValidationSummary
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &history) != nil || len(history) != 2 || history[0].ID != invalid.ID || history[0].Valid || !history[1].Valid || history[0].RowCount != 2 || history[0].ContractID != lock.Contract.ID || strings.Contains(w.Body.String(), `"fixture"`) {
		t.Fatalf("history: %d %s", w.Code, w.Body.String())
	}
	if w := req(t, s, "GET", "/api/data/contracts", nil); strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatalf("checks installed contract: %s", w.Body.String())
	}
}

func TestDataHTTPFailuresDoNotSaveEvidence(t *testing.T) {
	s := testServer(t)
	lock, fixture := dataHTTPExample(t)
	tampered := lock
	tampered.Digest = strings.Repeat("0", 64)
	if w := req(t, s, "POST", "/api/data/validate", dataCheckRequest{tampered, fixture}); w.Code != 422 {
		t.Fatalf("tampered lock: %d %s", w.Code, w.Body.String())
	}
	tooMany := fixture
	tooMany.Rows = map[string][]map[string]model.TypedValue{"people": make([]map[string]model.TypedValue, 257)}
	if w := req(t, s, "POST", "/api/data/validate", dataCheckRequest{lock, tooMany}); w.Code != 422 {
		t.Fatalf("row budget: %d %s", w.Code, w.Body.String())
	}
	large := fixture
	huge := strings.Repeat("x", 512*1024)
	large.Rows = map[string][]map[string]model.TypedValue{"people": {{"display": {Type: "string", String: &huge}}}}
	if w := req(t, s, "POST", "/api/data/validate", dataCheckRequest{lock, large}); w.Code != 422 {
		t.Fatalf("byte budget: %d %s", w.Code, w.Body.String())
	}
	for _, path := range []string{"/api/data/check", "/api/data/validate"} {
		body := dataCheckRequest{lock, fixture}
		s.runs <- struct{}{}
		s.runs <- struct{}{}
		w := req(t, s, "POST", path, body)
		<-s.runs
		<-s.runs
		if w.Code != 429 {
			t.Fatalf("shared admission %s: %d %s", path, w.Code, w.Body.String())
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
			t.Fatalf("cancelled %s: %d slots=%d %s", path, w.Code, len(s.runs), w.Body.String())
		}
	}
	if w := req(t, s, "GET", "/api/data/validations", nil); strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatalf("errors saved evidence: %s", w.Body.String())
	}
	for _, path := range []string{"/api/data/validations/missing", "/api/data/validations/a/b"} {
		if w := req(t, s, "GET", path, nil); w.Code != 404 {
			t.Fatalf("missing history %s: %d", path, w.Code)
		}
	}
}

func TestDataHTTPCanonicalStructuredValuesAndComparison(t *testing.T) {
	s := testServer(t)
	lock, fixture := dataHTTPExample(t)
	ref := model.DataTypeRef{ContractID: lock.Contract.ID, Version: lock.Contract.Version, Digest: lock.Digest, Record: "person"}
	items := []model.TypedValue{}
	values := map[string]model.TypedValue{"person": {Type: "record", RecordRef: &ref, Record: fixture.Rows["people"][0]}, "empty": {Type: "collection", RecordRef: &ref, Collection: &items}}
	body := map[string]any{"contracts": []model.DataContractLock{lock}, "values": values}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	w := rawDataRequest(s, "/api/data/values", strings.ReplaceAll(strings.ReplaceAll(string(raw), `"type":`, `"TYPE":`), `"recordRef":`, `"RECORDREF":`))
	var result struct {
		Valid  bool                        `json:"valid"`
		Issues []model.ValidationIssue     `json:"issues"`
		Values map[string]model.TypedValue `json:"values"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &result) != nil || !result.Valid || !reflect.DeepEqual(result.Values, values) || strings.Contains(w.Body.String(), `"TYPE"`) || !strings.Contains(w.Body.String(), `"collection":[]`) {
		t.Fatalf("canonical values: %d %s", w.Code, w.Body.String())
	}
	ref.Digest = strings.Repeat("0", 64)
	if w := req(t, s, "POST", "/api/data/values", body); w.Code != 200 || !strings.Contains(w.Body.String(), `"valid":false`) || strings.Contains(w.Body.String(), `"values":`) {
		t.Fatalf("wrong nominal identity: %d %s", w.Code, w.Body.String())
	}
	if w := rawDataRequest(s, "/api/data/values", `{"contracts":[],"values":{}}`); w.Code != 200 || !strings.Contains(w.Body.String(), `"values":{}`) {
		t.Fatalf("empty canonical map: %d %s", w.Code, w.Body.String())
	}
	if w := rawDataRequest(s, "/api/data/values", `{"contracts":[],"values":{"n":{"type":"integer","integer":9007199254740992}}}`); w.Code != 200 || !strings.Contains(w.Body.String(), `"valid":false`) {
		t.Fatalf("unsafe integer: %d %s", w.Code, w.Body.String())
	}
	if w := rawDataRequest(s, "/api/data/values", `{"contracts":[],"values":{"n":{"type":"integer","integer":1.5}}}`); w.Code != 400 {
		t.Fatalf("fractional integer: %d %s", w.Code, w.Body.String())
	}
	changed := lock
	changed.Contract.Version = "2.0.0"
	changed.Contract.Description = "Metadata only"
	changed.Digest, err = model.DataContractDigest(changed.Contract)
	if err != nil {
		t.Fatal(err)
	}
	w = req(t, s, "POST", "/api/data/compare", map[string]any{"old": lock, "new": changed})
	var comparison model.DataCompatibility
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &comparison) != nil || !comparison.Compatible || comparison.Direction != "old-to-new" {
		t.Fatalf("comparison: %d %s", w.Code, w.Body.String())
	}
	changed.Contract.ID = "test.other"
	changed.Digest, err = model.DataContractDigest(changed.Contract)
	if err != nil {
		t.Fatal(err)
	}
	if w := req(t, s, "POST", "/api/data/compare", map[string]any{"old": lock, "new": changed}); w.Code != 422 {
		t.Fatalf("unrelated contracts: %d %s", w.Code, w.Body.String())
	}
}

func TestDataHTTPBoundaryPolicies(t *testing.T) {
	s := testServer(t)
	for _, path := range []string{"/api/data/prepare", "/api/data/validate-lock", "/api/data/contracts", "/api/data/check", "/api/data/validate", "/api/data/compare", "/api/data/values"} {
		for _, tc := range []struct {
			body, media, origin string
			want                int
		}{{`{"unknown":true}`, "application/json", "", 400}, {`{} {}`, "application/json", "", 400}, {`{}`, "text/plain", "", 415}, {`{}`, "application/json", "https://attacker.example", 403}} {
			r := httptest.NewRequest("POST", path, strings.NewReader(tc.body))
			r.Header.Set("Content-Type", tc.media)
			if tc.origin != "" {
				r.Header.Set("Origin", tc.origin)
			}
			w := httptest.NewRecorder()
			s.ServeHTTP(w, r)
			if w.Code != tc.want {
				t.Fatalf("%s: want%d got%d %s", path, tc.want, w.Code, w.Body.String())
			}
		}
		if path != "/api/data/contracts" {
			if w := req(t, s, "GET", path, nil); w.Code != http.StatusMethodNotAllowed {
				t.Fatalf("GET write endpoint %s: %d", path, w.Code)
			}
		}
	}
	for _, path := range []string{"/api/data/examples", "/api/data/contracts", "/api/data/contract", "/api/data/validations", "/api/data/validations/validation-1", "/api/components/structured-primitives", "/api/components/structured-examples"} {
		if w := req(t, s, "DELETE", path, nil); w.Code != http.StatusMethodNotAllowed {
			t.Fatalf("DELETE immutable endpoint %s: %d", path, w.Code)
		}
	}
}

func TestStructuredHTTPPortableBundleAndCatalogIsolation(t *testing.T) {
	s := testServer(t)
	w := req(t, s, "GET", "/api/components/structured-primitives", nil)
	var primitives []model.PrimitiveSpec
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &primitives) != nil || len(primitives) != 16 {
		t.Fatalf("structured primitives: %d %s", w.Code, w.Body.String())
	}
	w = req(t, s, "GET", "/api/components/structured-examples", nil)
	var examples []model.ComponentBundle
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &examples) != nil || len(examples) < 2 {
		t.Fatalf("structured examples: %d %s", w.Code, w.Body.String())
	}
	for _, bundle := range examples {
		if issues := model.ValidateComponentBundle(bundle); len(issues) != 0 {
			t.Fatal(issues)
		}
		var root model.ComponentPackage
		for _, lock := range bundle.Packages {
			if lock.Package.ID == bundle.Root.PackageID && lock.Package.Version == bundle.Root.Version {
				root = lock.Package
			}
		}
		if root.SchemaVersion != 3 || len(root.Tests) == 0 {
			t.Fatalf("structured root: %#v", root)
		}
		if w := req(t, s, "POST", "/api/components", root); w.Code != 422 {
			t.Fatalf("raw schema3 accepted: %d %s", w.Code, w.Body.String())
		}
		if w := req(t, s, "POST", "/api/components/test", map[string]any{"bundle": bundle}); w.Code != 200 || !strings.Contains(w.Body.String(), `"passed":true`) {
			t.Fatalf("structured cases: %d %s", w.Code, w.Body.String())
		}
		w := req(t, s, "POST", "/api/components/evaluate", map[string]any{"bundle": bundle, "inputs": root.Tests[0].Inputs})
		var evaluation store.EvaluationRecord
		if w.Code != 201 || json.Unmarshal(w.Body.Bytes(), &evaluation) != nil || !reflect.DeepEqual(evaluation.Result.Outputs, root.Tests[0].Expected) || evaluation.Result.EngineVersion != "0.5.0" {
			t.Fatalf("structured evaluation: %d %s", w.Code, w.Body.String())
		}
		if w := req(t, s, "POST", "/api/components/bundles", bundle); w.Code != 201 {
			t.Fatalf("schema3 installation: %d %s", w.Code, w.Body.String())
		}
		query := url.Values{"id": {bundle.Root.PackageID}, "version": {bundle.Root.Version}, "digest": {bundle.Root.Digest}}
		w = req(t, s, "GET", "/api/components/bundle?"+query.Encode(), nil)
		var exported model.ComponentBundle
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &exported) != nil || exported.Root != bundle.Root || len(exported.Packages) != len(bundle.Packages) || len(model.ValidateComponentBundle(exported)) != 0 {
			t.Fatalf("schema3 closure export: %d %s", w.Code, w.Body.String())
		}
	}
	if w := req(t, s, "GET", "/api/data/contracts", nil); strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatalf("component install silently populated data catalog: %s", w.Body.String())
	}
}
