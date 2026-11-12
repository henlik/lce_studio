package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"software-studio/internal/model"
	"software-studio/internal/store"
	"software-studio/internal/workflow"
)

// Portable snapshots preserve canonical JSON. Empty optional Go collections may
// be omitted and decode as nil, which does not change the stored artifact.
func workflowJSONEqual(a, b any) bool {
	left, leftErr := json.Marshal(a)
	right, rightErr := json.Marshal(b)
	return leftErr == nil && rightErr == nil && bytes.Equal(left, right)
}

func workflowHTTPExample(t *testing.T) model.WorkflowExample {
	t.Helper()
	examples := model.ExampleWorkflowScenarios()
	if len(examples) == 0 {
		t.Fatalf("want workflow examples, got %d", len(examples))
	}
	if issues := model.ValidateWorkflowScenario(examples[0].Workflow, examples[0].Scenario); len(issues) > 0 {
		t.Fatal(issues)
	}
	if examples[0].Scenario.Expected == nil {
		t.Fatal("first example needs independent expectations")
	}
	return examples[0]
}

func TestWorkflowHTTPExamplesPrepareAndImmutableCatalog(t *testing.T) {
	s := testServer(t)
	example := workflowHTTPExample(t)
	lock := example.Workflow
	w := req(t, s, "GET", "/api/health", nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"version":"0.8.0"`) {
		t.Fatalf("health: %d %s", w.Code, w.Body.String())
	}
	w = req(t, s, "GET", "/api/workflow/examples", nil)
	var examples []model.WorkflowExample
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &examples) != nil || len(examples) == 0 {
		t.Fatalf("examples: %d %s", w.Code, w.Body.String())
	}
	for _, e := range examples {
		if issues := model.ValidateWorkflowScenario(e.Workflow, e.Scenario); len(issues) > 0 {
			t.Fatal(issues)
		}
	}
	if w := req(t, s, "GET", "/api/workflow/packages", nil); w.Code != 200 || strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatalf("empty catalog: %d %s", w.Code, w.Body.String())
	}
	w = req(t, s, "POST", "/api/workflow/prepare", lock.Workflow)
	var prepared model.WorkflowLock
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &prepared) != nil || !workflowJSONEqual(prepared, lock) {
		t.Fatalf("prepare: %d %s", w.Code, w.Body.String())
	}
	raw, err := json.Marshal(lock)
	if err != nil {
		t.Fatal(err)
	}
	w = rawDataRequest(s, "/api/workflow/validate-lock", strings.ReplaceAll(strings.ReplaceAll(string(raw), `"workflow":`, `"WORKFLOW":`), `"schemaVersion":`, `"SCHEMAVERSION":`))
	var validated struct {
		Valid    bool                    `json:"valid"`
		Issues   []model.ValidationIssue `json:"issues"`
		Workflow model.WorkflowLock      `json:"workflow"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &validated) != nil || !validated.Valid || len(validated.Issues) != 0 || !workflowJSONEqual(validated.Workflow, lock) || strings.Contains(w.Body.String(), "SCHEMAVERSION") {
		t.Fatalf("canonical lock: %d %s", w.Code, w.Body.String())
	}
	tampered := lock
	tampered.Digest = strings.Repeat("0", 64)
	if w := req(t, s, "POST", "/api/workflow/validate-lock", tampered); w.Code != 200 || !strings.Contains(w.Body.String(), `"valid":false`) || strings.Contains(w.Body.String(), `"workflow":`) {
		t.Fatalf("invalid lock verdict: %d %s", w.Code, w.Body.String())
	}
	if w := req(t, s, "POST", "/api/workflow/packages", tampered); w.Code != 422 {
		t.Fatalf("invalid install: %d %s", w.Code, w.Body.String())
	}
	w = req(t, s, "POST", "/api/workflow/packages", lock)
	var installed store.WorkflowPackageRecord
	if w.Code != 201 || json.Unmarshal(w.Body.Bytes(), &installed) != nil || installed.Digest != lock.Digest || installed.InstalledAt == "" {
		t.Fatalf("install: %d %s", w.Code, w.Body.String())
	}
	w = req(t, s, "POST", "/api/workflow/packages", lock)
	var again store.WorkflowPackageRecord
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &again) != nil || !workflowJSONEqual(again, installed) {
		t.Fatalf("reinstall: %d %s", w.Code, w.Body.String())
	}
	changed := lock
	changed.Workflow.Description = "Changed metadata"
	changed.Digest, err = model.WorkflowDigest(changed.Workflow)
	if err != nil {
		t.Fatal(err)
	}
	if w := req(t, s, "POST", "/api/workflow/packages", changed); w.Code != 409 {
		t.Fatalf("immutable conflict: %d %s", w.Code, w.Body.String())
	}
	query := url.Values{"id": {lock.Workflow.ID}, "version": {lock.Workflow.Version}, "digest": {lock.Digest}}
	w = req(t, s, "GET", "/api/workflow/package?"+query.Encode(), nil)
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &again) != nil || !workflowJSONEqual(again, installed) {
		t.Fatalf("exact retrieval: %d %s", w.Code, w.Body.String())
	}
	query.Set("digest", strings.Repeat("0", 64))
	if w := req(t, s, "GET", "/api/workflow/package?"+query.Encode(), nil); w.Code != 404 {
		t.Fatalf("wrong digest: %d %s", w.Code, w.Body.String())
	}
	if w := req(t, s, "GET", "/api/workflow/package?id=bad&version=latest&digest=no", nil); w.Code != 422 {
		t.Fatalf("invalid reference: %d %s", w.Code, w.Body.String())
	}
	for _, path := range []string{"/api/delivery/models", "/api/state/machines", "/api/components", "/api/data/contracts"} {
		if w := req(t, s, "GET", path, nil); strings.TrimSpace(w.Body.String()) != "[]" {
			t.Fatalf("definition installed dependencies at %s: %s", path, w.Body.String())
		}
	}
}

