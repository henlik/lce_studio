package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"software-studio/internal/model"
	"software-studio/internal/state"
	"software-studio/internal/store"
)

func stateHTTPExample(t *testing.T) model.StateExample {
	t.Helper()
	examples := model.ExampleStateScenarios()
	if len(examples) != 2 {
		t.Fatalf("want two state examples, got %d", len(examples))
	}
	if issues := model.ValidateStateScenario(examples[0].Machine, examples[0].Scenario); len(issues) > 0 {
		t.Fatal(issues)
	}
	return examples[0]
}

func TestStateHTTPExamplesPrepareAndImmutableCatalog(t *testing.T) {
	s := testServer(t)
	example := stateHTTPExample(t)
	lock := example.Machine
	w := req(t, s, "GET", "/api/health", nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"version":"0.8.0"`) {
		t.Fatalf("health: %d %s", w.Code, w.Body.String())
	}
	w = req(t, s, "GET", "/api/state/examples", nil)
	var examples []model.StateExample
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &examples) != nil || len(examples) != 2 || examples[0].Machine.Digest != examples[1].Machine.Digest {
		t.Fatalf("examples: %d %s", w.Code, w.Body.String())
	}
	for _, e := range examples {
		if issues := model.ValidateStateScenario(e.Machine, e.Scenario); len(issues) > 0 {
			t.Fatal(issues)
		}
	}
	if w := req(t, s, "GET", "/api/state/machines", nil); w.Code != 200 || strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatalf("empty catalog: %d %s", w.Code, w.Body.String())
	}
	w = req(t, s, "POST", "/api/state/prepare", lock.Machine)
	var prepared model.StateMachineLock
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &prepared) != nil || !reflect.DeepEqual(prepared, lock) {
		t.Fatalf("prepare: %d %s", w.Code, w.Body.String())
	}
	raw, err := json.Marshal(lock)
	if err != nil {
		t.Fatal(err)
	}
	w = rawDataRequest(s, "/api/state/validate-lock", strings.ReplaceAll(strings.ReplaceAll(string(raw), `"machine":`, `"MACHINE":`), `"schemaVersion":`, `"SCHEMAVERSION":`))
	var validated struct {
		Valid   bool                    `json:"valid"`
		Issues  []model.ValidationIssue `json:"issues"`
		Machine model.StateMachineLock  `json:"machine"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &validated) != nil || !validated.Valid || len(validated.Issues) != 0 || !reflect.DeepEqual(validated.Machine, lock) || strings.Contains(w.Body.String(), "SCHEMAVERSION") {
		t.Fatalf("canonical lock: %d %s", w.Code, w.Body.String())
	}
	tampered := lock
	tampered.Digest = strings.Repeat("0", 64)
	if w := req(t, s, "POST", "/api/state/validate-lock", tampered); w.Code != 200 || !strings.Contains(w.Body.String(), `"valid":false`) || strings.Contains(w.Body.String(), `"machine":`) {
		t.Fatalf("invalid lock verdict: %d %s", w.Code, w.Body.String())
	}
	if w := req(t, s, "POST", "/api/state/machines", tampered); w.Code != 422 {
		t.Fatalf("invalid install: %d %s", w.Code, w.Body.String())
	}
	w = req(t, s, "POST", "/api/state/machines", lock)
	var installed store.StateMachineRecord
	if w.Code != 201 || json.Unmarshal(w.Body.Bytes(), &installed) != nil || installed.Digest != lock.Digest || installed.InstalledAt == "" {
		t.Fatalf("install: %d %s", w.Code, w.Body.String())
	}
	w = req(t, s, "POST", "/api/state/machines", lock)
	var again store.StateMachineRecord
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &again) != nil || !reflect.DeepEqual(again, installed) {
		t.Fatalf("reinstall: %d %s", w.Code, w.Body.String())
	}
	changed := lock
	changed.Machine.Description = "Changed metadata"
	changed.Digest, err = model.StateMachineDigest(changed.Machine)
	if err != nil {
		t.Fatal(err)
	}
	if w := req(t, s, "POST", "/api/state/machines", changed); w.Code != 409 {
		t.Fatalf("immutable conflict: %d %s", w.Code, w.Body.String())
	}
	query := url.Values{"id": {lock.Machine.ID}, "version": {lock.Machine.Version}, "digest": {lock.Digest}}
	w = req(t, s, "GET", "/api/state/machine?"+query.Encode(), nil)
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &again) != nil || !reflect.DeepEqual(again, installed) {
		t.Fatalf("exact retrieval: %d %s", w.Code, w.Body.String())
	}
	query.Set("digest", strings.Repeat("0", 64))
	if w := req(t, s, "GET", "/api/state/machine?"+query.Encode(), nil); w.Code != 404 {
		t.Fatalf("wrong digest: %d %s", w.Code, w.Body.String())
	}
	if w := req(t, s, "GET", "/api/state/machine?id=bad&version=latest&digest=no", nil); w.Code != 422 {
		t.Fatalf("invalid reference: %d %s", w.Code, w.Body.String())
	}
	for _, path := range []string{"/api/components", "/api/data/contracts"} {
		if w := req(t, s, "GET", path, nil); strings.TrimSpace(w.Body.String()) != "[]" {
			t.Fatalf("machine installed dependencies at %s: %s", path, w.Body.String())
		}
	}
}

