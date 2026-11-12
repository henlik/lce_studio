package model_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"software-studio/internal/model"
	"software-studio/internal/state"
	"software-studio/internal/workflow"
)

func receptionTestText(s string) model.TypedValue {
	return model.TypedValue{Type: "string", String: &s}
}
func receptionTestInt(i int64) model.TypedValue {
	return model.TypedValue{Type: "integer", Integer: &i}
}
func receptionTestBool(b bool) model.TypedValue {
	return model.TypedValue{Type: "boolean", Boolean: &b}
}
func TestReceptionEmergencyInterruptAtEveryRoutineTask(t *testing.T) {
	examples := model.ExampleWorkflowScenarios()
	normal := examples[0]
	emergency := examples[6]
	for completed := 0; completed < 11; completed++ {
		t.Run(normal.Workflow.Workflow.Nodes[0].ID+string(rune('A'+completed)), func(t *testing.T) {
			scenario := normal.Scenario
			scenario.Expected = nil
			scenario.Events = append([]model.WorkflowEvent{}, normal.Scenario.Events[:completed]...)
			scenario.Events = append(scenario.Events, emergency.Scenario.Events[0])
			accept := emergency.Scenario.Events[1]
			accept.TaskID = "task" + receptionDecimal(completed+2)
			accept.ID = "ecuAcceptance"
			scenario.Events = append(scenario.Events, accept)
			late := normal.Scenario.Events[completed]
			late.ID = "lateRoutine"
			scenario.Events = append(scenario.Events, late)
			result, err := workflow.Run(context.Background(), normal.Workflow, scenario)
			if err != nil {
				t.Fatal(err)
			}
			if result.Status != "completed" || result.Outcome != "emergencyAccepted" || result.Task != nil {
				t.Fatalf("not accepted: %+v", result)
			}
			if len(result.Tasks) != completed+2 || len(result.Receipts) != completed+3 || result.Tasks[completed].Status != "cancelled" || result.Tasks[completed+1].Status != "completed" {
				t.Fatalf("incorrect fencing/counts %+v", result.Tasks)
			}
			if *result.Variables["routineActive"].Boolean || *result.Variables["clinicalReleased"].Boolean || *result.Variables["reservationHeld"].Boolean {
				t.Fatal("ordinary path resurrected")
			}
			for _, owner := range result.Owners {
				if owner.ID == "doctorA" || owner.ID == "doctorB" {
					if *owner.State["available"].Integer != 1 || *owner.State["holder"].String != "" || *owner.State["accepted"].Boolean {
						t.Fatalf("reservation retained: %+v", owner)
					}
				}
				if owner.ID == "queue" {
					wantRevision := int64(0)
					if completed == 1 {
						wantRevision = 2
					} else if completed >= 2 {
						wantRevision = 3
					}
					if owner.Revision != wantRevision || *owner.State["cancelled"].Boolean != (completed >= 1) {
						t.Fatalf("incorrect queue: %+v", owner)
					}
				}
			}
			if result.Events[len(result.Events)-1].Status == "accepted" {
				t.Fatal("late routine response accepted")
			}
		})
	}
}
func receptionDecimal(n int) string {
	if n < 10 {
		return string(rune('0' + n))
	}
	return string(rune('0'+n/10)) + string(rune('0'+n%10))
}
func TestReceptionSingleSlotRaceAndReceivingAcceptance(t *testing.T) {
	lock := model.ReceptionStateMachines()[2]
	revision := func(n int64) *int64 { return &n }
	step := func(id, command, caseID string, rev int64) model.StateStep {
		inputs := map[string]model.TypedValue{"resourceId": receptionTestText("providerA"), "caseId": receptionTestText(caseID)}
		if command == "reserve" {
			inputs["eligible"] = receptionTestBool(true)
		}
		return model.StateStep{Kind: "command", InstanceID: "slot", CommandID: id, Command: command, ExpectedRevision: revision(rev), Inputs: inputs}
	}
	first := step("reserveA", "reserve", "caseA", 0)
	scenario := model.StateScenario{SchemaVersion: 1, Name: "Independent reservation ownership oracle", Instances: []model.StateInstance{{ID: "slot", Name: "One slot", Initial: map[string]model.TypedValue{"resourceId": receptionTestText("providerA")}}}, Steps: []model.StateStep{first, step("reserveB", "reserve", "caseB", 1), step("acceptA", "accept", "caseA", 1), step("repeatA", "reserve", "caseA", 2), {Kind: "restart", InstanceID: "slot"}, first, step("releaseOther", "release", "caseB", 3), step("releaseA", "release", "caseA", 3)}, Expected: []model.StateExpectation{}}
	result, err := state.Run(context.Background(), lock, scenario)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"committed", "denied", "committed", "committed", "restarted", "replayed", "denied", "committed"}
	for i, status := range want {
		if result.Steps[i].Status != status {
			t.Fatalf("step%d: %s != %s", i, result.Steps[i].Status, status)
		}
	}
	if !*result.Steps[3].After["accepted"].Boolean || !*result.Steps[4].After["accepted"].Boolean || !*result.Steps[5].After["accepted"].Boolean {
		t.Fatal("repeat/restart/replay undid receiving acceptance")
	}
	denied := result.Steps[1]
	if denied.Receipt == nil || denied.Receipt.Accepted || *denied.Receipt.Outputs["held"].Boolean || *denied.Receipt.Outputs["received"].Boolean || *denied.Receipt.Outputs["reservationToken"].String != "" {
		t.Fatal("denied competing case was reported assigned")
	}
	final := result.Instances[0]
	if final.Revision != 4 || len(final.Receipts) != 6 || *final.State["available"].Integer != 1 || *final.State["holder"].String != "" || *final.State["accepted"].Boolean {
		t.Fatalf("incorrect final slot: %+v", final)
	}
}
func TestReceptionTicketNumericSequenceReuseAndBoundary(t *testing.T) {
	lock := model.ReceptionStateMachines()[0]
	zero, one := int64(0), int64(1)
	inputs := map[string]model.TypedValue{"caseId": receptionTestText("caseA")}
	scenario := model.StateScenario{SchemaVersion: 1, Name: "Ticket number is stable for one case", Instances: []model.StateInstance{{ID: "queue", Name: "Case scoped queue", Initial: map[string]model.TypedValue{"nextNumber": receptionTestInt(41)}}}, Steps: []model.StateStep{{Kind: "command", InstanceID: "queue", CommandID: "first", Command: "issue", ExpectedRevision: &zero, Inputs: inputs}, {Kind: "command", InstanceID: "queue", CommandID: "sameCase", Command: "issue", ExpectedRevision: &one, Inputs: inputs}}, Expected: []model.StateExpectation{}}
	result, err := state.Run(context.Background(), lock, scenario)
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range result.Steps {
		if step.Status != "committed" || *step.Receipt.Outputs["ticketNumber"].Integer != 41 {
			t.Fatal("same case did not reuse numeric ticket")
		}
	}
	if *result.Instances[0].State["nextNumber"].Integer != 42 {
		t.Fatal("semantic retry advanced sequence")
	}
	scenario.Instances[0].Initial["nextNumber"] = receptionTestInt(model.MaxBehaviorInteger)
	scenario.Steps = scenario.Steps[:1]
	result, err = state.Run(context.Background(), lock, scenario)
	if err != nil || result.Steps[0].Status != "denied" || result.Instances[0].Revision != 0 {
		t.Fatal("safe-integer boundary must deny before overflow", err)
	}
}
func TestReceptionSamePatientDistinctVisitsAndPortableParts(t *testing.T) {
	examples := model.ExampleWorkflowScenarios()
	normal := examples[0]
	normal.Scenario.Expected = nil
	normal.Scenario.Inputs["caseId"] = receptionTestText("case-200")
	normal.Scenario.Inputs["arrivalId"] = receptionTestText("arrival-200")
	normal.Scenario.Events[10].Values["caseId"] = receptionTestText("case-200")
	result, err := workflow.Run(context.Background(), normal.Workflow, normal.Scenario)
	if err != nil || result.Outcome != "consultationAccepted" {
		t.Fatal(err)
	}
	if *result.Outputs["patientId"].String != "patient-42" || *result.Outputs["encounterId"].String != "encounter:case-200" || *result.Outputs["ticketId"].String != "ticket:case-200" {
		t.Fatal("patient identity collapsed distinct visits")
	}
	original, err := os.ReadFile("../../examples/composites/reception-entry-route.json")
	if err != nil {
		t.Fatal(err)
	}
	var entry model.ComponentBundle
	if err = json.Unmarshal(original, &entry); err != nil {
		t.Fatal(err)
	}
	if entry.Root.Digest != "a3b1ba4f0fd5faa449a511c9d1f168c8f11d744705f78d23abc43205b22aa1c4" || !reflect.DeepEqual(entry, model.ReceptionDecisionBundles()[0]) {
		t.Fatal("original entry-route changed")
	}
	hospital, desk := model.HospitalReceptionWorkflow(), model.ServiceDeskWorkflow()
	for _, part := range desk.Workflow.Machines {
		found := false
		for _, candidate := range hospital.Workflow.Machines {
			if part.Machine.ID == candidate.Machine.ID {
				found = true
				if !reflect.DeepEqual(part, candidate) {
					t.Fatal("domain reuse changed lower part")
				}
			}
		}
		if !found {
			t.Fatal("reuse owner missing")
		}
	}
	for _, path := range []string{"../../examples/reception/hospital-reception.lock.json", "../../examples/reception/service-desk.lock.json"} {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var lock model.WorkflowLock
		if err = json.Unmarshal(body, &lock); err != nil {
			t.Fatal(err)
		}
		if issues := model.ValidateWorkflowLock(lock); len(issues) > 0 {
			t.Fatal(issues)
		}
		want := hospital
		if lock.Workflow.ID == desk.Workflow.ID {
			want = desk
		}
		if lock.Digest != want.Digest {
			t.Fatalf("stale portable lock %s", path)
		}
	}
	paths, err := filepath.Glob("../../examples/reception/scenarios/*.run.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != len(examples) {
		t.Fatal("portable scenario count")
	}
	fresh := model.ExampleWorkflowScenarios()
	byName := map[string]model.WorkflowExample{}
	for _, example := range fresh {
		byName[example.Scenario.Name] = example
	}
	for _, path := range paths {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var example model.WorkflowExample
		if err = json.Unmarshal(body, &example); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(example, byName[example.Scenario.Name]) {
			t.Fatalf("stale portable example %s", path)
		}
	}
}