func TestWorkflowHTTPChecksRecordedOutcomesAndIndependentRuns(t *testing.T) {
	s := testServer(t)
	example := workflowHTTPExample(t)
	raw, err := json.Marshal(example)
	if err != nil {
		t.Fatal(err)
	}
	w := rawDataRequest(s, "/api/workflow/check", strings.ReplaceAll(strings.ReplaceAll(string(raw), `"instanceId":`, `"INSTANCEID":`), `"integer":`, `"INTEGER":`))
	var checked struct {
		Scenario model.WorkflowScenario `json:"scenario"`
		Result   workflow.Result        `json:"result"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &checked) != nil || !checked.Result.Verified || !checked.Result.Passed || checked.Result.EngineVersion != "0.8.0" || !workflowJSONEqual(checked.Scenario, example.Scenario) || strings.Contains(w.Body.String(), "INSTANCEID") {
		t.Fatalf("canonical check: %d %s", w.Code, w.Body.String())
	}
	if w := req(t, s, "GET", "/api/workflow/runs", nil); strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatalf("check persisted evidence: %s", w.Body.String())
	}
	w = req(t, s, "POST", "/api/workflow/runs", example)
	var first store.WorkflowRunRecord
	if w.Code != 201 || json.Unmarshal(w.Body.Bytes(), &first) != nil || first.ID == "" || first.CreatedAt == "" || !workflowJSONEqual(first.Result, checked.Result) || !workflowJSONEqual(first.Workflow, example.Workflow) || !workflowJSONEqual(first.Scenario, example.Scenario) {
		t.Fatalf("recorded run: %d %s", w.Code, w.Body.String())
	}
	w = req(t, s, "POST", "/api/workflow/runs", example)
	var second store.WorkflowRunRecord
	if w.Code != 201 || json.Unmarshal(w.Body.Bytes(), &second) != nil || second.ID == first.ID || !workflowJSONEqual(first.Result, second.Result) {
		t.Fatalf("independent runs reused live workflow: %d %s", w.Code, w.Body.String())
	}
	example.Scenario.Expected.Tasks++
	w = req(t, s, "POST", "/api/workflow/runs", example)
	var failed store.WorkflowRunRecord
	if w.Code != 201 || json.Unmarshal(w.Body.Bytes(), &failed) != nil || !failed.Result.Verified || failed.Result.Passed {
		t.Fatalf("failed expectations should be evidence: %d %s", w.Code, w.Body.String())
	}
	example.Scenario.Expected = nil
	w = req(t, s, "POST", "/api/workflow/runs", example)
	var unverified store.WorkflowRunRecord
	if w.Code != 201 || json.Unmarshal(w.Body.Bytes(), &unverified) != nil || unverified.Result.Verified || unverified.Result.Passed {
		t.Fatalf("empty expectations must remain unverified: %d %s", w.Code, w.Body.String())
	}
	example.Workflow.Workflow.Name = "Edited draft"
	example.Scenario.Name = "Edited scenario"
	w = req(t, s, "GET", "/api/workflow/runs/"+first.ID, nil)
	var detail store.WorkflowRunRecord
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &detail) != nil || !workflowJSONEqual(first, detail) {
		t.Fatalf("prior evidence mutated: %d %s", w.Code, w.Body.String())
	}
	w = req(t, s, "GET", "/api/workflow/runs", nil)
	var history []store.WorkflowRunSummary
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &history) != nil || len(history) != 4 || history[0].ID != unverified.ID || history[0].Verified || history[0].Passed || history[1].ID != failed.ID || !history[1].Verified || history[1].Passed || history[3].WorkflowID != first.Workflow.Workflow.ID || history[3].WorkflowVersion != first.Workflow.Workflow.Version || history[3].Status != first.Result.Status || history[3].Outcome != first.Result.Outcome || history[3].EventCount != len(first.Result.Events) || history[3].TaskCount != len(first.Result.Tasks) || strings.Contains(w.Body.String(), `"scenario":`) || strings.Contains(w.Body.String(), `"instances":`) {
		t.Fatalf("history summary: %d %s", w.Code, w.Body.String())
	}
	for _, path := range []string{"/api/workflow/packages", "/api/delivery/models", "/api/state/machines", "/api/components", "/api/data/contracts"} {
		if w := req(t, s, "GET", path, nil); strings.TrimSpace(w.Body.String()) != "[]" {
			t.Fatalf("portable execution installed at %s: %s", path, w.Body.String())
		}
	}
}

func TestWorkflowHTTPBoundaryPolicies(t *testing.T) {
	s := testServer(t)
	for _, path := range []string{"/api/workflow/prepare", "/api/workflow/validate-lock", "/api/workflow/packages", "/api/workflow/check", "/api/workflow/runs"} {
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
				t.Fatalf("%s want%d got%d %s", path, tc.want, w.Code, w.Body.String())
			}
		}
		if path != "/api/workflow/packages" && path != "/api/workflow/runs" {
			if w := req(t, s, "GET", path, nil); w.Code != http.StatusMethodNotAllowed {
				t.Fatalf("GET mutation %s: %d", path, w.Code)
			}
		}
	}
	if w := rawDataRequest(s, "/api/workflow/prepare", `{"name":"`+strings.Repeat("x", MaxBodyBytes)+`"}`); w.Code != 413 {
		t.Fatalf("HTTP body limit: %d %s", w.Code, w.Body.String())
	}
	for _, path := range []string{"/api/workflow/examples", "/api/workflow/packages", "/api/workflow/package", "/api/workflow/runs", "/api/workflow/runs/workflow-1"} {
		if w := req(t, s, "DELETE", path, nil); w.Code != http.StatusMethodNotAllowed {
			t.Fatalf("DELETE immutable endpoint %s: %d", path, w.Code)
		}
	}
}

func TestWorkflowHTTPAdmissionIncludesValidation(t *testing.T) {
	s := testServer(t)
	for _, path := range []string{"/api/workflow/check", "/api/workflow/runs"} {
		// This is decodable but semantically invalid. A saturated server must
		// reject it before traversing the definition/scenario definitions.
		s.runs <- struct{}{}
		s.runs <- struct{}{}
		w := rawDataRequest(s, path, `{}`)
		<-s.runs
		<-s.runs
		if w.Code != 429 {
			t.Fatalf("admission before validation %s: %d %s", path, w.Code, w.Body.String())
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		r := httptest.NewRequest("POST", path, strings.NewReader(`{}`)).WithContext(ctx)
		r.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		s.ServeHTTP(w, r)
		if w.Code != 408 || len(s.runs) != 0 {
			t.Fatalf("cancelled validation %s: %d slots=%d %s", path, w.Code, len(s.runs), w.Body.String())
		}
		w = rawDataRequest(s, path, `{}`)
		if w.Code != 422 || len(s.runs) != 0 {
			t.Fatalf("validation releases admission %s: %d slots=%d %s", path, w.Code, len(s.runs), w.Body.String())
		}
	}
	if w := req(t, s, "GET", "/api/workflow/runs", nil); strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatalf("rejected validation saved evidence: %s", w.Body.String())
	}
}

func TestWorkflowHTTPWaitingPreviewAndSemanticFailures(t *testing.T) {
	s := testServer(t)
	example := workflowHTTPExample(t)
	example.Scenario.Events = []model.WorkflowEvent{}
	example.Scenario.Expected = nil
	w := req(t, s, "POST", "/api/workflow/check", example)
	var checked struct {
		Scenario model.WorkflowScenario `json:"scenario"`
		Result   workflow.Result        `json:"result"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &checked) != nil || checked.Result.Status != "waiting" || checked.Result.Task == nil || checked.Result.Verified || checked.Result.Passed || !workflowJSONEqual(checked.Scenario, example.Scenario) {
		t.Fatalf("waiting preview: %d %s", w.Code, w.Body.String())
	}
	if w := req(t, s, "GET", "/api/workflow/runs", nil); strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatalf("preview saved evidence: %s", w.Body.String())
	}
	bad := workflowHTTPExample(t)
	bad.Workflow.Digest = strings.Repeat("0", 64)
	if w := req(t, s, "POST", "/api/workflow/runs", bad); w.Code != 422 {
		t.Fatalf("tampered workflow: %d %s", w.Code, w.Body.String())
	}
	bad = workflowHTTPExample(t)
	bad.Scenario.Inputs = nil
	if w := req(t, s, "POST", "/api/workflow/runs", bad); w.Code != 422 {
		t.Fatalf("missing scenario inputs: %d %s", w.Code, w.Body.String())
	}
	bad = workflowHTTPExample(t)
	bad.Scenario.Events = make([]model.WorkflowEvent, 65)
	if w := req(t, s, "POST", "/api/workflow/runs", bad); w.Code != 422 {
		t.Fatalf("event bound: %d %s", w.Code, w.Body.String())
	}
	if w := req(t, s, "GET", "/api/workflow/runs", nil); strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatalf("invalid runs saved evidence: %s", w.Body.String())
	}
	for _, path := range []string{"/api/workflow/runs/missing", "/api/workflow/runs/a/b"} {
		if w := req(t, s, "GET", path, nil); w.Code != 404 {
			t.Fatalf("missing evidence %s: %d", path, w.Code)
		}
	}
	w = req(t, s, "POST", "/api/workflow/runs", example)
	var record store.WorkflowRunRecord
	if w.Code != 201 || json.Unmarshal(w.Body.Bytes(), &record) != nil || record.Result.Status != "waiting" || record.Result.Passed || record.Result.Task == nil {
		t.Fatalf("waiting run snapshot: %d %s", w.Code, w.Body.String())
	}
}