func TestStateHTTPChecksRecordedOutcomesAndIndependentRuns(t *testing.T) {
	s := testServer(t)
	example := stateHTTPExample(t)
	raw, err := json.Marshal(example)
	if err != nil {
		t.Fatal(err)
	}
	w := rawDataRequest(s, "/api/state/check", strings.ReplaceAll(strings.ReplaceAll(string(raw), `"instanceId":`, `"INSTANCEID":`), `"integer":`, `"INTEGER":`))
	var checked struct {
		Scenario model.StateScenario `json:"scenario"`
		Result   state.Result        `json:"result"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &checked) != nil || !checked.Result.Verified || !checked.Result.Passed || checked.Result.EngineVersion != "0.6.0" || !reflect.DeepEqual(checked.Scenario, example.Scenario) || strings.Contains(w.Body.String(), "INSTANCEID") {
		t.Fatalf("canonical check: %d %s", w.Code, w.Body.String())
	}
	if w := req(t, s, "GET", "/api/state/runs", nil); strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatalf("check persisted evidence: %s", w.Body.String())
	}
	w = req(t, s, "POST", "/api/state/runs", example)
	var first store.StateRunRecord
	if w.Code != 201 || json.Unmarshal(w.Body.Bytes(), &first) != nil || first.ID == "" || first.CreatedAt == "" || !reflect.DeepEqual(first.Result, checked.Result) || !reflect.DeepEqual(first.Machine, example.Machine) || !reflect.DeepEqual(first.Scenario, example.Scenario) {
		t.Fatalf("recorded run: %d %s", w.Code, w.Body.String())
	}
	statuses := map[string]bool{}
	for _, step := range first.Result.Steps {
		statuses[step.Status] = true
	}
	for _, status := range []string{"committed", "denied", "replayed", "id_conflict", "revision_conflict", "rolled_back", "restarted"} {
		if !statuses[status] {
			t.Fatalf("missing recorded modeled outcome %s", status)
		}
	}
	w = req(t, s, "POST", "/api/state/runs", example)
	var second store.StateRunRecord
	if w.Code != 201 || json.Unmarshal(w.Body.Bytes(), &second) != nil || second.ID == first.ID || !reflect.DeepEqual(first.Result, second.Result) {
		t.Fatalf("independent runs reused live state: %d %s", w.Code, w.Body.String())
	}
	example.Scenario.Expected[0].Revision++
	w = req(t, s, "POST", "/api/state/runs", example)
	var failed store.StateRunRecord
	if w.Code != 201 || json.Unmarshal(w.Body.Bytes(), &failed) != nil || !failed.Result.Verified || failed.Result.Passed {
		t.Fatalf("failed expectations should be evidence: %d %s", w.Code, w.Body.String())
	}
	example.Scenario.Expected = []model.StateExpectation{}
	w = req(t, s, "POST", "/api/state/runs", example)
	var unverified store.StateRunRecord
	if w.Code != 201 || json.Unmarshal(w.Body.Bytes(), &unverified) != nil || unverified.Result.Verified || unverified.Result.Passed {
		t.Fatalf("empty expectations must remain unverified: %d %s", w.Code, w.Body.String())
	}
	example.Machine.Machine.Name = "Edited draft"
	example.Scenario.Name = "Edited scenario"
	w = req(t, s, "GET", "/api/state/runs/"+first.ID, nil)
	var detail store.StateRunRecord
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &detail) != nil || !reflect.DeepEqual(first, detail) {
		t.Fatalf("prior evidence mutated: %d %s", w.Code, w.Body.String())
	}
	w = req(t, s, "GET", "/api/state/runs", nil)
	var history []store.StateRunSummary
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &history) != nil || len(history) != 4 || history[0].ID != unverified.ID || history[0].Verified || history[0].Passed || history[1].ID != failed.ID || !history[1].Verified || history[1].Passed || history[3].MachineID != first.Machine.Machine.ID || history[3].MachineVersion != first.Machine.Machine.Version || history[3].StepCount != len(first.Result.Steps) || strings.Contains(w.Body.String(), `"scenario":`) || strings.Contains(w.Body.String(), `"instances":`) {
		t.Fatalf("history summary: %d %s", w.Code, w.Body.String())
	}
	for _, path := range []string{"/api/state/machines", "/api/components", "/api/data/contracts"} {
		if w := req(t, s, "GET", path, nil); strings.TrimSpace(w.Body.String()) != "[]" {
			t.Fatalf("portable execution installed at %s: %s", path, w.Body.String())
		}
	}
}

func TestStateHTTPOrdinaryEvaluationErrorIsRecordable(t *testing.T) {
	s := testServer(t)
	example := stateHTTPExample(t)
	maximum := model.MaxBehaviorInteger
	example.Scenario.Instances = example.Scenario.Instances[:1]
	example.Scenario.Instances[0].Initial["acceptedSinceRestart"] = model.TypedValue{Type: "integer", Integer: &maximum}
	example.Scenario.Steps = example.Scenario.Steps[:1]
	example.Scenario.Expected = []model.StateExpectation{}
	w := req(t, s, "POST", "/api/state/runs", example)
	var record store.StateRunRecord
	if w.Code != 201 || json.Unmarshal(w.Body.Bytes(), &record) != nil || len(record.Result.Steps) != 1 || record.Result.Steps[0].Status != "evaluation_error" || record.Result.Instances[0].Revision != 0 || len(record.Result.Instances[0].Receipts) != 0 || len(record.Result.Instances[0].Effects) != 0 {
		t.Fatalf("ordinary error must retain complete no-commit evidence: %d %s", w.Code, w.Body.String())
	}
}

func TestStateHTTPInvalidOrAbortedRunsDoNotPersist(t *testing.T) {
	s := testServer(t)
	example := stateHTTPExample(t)
	tampered := example
	tampered.Machine.Digest = strings.Repeat("0", 64)
	if w := req(t, s, "POST", "/api/state/runs", tampered); w.Code != 422 {
		t.Fatalf("bad lock: %d %s", w.Code, w.Body.String())
	}
	bad := stateHTTPExample(t)
	bad.Scenario.Steps[0].ExpectedRevision = nil
	if w := req(t, s, "POST", "/api/state/runs", bad); w.Code != 422 {
		t.Fatalf("missing expected revision: %d %s", w.Code, w.Body.String())
	}
	bad = stateHTTPExample(t)
	unsafe := model.MaxBehaviorInteger + 1
	bad.Scenario.Steps[0].ExpectedRevision = &unsafe
	if w := req(t, s, "POST", "/api/state/runs", bad); w.Code != 422 {
		t.Fatalf("unsafe revision: %d %s", w.Code, w.Body.String())
	}
	bad = stateHTTPExample(t)
	bad.Scenario.Steps = make([]model.StateStep, 65)
	if w := req(t, s, "POST", "/api/state/runs", bad); w.Code != 422 {
		t.Fatalf("step limit: %d %s", w.Code, w.Body.String())
	}
	// A valid bounded scenario can still exceed the outer result budget. Large
	// owned strings amplify before/after snapshots even without reducer calls.
	large := stateHTTPExample(t)
	text := strings.Repeat("x", 4096)
	for i := 0; i < 4; i++ {
		large.Machine.Machine.State = append(large.Machine.Machine.State, model.StateSlot{Name: fmt.Sprintf("extra%d", i), Type: "string", Durability: "durable", Initial: model.TypedValue{Type: "string", String: &text}})
	}
	var err error
	large.Machine.Digest, err = model.StateMachineDigest(large.Machine.Machine)
	if err != nil {
		t.Fatal(err)
	}
	large.Scenario.Instances = large.Scenario.Instances[:1]
	large.Scenario.Expected = []model.StateExpectation{}
	large.Scenario.Steps = make([]model.StateStep, 64)
	for i := range large.Scenario.Steps {
		large.Scenario.Steps[i] = model.StateStep{Kind: "restart", InstanceID: large.Scenario.Instances[0].ID}
	}
	if issues := model.ValidateStateScenario(large.Machine, large.Scenario); len(issues) > 0 {
		t.Fatalf("outer-budget fixture must be valid: %#v", issues)
	}
	if w := req(t, s, "POST", "/api/state/runs", large); w.Code != 422 {
		t.Fatalf("result budget: %d %s", w.Code, w.Body.String())
	}
	for _, path := range []string{"/api/state/check", "/api/state/runs"} {
		s.runs <- struct{}{}
		s.runs <- struct{}{}
		w := req(t, s, "POST", path, example)
		<-s.runs
		<-s.runs
		if w.Code != 429 {
			t.Fatalf("shared admission %s: %d %s", path, w.Code, w.Body.String())
		}
		raw, err := json.Marshal(example)
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
	if w := req(t, s, "GET", "/api/state/runs", nil); strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatalf("failed execution saved partial evidence: %s", w.Body.String())
	}
	for _, path := range []string{"/api/state/runs/missing", "/api/state/runs/a/b"} {
		if w := req(t, s, "GET", path, nil); w.Code != 404 {
			t.Fatalf("missing %s: %d", path, w.Code)
		}
	}
}

func TestStateHTTPBoundaryPolicies(t *testing.T) {
	s := testServer(t)
	for _, path := range []string{"/api/state/prepare", "/api/state/validate-lock", "/api/state/machines", "/api/state/check", "/api/state/runs"} {
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
		if path != "/api/state/machines" && path != "/api/state/runs" {
			if w := req(t, s, "GET", path, nil); w.Code != http.StatusMethodNotAllowed {
				t.Fatalf("GET mutation %s: %d", path, w.Code)
			}
		}
	}
	if w := rawDataRequest(s, "/api/state/prepare", `{"name":"`+strings.Repeat("x", MaxBodyBytes)+`"}`); w.Code != 413 {
		t.Fatalf("HTTP body limit: %d %s", w.Code, w.Body.String())
	}
	for _, path := range []string{"/api/state/examples", "/api/state/machines", "/api/state/machine", "/api/state/runs", "/api/state/runs/state-1"} {
		if w := req(t, s, "DELETE", path, nil); w.Code != http.StatusMethodNotAllowed {
			t.Fatalf("DELETE immutable endpoint %s: %d", path, w.Code)
		}
	}
}

func TestStateHTTPAdmissionIncludesValidation(t *testing.T) {
	s := testServer(t)
	for _, path := range []string{"/api/state/check", "/api/state/runs"} {
		// This is decodable but semantically invalid. A saturated server must
		// reject it before traversing the machine/scenario definitions.
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
	if w := req(t, s, "GET", "/api/state/runs", nil); strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatalf("rejected validation saved evidence: %s", w.Body.String())
	}
}

func TestStateHTTPExpectationRawFields(t *testing.T) {
	s := testServer(t)
	example := stateHTTPExample(t)
	example.Scenario.Instances = example.Scenario.Instances[:1]
	example.Scenario.Steps = []model.StateStep{{Kind: "restart", InstanceID: example.Scenario.Instances[0].ID}}
	example.Scenario.Expected = []model.StateExpectation{{InstanceID: example.Scenario.Instances[0].ID, State: example.Scenario.Instances[0].Initial, Revision: 0, Receipts: 0, Effects: 0}}
	// Change only raw expectation members, keeping the rest of the portable
	// fixture exact. Required counters must not silently decode null as zero.
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
		var expected []map[string]json.RawMessage
		if err := json.Unmarshal(scenario["expected"], &expected); err != nil {
			t.Fatal(err)
		}
		change(expected[0])
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
	for _, field := range []string{"revision", "receipts", "effects"} {
		for _, missing := range []bool{false, true} {
			raw := rawExpectation(func(expected map[string]json.RawMessage) {
				if missing {
					delete(expected, field)
				} else {
					expected[field] = json.RawMessage("null")
				}
			})
			for _, path := range []string{"/api/state/check", "/api/state/runs"} {
				if w := rawDataRequest(s, path, raw); w.Code != 400 {
					t.Fatalf("%s %s missing=%v: want400 got%d %s", path, field, missing, w.Code, w.Body.String())
				}
			}
		}
	}
	unknown := rawExpectation(func(expected map[string]json.RawMessage) { expected["unknown"] = json.RawMessage("0") })
	if w := rawDataRequest(s, "/api/state/runs", unknown); w.Code != 400 {
		t.Fatalf("unknown expectation member: %d %s", w.Code, w.Body.String())
	}
	if w := req(t, s, "GET", "/api/state/runs", nil); strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatalf("malformed expectations saved evidence: %s", w.Body.String())
	}
	canonical := rawExpectation(func(expected map[string]json.RawMessage) {
		for field, mixed := range map[string]string{"revision": "ReViSiOn", "receipts": "ReCeIpTs", "effects": "EfFeCtS"} {
			expected[mixed] = expected[field]
			delete(expected, field)
		}
	})
	w := rawDataRequest(s, "/api/state/check", canonical)
	var checked struct {
		Scenario model.StateScenario `json:"scenario"`
		Result   state.Result        `json:"result"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &checked) != nil || !checked.Result.Passed || !reflect.DeepEqual(checked.Scenario, example.Scenario) || strings.Contains(w.Body.String(), "ReViSiOn") {
		t.Fatalf("explicit zero/mixed-case expectations: %d %s", w.Code, w.Body.String())
	}
}
