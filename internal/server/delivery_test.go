package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"software-studio/internal/delivery"
	"software-studio/internal/model"
	"software-studio/internal/store"
)

func deliveryHTTPExample(t *testing.T) model.DeliveryExample {
	t.Helper()
	examples := model.ExampleDeliveryScenarios()
	if len(examples) != 2 {
		t.Fatalf("want two delivery examples, got %d", len(examples))
	}
	if issues := model.ValidateDeliveryScenario(examples[0].Model, examples[0].Scenario); len(issues) > 0 {
		t.Fatal(issues)
	}
	return examples[0]
}

func TestDeliveryHTTPExamplesPrepareAndImmutableCatalog(t *testing.T) {
	s := testServer(t)
	example := deliveryHTTPExample(t)
	lock := example.Model
	w := req(t, s, "GET", "/api/health", nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"version":"0.7.1"`) {
		t.Fatalf("health: %d %s", w.Code, w.Body.String())
	}
	w = req(t, s, "GET", "/api/delivery/examples", nil)
	var examples []model.DeliveryExample
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &examples) != nil || len(examples) != 2 || examples[0].Model.Digest != examples[1].Model.Digest {
		t.Fatalf("examples: %d %s", w.Code, w.Body.String())
	}
	for _, e := range examples {
		if issues := model.ValidateDeliveryScenario(e.Model, e.Scenario); len(issues) > 0 {
			t.Fatal(issues)
		}
	}
	if w := req(t, s, "GET", "/api/delivery/models", nil); w.Code != 200 || strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatalf("empty catalog: %d %s", w.Code, w.Body.String())
	}
	w = req(t, s, "POST", "/api/delivery/prepare", lock.Model)
	var prepared model.DeliveryModelLock
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &prepared) != nil || !reflect.DeepEqual(prepared, lock) {
		t.Fatalf("prepare: %d %s", w.Code, w.Body.String())
	}
	raw, err := json.Marshal(lock)
	if err != nil {
		t.Fatal(err)
	}
	w = rawDataRequest(s, "/api/delivery/validate-lock", strings.ReplaceAll(strings.ReplaceAll(string(raw), `"model":`, `"MODEL":`), `"schemaVersion":`, `"SCHEMAVERSION":`))
	var validated struct {
		Valid  bool                    `json:"valid"`
		Issues []model.ValidationIssue `json:"issues"`
		Model  model.DeliveryModelLock `json:"model"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &validated) != nil || !validated.Valid || len(validated.Issues) != 0 || !reflect.DeepEqual(validated.Model, lock) || strings.Contains(w.Body.String(), "SCHEMAVERSION") {
		t.Fatalf("canonical lock: %d %s", w.Code, w.Body.String())
	}
	tampered := lock
	tampered.Digest = strings.Repeat("0", 64)
	if w := req(t, s, "POST", "/api/delivery/validate-lock", tampered); w.Code != 200 || !strings.Contains(w.Body.String(), `"valid":false`) || strings.Contains(w.Body.String(), `"model":`) {
		t.Fatalf("invalid lock verdict: %d %s", w.Code, w.Body.String())
	}
	if w := req(t, s, "POST", "/api/delivery/models", tampered); w.Code != 422 {
		t.Fatalf("invalid install: %d %s", w.Code, w.Body.String())
	}
	w = req(t, s, "POST", "/api/delivery/models", lock)
	var installed store.DeliveryModelRecord
	if w.Code != 201 || json.Unmarshal(w.Body.Bytes(), &installed) != nil || installed.Digest != lock.Digest || installed.InstalledAt == "" {
		t.Fatalf("install: %d %s", w.Code, w.Body.String())
	}
	w = req(t, s, "POST", "/api/delivery/models", lock)
	var again store.DeliveryModelRecord
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &again) != nil || !reflect.DeepEqual(again, installed) {
		t.Fatalf("reinstall: %d %s", w.Code, w.Body.String())
	}
	changed := lock
	changed.Model.Description = "Changed metadata"
	changed.Digest, err = model.DeliveryModelDigest(changed.Model)
	if err != nil {
		t.Fatal(err)
	}
	if w := req(t, s, "POST", "/api/delivery/models", changed); w.Code != 409 {
		t.Fatalf("immutable conflict: %d %s", w.Code, w.Body.String())
	}
	query := url.Values{"id": {lock.Model.ID}, "version": {lock.Model.Version}, "digest": {lock.Digest}}
	w = req(t, s, "GET", "/api/delivery/model?"+query.Encode(), nil)
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &again) != nil || !reflect.DeepEqual(again, installed) {
		t.Fatalf("exact retrieval: %d %s", w.Code, w.Body.String())
	}
	query.Set("digest", strings.Repeat("0", 64))
	if w := req(t, s, "GET", "/api/delivery/model?"+query.Encode(), nil); w.Code != 404 {
		t.Fatalf("wrong digest: %d %s", w.Code, w.Body.String())
	}
	if w := req(t, s, "GET", "/api/delivery/model?id=bad&version=latest&digest=no", nil); w.Code != 422 {
		t.Fatalf("invalid reference: %d %s", w.Code, w.Body.String())
	}
	for _, path := range []string{"/api/state/machines", "/api/components", "/api/data/contracts"} {
		if w := req(t, s, "GET", path, nil); strings.TrimSpace(w.Body.String()) != "[]" {
			t.Fatalf("definition installed dependencies at %s: %s", path, w.Body.String())
		}
	}
}