func TestWorkflowHTTPExpectationRawFields(t *testing.T) {
	s := testServer(t)
	example := workflowHTTPExample(t)
	rawExpectation := func(change func(map[string]json.RawMessage)) string {
		t.Helper()
		encoded, err := json.Marshal(example)
		if err != nil {
			t.Fatal(err)
		}
		var body map[string]json.RawMessage
		if err := json.Unmarshal(encoded, &body); err != nil {
			t.Fatal(err)
		}
		var scenario map[string]json.RawMessage
		if err := json.Unmarshal(body["scenario"], &scenario); err != nil {
			t.Fatal(err)
		}
		var expected map[string]json.RawMessage
		if err := json.Unmarshal(scenario["expected"], &expected); err != nil {
			t.Fatal(err)
		}
		change(expected)
		scenario["expected"], err = json.Marshal(expected)
		if err != nil {
			t.Fatal(err)
		}
		body["scenario"], err = json.Marshal(scenario)
		if err != nil {
			t.Fatal(err)
		}
		encoded, err = json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		return string(encoded)
	}
	for _, field := range []string{"tasks", "completedTasks", "cancelledTasks", "eventReceipts"} {
		for _, value := range []string{"", "null", "1.5"} {
			raw := rawExpectation(func(expected map[string]json.RawMessage) {
				if value == "" {
					delete(expected, field)
				} else {
					expected[field] = json.RawMessage(value)
				}
			})
			if w := rawDataRequest(s, "/api/workflow/runs", raw); w.Code != 400 {
				t.Fatalf("%s=%q want400 got%d %s", field, value, w.Code, w.Body.String())
			}
		}
	}
	unknown := rawExpectation(func(expected map[string]json.RawMessage) { expected["unknown"] = json.RawMessage("0") })
	if w := rawDataRequest(s, "/api/workflow/runs", unknown); w.Code != 400 {
		t.Fatalf("unknown expectation member: %d %s", w.Code, w.Body.String())
	}
	if w := req(t, s, "GET", "/api/workflow/runs", nil); strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatalf("malformed expectations saved evidence: %s", w.Body.String())
	}
	mixed := rawExpectation(func(expected map[string]json.RawMessage) {
		for _, field := range []string{"tasks", "completedTasks", "cancelledTasks", "eventReceipts"} {
			expected[strings.ToUpper(field)] = expected[field]
			delete(expected, field)
		}
	})
	w := rawDataRequest(s, "/api/workflow/check", mixed)
	var checked struct {
		Scenario model.WorkflowScenario `json:"scenario"`
		Result   workflow.Result        `json:"result"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &checked) != nil || !checked.Result.Verified || !checked.Result.Passed || !workflowJSONEqual(checked.Scenario, example.Scenario) || strings.Contains(w.Body.String(), `"COMPLETEDTASKS"`) {
		t.Fatalf("mixed-case expectation canonicalization: %d %s", w.Code, w.Body.String())
	}
}

func TestWorkflowHTTPExecutionLimitDoesNotPersist(t *testing.T) {
	s := testServer(t)
	yes := true
	value := model.TypedValue{Type: "boolean", Boolean: &yes}
	component := model.ComponentPackage{
		SchemaVersion: 2, ID: "test.always-repeat", Version: "1.0.0", Name: "Constant branch", Author: "Test engineer", License: "MIT",
		Kernel: "composite", Capabilities: []string{"design", "evaluate"},
		Behavior: &model.Behavior{Inputs: []model.BehaviorPort{}, Outputs: []model.BehaviorPort{{Name: "result", Type: "boolean"}}, Nodes: []model.BehaviorNode{}, Bindings: map[string]model.BehaviorBinding{"result": {Literal: &value}}},
	}
	bundle, issues := model.PrepareComponentBundle(component, []model.PackageLock{})
	if len(issues) != 0 {
		t.Fatal(issues)
	}
	definition := model.WorkflowDefinition{
		SchemaVersion: 1, ID: "test.bounded-loop", Version: "1.0.0", Name: "Bounded automatic loop", Author: "Test engineer", License: "MIT",
		DataContracts: []model.DataContractLock{}, Decisions: []model.ComponentBundle{bundle}, Machines: []model.StateMachineLock{}, Owners: []model.WorkflowOwner{},
		Inputs: []model.BehaviorPort{}, Outputs: []model.BehaviorPort{}, Variables: []model.WorkflowVariable{{Name: "repeat", Type: "boolean", Initial: value}}, Entry: "loop", Signals: []model.WorkflowSignal{},
		Nodes: []model.WorkflowNode{
			{ID: "loop", Name: "Repeat", Kind: "decision", Decision: &model.WorkflowDecision{Component: bundle.Root, Inputs: map[string]model.WorkflowBinding{}, Assign: map[string]string{"repeat": "result"}}, Branch: &model.WorkflowBranch{Variable: "repeat", Then: "loop", Else: "done"}},
			{ID: "done", Name: "Done", Kind: "end", End: &model.WorkflowEnd{Outcome: "finished", Outputs: map[string]model.WorkflowBinding{}}},
		},
	}
	digest, err := model.WorkflowDigest(definition)
	if err != nil {
		t.Fatal(err)
	}
	example := model.WorkflowExample{Workflow: model.WorkflowLock{Workflow: definition, Digest: digest}, Scenario: model.WorkflowScenario{SchemaVersion: 1, Name: "Automatic transition exhaustion", Inputs: map[string]model.TypedValue{}, Events: []model.WorkflowEvent{}}}
	if issues := model.ValidateWorkflowScenario(example.Workflow, example.Scenario); len(issues) != 0 {
		t.Fatal(issues)
	}
	if w := req(t, s, "POST", "/api/workflow/runs", example); w.Code != 422 || !strings.Contains(w.Body.String(), "limit") {
		t.Fatalf("execution limit: %d %s", w.Code, w.Body.String())
	}
	if len(s.runs) != 0 {
		t.Fatal("failed execution retained an admission slot")
	}
	if w := req(t, s, "GET", "/api/workflow/runs", nil); strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatalf("execution limit persisted partial evidence: %s", w.Body.String())
	}
}
