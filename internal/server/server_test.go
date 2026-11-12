package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"software-studio/internal/model"
	"software-studio/internal/store"
)

func testServer(t *testing.T) *Server {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "studio.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return New(db, "")
}

func req(t *testing.T, s http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var raw []byte
	if body != nil {
		var err error
		raw, err = json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
	}
	r := httptest.NewRequest(method, path, bytes.NewReader(raw))
	if body != nil {
		r.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	return w
}

func TestProjectCRUDAndDrafts(t *testing.T) {
	s := testServer(t)
	if w := req(t, s, "GET", "/api/projects", nil); w.Code != 200 || strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatalf("empty projects: %d %s", w.Code, w.Body.String())
	}
	p := model.ExampleProject()
	p.ID = ""
	p.Edges = []model.Edge{} // incomplete design is a valid saved draft
	w := req(t, s, "POST", "/api/projects", p)
	if w.Code != 200 {
		t.Fatalf("save draft: %d %s", w.Code, w.Body.String())
	}
	var saved model.Project
	if err := json.Unmarshal(w.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	if saved.ID == "" {
		t.Fatal("missing generated ID")
	}
	if w := req(t, s, "GET", "/api/projects/"+saved.ID, nil); w.Code != 200 {
		t.Fatalf("read draft: %d %s", w.Code, w.Body.String())
	}
	if w := req(t, s, "POST", "/api/validate", saved); w.Code != 200 || !strings.Contains(w.Body.String(), `"valid":false`) {
		t.Fatalf("draft must not validate for simulation: %d %s", w.Code, w.Body.String())
	}
	if w := req(t, s, "POST", "/api/validate?draft=true", saved); w.Code != 200 || !strings.Contains(w.Body.String(), `"valid":true`) {
		t.Fatalf("well-formed incomplete draft must be importable: %d %s", w.Code, w.Body.String())
	}
	invalidDraft := saved
	invalidDraft.Scenario.RequestCount = 201
	if w := req(t, s, "POST", "/api/validate?draft=true", invalidDraft); w.Code != 200 || !strings.Contains(w.Body.String(), `"valid":false`) {
		t.Fatalf("draft validation must still enforce field bounds: %d %s", w.Code, w.Body.String())
	}
	saved.Name = "Updated name"
	if w := req(t, s, "POST", "/api/projects", saved); w.Code != 200 {
		t.Fatalf("update: %d %s", w.Code, w.Body.String())
	}
	if w := req(t, s, "DELETE", "/api/projects/"+saved.ID, nil); w.Code != 204 {
		t.Fatalf("delete: %d", w.Code)
	}
	if w := req(t, s, "GET", "/api/projects/"+saved.ID, nil); w.Code != 404 {
		t.Fatalf("after delete: %d", w.Code)
	}
}

func TestRunSnapshotAndHistory(t *testing.T) {
	s := testServer(t)
	p := model.ExampleProject()
	w := req(t, s, "POST", "/api/runs", map[string]any{"project": p})
	if w.Code != 201 {
		t.Fatalf("run: %d %s", w.Code, w.Body.String())
	}
	var run model.Run
	if err := json.Unmarshal(w.Body.Bytes(), &run); err != nil {
		t.Fatal(err)
	}
	if run.ID == "" || run.CreatedAt == "" || run.ModelHash != ModelHash(p) || len(run.Events) == 0 {
		t.Fatalf("incomplete run: %#v", run)
	}
	for _, v := range run.Verdicts {
		if !v.Passed {
			t.Errorf("default requirement failed: %#v", v)
		}
	}
	if w := req(t, s, "GET", "/api/projects", nil); strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatal("running a draft must not silently save editable project")
	}
	if w := req(t, s, "GET", "/api/runs/"+run.ID, nil); w.Code != 200 || !bytes.Contains(w.Body.Bytes(), []byte(run.ModelHash)) {
		t.Fatalf("detail: %d %s", w.Code, w.Body.String())
	}
	w = req(t, s, "GET", "/api/runs?projectId="+p.ID, nil)
	var history []store.RunSummary
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &history) != nil || len(history) != 1 || history[0].ID != run.ID || len(history[0].Events) != 0 {
		t.Fatalf("history: %d %s", w.Code, w.Body.String())
	}
	p.Nodes[2].Config.Deduplicate = false
	w = req(t, s, "POST", "/api/runs", map[string]any{"project": p})
	var failed model.Run
	_ = json.Unmarshal(w.Body.Bytes(), &failed)
	if w.Code != 201 || failed.Metrics.DuplicateWrites == 0 {
		t.Fatalf("unsafe retry must be a real failed design: %d %s", w.Code, w.Body.String())
	}
	if failed.ModelHash == run.ModelHash {
		t.Fatal("changed snapshot reused old hash")
	}
}