func TestDeliveryHTTPChecksRecordedOutcomesAndIndependentRuns(t *testing.T) {
	s := testServer(t)
	example := deliveryHTTPExample(t)
	raw, err := json.Marshal(example)
	if err != nil {
		t.Fatal(err)
	}
	w := rawDataRequest(s, "/api/delivery/check", strings.ReplaceAll(strings.ReplaceAll(string(raw), `"instanceId":`, `"INSTANCEID":`), `"integer":`, `"INTEGER":`))
	var checked struct {
		Scenario model.DeliveryScenario `json:"scenario"`
		Result   delivery.Result        `json:"result"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &checked) != nil || !checked.Result.Verified || !checked.Result.Passed || checked.Result.EngineVersion != "0.7.0" || !reflect.DeepEqual(checked.Scenario, example.Scenario) || strings.Contains(w.Body.String(), "INSTANCEID") {
		t.Fatalf("canonical check: %d %s", w.Code, w.Body.String())
	}
	if w := req(t, s, "GET", "/api/delivery/runs", nil); strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatalf("check persisted evidence: %s", w.Body.String())
	}
	w = req(t, s, "POST", "/api/delivery/runs", example)
	var first store.DeliveryRunRecord
	if w.Code != 201 || json.Unmarshal(w.Body.Bytes(), &first) != nil || first.ID == "" || first.CreatedAt == "" || !reflect.DeepEqual(first.Result, checked.Result) || !reflect.DeepEqual(first.Model, example.Model) || !reflect.DeepEqual(first.Scenario, example.Scenario) {
		t.Fatalf("recorded run: %d %s", w.Code, w.Body.String())
	}
	statuses := map[string]bool{}
	for _, step := range first.Result.Steps {
		statuses[step.Status] = true
	}
	for _, status := range []string{"dispatched", "dropped", "recipient_unacknowledged", "acknowledgement_lost", "acknowledged", "dispatch_replayed"} {
		if !statuses[status] {
			t.Fatalf("missing recorded modeled outcome %s", status)
		}
	}
	w = req(t, s, "POST", "/api/delivery/runs", example)
	var second store.DeliveryRunRecord
	if w.Code != 201 || json.Unmarshal(w.Body.Bytes(), &second) != nil || second.ID == first.ID || !reflect.DeepEqual(first.Result, second.Result) {
		t.Fatalf("independent runs reused live delivery: %d %s", w.Code, w.Body.String())
	}
	example.Scenario.Expected[0].Revision++
	w = req(t, s, "POST", "/api/delivery/runs", example)
	var failed store.DeliveryRunRecord
	if w.Code != 201 || json.Unmarshal(w.Body.Bytes(), &failed) != nil || !failed.Result.Verified || failed.Result.Passed {
		t.Fatalf("failed expectations should be evidence: %d %s", w.Code, w.Body.String())
	}
	example.Scenario.Expected = []model.StateExpectation{}
	example.Scenario.ExpectedMessages = []model.DeliveryMessageExpectation{}
	w = req(t, s, "POST", "/api/delivery/runs", example)
	var unverified store.DeliveryRunRecord
	if w.Code != 201 || json.Unmarshal(w.Body.Bytes(), &unverified) != nil || unverified.Result.Verified || unverified.Result.Passed {
		t.Fatalf("empty expectations must remain unverified: %d %s", w.Code, w.Body.String())
	}
	example.Model.Model.Name = "Edited draft"
	example.Scenario.Name = "Edited scenario"
	w = req(t, s, "GET", "/api/delivery/runs/"+first.ID, nil)
	var detail store.DeliveryRunRecord
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &detail) != nil || !reflect.DeepEqual(first, detail) {
		t.Fatalf("prior evidence mutated: %d %s", w.Code, w.Body.String())
	}
	w = req(t, s, "GET", "/api/delivery/runs", nil)
	var history []store.DeliveryRunSummary
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &history) != nil || len(history) != 4 || history[0].ID != unverified.ID || history[0].Verified || history[0].Passed || history[1].ID != failed.ID || !history[1].Verified || history[1].Passed || history[3].ModelID != first.Model.Model.ID || history[3].ModelVersion != first.Model.Model.Version || history[3].StepCount != len(first.Result.Steps) || history[3].MessageCount != len(first.Result.Messages) || strings.Contains(w.Body.String(), `"scenario":`) || strings.Contains(w.Body.String(), `"instances":`) {
		t.Fatalf("history summary: %d %s", w.Code, w.Body.String())
	}
	for _, path := range []string{"/api/delivery/models", "/api/state/machines", "/api/components", "/api/data/contracts"} {
		if w := req(t, s, "GET", path, nil); strings.TrimSpace(w.Body.String()) != "[]" {
			t.Fatalf("portable execution installed at %s: %s", path, w.Body.String())
		}
	}
}

func TestDeliveryHTTPBoundaryPolicies(t *testing.T) {
	s := testServer(t)
	for _, path := range []string{"/api/delivery/prepare", "/api/delivery/validate-lock", "/api/delivery/models", "/api/delivery/check", "/api/delivery/runs"} {
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
		if path != "/api/delivery/models" && path != "/api/delivery/runs" {
			if w := req(t, s, "GET", path, nil); w.Code != http.StatusMethodNotAllowed {
				t.Fatalf("GET mutation %s: %d", path, w.Code)
			}
		}
	}
	if w := rawDataRequest(s, "/api/delivery/prepare", `{"name":"`+strings.Repeat("x", MaxBodyBytes)+`"}`); w.Code != 413 {
		t.Fatalf("HTTP body limit: %d %s", w.Code, w.Body.String())
	}
	for _, path := range []string{"/api/delivery/examples", "/api/delivery/models", "/api/delivery/model", "/api/delivery/runs", "/api/delivery/runs/delivery-1"} {
		if w := req(t, s, "DELETE", path, nil); w.Code != http.StatusMethodNotAllowed {
			t.Fatalf("DELETE immutable endpoint %s: %d", path, w.Code)
		}
	}
}

func TestDeliveryHTTPAdmissionIncludesValidation(t *testing.T) {
	s := testServer(t)
	for _, path := range []string{"/api/delivery/check", "/api/delivery/runs"} {
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
	if w := req(t, s, "GET", "/api/delivery/runs", nil); strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatalf("rejected validation saved evidence: %s", w.Body.String())
	}
}

func TestDeliveryHTTPMissingIntentIsRecordable(t *testing.T) {
	s := testServer(t)
	example := deliveryHTTPExample(t)
	var dispatch *model.DeliveryStep
	for i := range example.Scenario.Steps {
		step := example.Scenario.Steps[i]
		if step.Kind == "dispatch" && step.SourceCommandID != "" {
			dispatch = &step
			break
		}
	}
	if dispatch == nil {
		t.Fatal("example must contain a direct-source dispatch")
	}
	example.Scenario.Steps = []model.DeliveryStep{*dispatch, {Kind: "deliver", Message: dispatch.Message}}
	example.Scenario.Expected = []model.StateExpectation{}
	example.Scenario.ExpectedMessages = []model.DeliveryMessageExpectation{{Name: dispatch.Message, Status: "absent", Attempts: 0}}
	w := req(t, s, "POST", "/api/delivery/runs", example)
	var record store.DeliveryRunRecord
	if w.Code != 201 || json.Unmarshal(w.Body.Bytes(), &record) != nil || !record.Result.Passed || len(record.Result.Messages) != 0 || len(record.Result.Steps) != 2 || record.Result.Steps[0].Status != "missing_intent" || record.Result.Steps[1].Status != "missing_message" {
		t.Fatalf("modeled missing-intent evidence: %d %s", w.Code, w.Body.String())
	}
}

func TestDeliveryHTTPInvalidOrOversizedRunsDoNotPersist(t *testing.T) {
	s := testServer(t)
	example := deliveryHTTPExample(t)
	tampered := example
	tampered.Model.Digest = strings.Repeat("0", 64)
	if w := req(t, s, "POST", "/api/delivery/runs", tampered); w.Code != 422 {
		t.Fatalf("bad model lock: %d %s", w.Code, w.Body.String())
	}
	bad := deliveryHTTPExample(t)
	bad.Scenario.Initial = []model.DeliveryInitial{}
	if w := req(t, s, "POST", "/api/delivery/runs", bad); w.Code != 422 {
		t.Fatalf("missing initial overrides: %d %s", w.Code, w.Body.String())
	}
	bad = deliveryHTTPExample(t)
	bad.Scenario.Steps = make([]model.DeliveryStep, 65)
	if w := req(t, s, "POST", "/api/delivery/runs", bad); w.Code != 422 {
		t.Fatalf("step limit: %d %s", w.Code, w.Body.String())
	}
	bad = deliveryHTTPExample(t)
	unsafe := model.MaxBehaviorInteger + 1
	for i := range bad.Scenario.Steps {
		if bad.Scenario.Steps[i].Kind == "dispatch" {
			bad.Scenario.Steps[i].ExpectedRevision = &unsafe
			break
		}
	}
	if w := req(t, s, "POST", "/api/delivery/runs", bad); w.Code != 422 {
		t.Fatalf("unsafe dispatch revision: %d %s", w.Code, w.Body.String())
	}
	// These valid owned values amplify restart snapshots beyond the complete
	// delivery result budget, which must abort before any evidence is saved.
	large := deliveryHTTPExample(t)
	sourceID := large.Model.Model.Routes[0].SourceInstanceID
	var sourceRef model.DeliveryMachineRef
	for _, instance := range large.Model.Model.Instances {
		if instance.ID == sourceID {
			sourceRef = instance.Machine
		}
	}
	for i := range large.Model.Model.Machines {
		lock := &large.Model.Model.Machines[i]
		if lock.Machine.ID != sourceRef.MachineID || lock.Machine.Version != sourceRef.Version {
			continue
		}
		if len(lock.Machine.State) > 4 {
			t.Fatal("source fixture must leave four slots for budget test")
		}
		text := strings.Repeat("x", 4096)
		for n := 0; n < 4; n++ {
			lock.Machine.State = append(lock.Machine.State, model.StateSlot{Name: fmt.Sprintf("extra%d", n), Type: "string", Durability: "durable", Initial: model.TypedValue{Type: "string", String: &text}})
		}
		var err error
		lock.Digest, err = model.StateMachineDigest(lock.Machine)
		if err != nil {
			t.Fatal(err)
		}
		for j := range large.Model.Model.Instances {
			ref := &large.Model.Model.Instances[j].Machine
			if ref.MachineID == lock.Machine.ID && ref.Version == lock.Machine.Version {
				ref.Digest = lock.Digest
			}
		}
	}
	var err error
	large.Model.Digest, err = model.DeliveryModelDigest(large.Model.Model)
	if err != nil {
		t.Fatal(err)
	}
	large.Scenario.Steps = make([]model.DeliveryStep, 64)
	for i := range large.Scenario.Steps {
		large.Scenario.Steps[i] = model.DeliveryStep{Kind: "restart", InstanceID: sourceID}
	}
	large.Scenario.Expected = []model.StateExpectation{}
	large.Scenario.ExpectedMessages = []model.DeliveryMessageExpectation{}
	if issues := model.ValidateDeliveryScenario(large.Model, large.Scenario); len(issues) > 0 {
		t.Fatalf("outer-budget scenario must validate: %#v", issues)
	}
	if w := req(t, s, "POST", "/api/delivery/runs", large); w.Code != 422 {
		t.Fatalf("outer evidence budget: %d %s", w.Code, w.Body.String())
	}
	if w := req(t, s, "GET", "/api/delivery/runs", nil); strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatalf("failed execution saved partial evidence: %s", w.Body.String())
	}
	for _, path := range []string{"/api/delivery/runs/missing", "/api/delivery/runs/a/b"} {
		if w := req(t, s, "GET", path, nil); w.Code != 404 {
			t.Fatalf("missing evidence %s: %d", path, w.Code)
		}
	}
}

func TestDeliveryHTTPMessageExpectationRawFields(t *testing.T) {
	s := testServer(t)
	example := deliveryHTTPExample(t)
	example.Scenario.Steps = []model.DeliveryStep{{Kind: "restart", InstanceID: example.Model.Model.Instances[0].ID}}
	example.Scenario.Expected = []model.StateExpectation{}
	example.Scenario.ExpectedMessages = []model.DeliveryMessageExpectation{{Name: "absentMessage", Status: "absent", Attempts: 0}}
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
		if err := json.Unmarshal(scenario["expectedMessages"], &expected); err != nil {
			t.Fatal(err)
		}
		change(expected[0])
		scenario["expectedMessages"], err = json.Marshal(expected)
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
	for _, value := range []string{"", "null", "1.5"} {
		raw := rawExpectation(func(expected map[string]json.RawMessage) {
			if value == "" {
				delete(expected, "attempts")
			} else {
				expected["attempts"] = json.RawMessage(value)
			}
		})
		for _, path := range []string{"/api/delivery/check", "/api/delivery/runs"} {
			if w := rawDataRequest(s, path, raw); w.Code != 400 {
				t.Fatalf("%s attempts=%q want400 got%d %s", path, value, w.Code, w.Body.String())
			}
		}
	}
	unknown := rawExpectation(func(expected map[string]json.RawMessage) { expected["unknown"] = json.RawMessage("0") })
	if w := rawDataRequest(s, "/api/delivery/runs", unknown); w.Code != 400 {
		t.Fatalf("unknown expectation member: %d %s", w.Code, w.Body.String())
	}
	unsafe := rawExpectation(func(expected map[string]json.RawMessage) { expected["attempts"] = json.RawMessage("9007199254740992") })
	if w := rawDataRequest(s, "/api/delivery/runs", unsafe); w.Code != 422 {
		t.Fatalf("unsafe attempts: %d %s", w.Code, w.Body.String())
	}
	if w := req(t, s, "GET", "/api/delivery/runs", nil); strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatalf("invalid expectations saved evidence: %s", w.Body.String())
	}
	mixed := rawExpectation(func(expected map[string]json.RawMessage) {
		expected["AtTeMpTs"] = expected["attempts"]
		delete(expected, "attempts")
	})
	w := rawDataRequest(s, "/api/delivery/check", mixed)
	var checked struct {
		Scenario model.DeliveryScenario `json:"scenario"`
		Result   delivery.Result        `json:"result"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &checked) != nil || !checked.Result.Verified || !checked.Result.Passed || !reflect.DeepEqual(checked.Scenario, example.Scenario) || strings.Contains(w.Body.String(), "AtTeMpTs") {
		t.Fatalf("explicit zero/mixed-case message expectation: %d %s", w.Code, w.Body.String())
	}
}