func TestAPIErrorBoundaries(t *testing.T) {
	s := testServer(t)
	cases := []struct {
		name, body, contentType string
		want                    int
	}{
		{"missing content type", `{}`, "", 415},
		{"unknown field", `{"unknown":true}`, "application/json", 400},
		{"trailing JSON", `{} {}`, "application/json", 400},
		{"malformed", `{"name":`, "application/json", 400},
		{"too large", `{"description":"` + strings.Repeat("x", MaxBodyBytes) + `"}`, "application/json", 413},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "/api/projects", strings.NewReader(tc.body))
			r.Header.Set("Content-Type", tc.contentType)
			w := httptest.NewRecorder()
			s.ServeHTTP(w, r)
			if w.Code != tc.want {
				t.Fatalf("got %d: %s", w.Code, w.Body.String())
			}
		})
	}
	bad := model.ExampleProject()
	bad.Scenario.RequestCount = 201
	if w := req(t, s, "POST", "/api/runs", map[string]any{"project": bad}); w.Code != 422 {
		t.Fatalf("invalid model: %d %s", w.Code, w.Body.String())
	}
	for _, path := range []string{"/api", "/api/missing", "/api/projects/a/b", "/api/runs/missing"} {
		w := req(t, s, "GET", path, nil)
		if w.Code != 404 || !strings.HasPrefix(w.Header().Get("Content-Type"), "application/json") {
			t.Fatalf("API fallback %s: %d %s", path, w.Code, w.Body.String())
		}
	}
	if w := req(t, s, "PUT", "/api/projects", nil); w.Code != 405 {
		t.Fatalf("unsupported method: %d", w.Code)
	}
}

func TestCrossOriginWrites(t *testing.T) {
	s := testServer(t)
	p := model.ExampleProject()
	body, _ := json.Marshal(p)
	for _, origin := range []string{"https://attacker.example", "null", "http://example.com/anything"} {
		r := httptest.NewRequest("POST", "http://example.com/api/projects", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Origin", origin)
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatalf("origin %s accepted: %d", origin, w.Code)
		}
	}
	r := httptest.NewRequest("POST", "http://example.com/api/projects", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Origin", "http://example.com")
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("same-origin request rejected: %d %s", w.Code, w.Body.String())
	}
	r = httptest.NewRequest("DELETE", "http://example.com/api/projects/"+p.ID, nil)
	r.Header.Set("Sec-Fetch-Site", "cross-site")
	w = httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatalf("cross-site deletion accepted: %d", w.Code)
	}
}

func TestStaticFrontendAndAPIFallback(t *testing.T) {
	s := testServer(t)
	s.webDir = t.TempDir()
	if err := os.WriteFile(filepath.Join(s.webDir, "index.html"), []byte("<h1>Studio</h1>"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/", "/design", "/project/retry-lab"} {
		w := req(t, s, "GET", path, nil)
		if w.Code != 200 || !strings.Contains(w.Body.String(), "Studio") {
			t.Fatalf("SPA route %s: %d %s", path, w.Code, w.Body.String())
		}
	}
	if w := req(t, s, "GET", "/assets/missing.js", nil); w.Code != 404 {
		t.Fatalf("missing asset served SPA: %d", w.Code)
	}
	if w := req(t, s, "GET", "/api/unknown", nil); w.Code != 404 || strings.Contains(w.Body.String(), "<h1>") {
		t.Fatalf("API got HTML: %d %s", w.Code, w.Body.String())
	}
	if w := req(t, s, "HEAD", "/", nil); w.Code != 200 {
		t.Fatalf("HEAD: %d", w.Code)
	}
}

func TestDecodeReadFailure(t *testing.T) {
	s := testServer(t)
	r := httptest.NewRequest("POST", "/api/projects", io.NopCloser(strings.NewReader("")))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != 400 {
		t.Fatalf("empty body: %d", w.Code)
	}
}
