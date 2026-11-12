package model_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"software-studio/internal/behavior"
	"software-studio/internal/model"
	"software-studio/internal/state"
)

func TestStateExamplesReuseExactDefinitionWithIndependentOwnership(t *testing.T) {
	examples := model.ExampleStateScenarios()
	if len(examples) != 2 || !reflect.DeepEqual(examples[0].Machine, examples[1].Machine) {
		t.Fatal("two domains must reuse one exact machine, reducer and data contract")
	}
	if examples[0].Machine.Machine.ID != "lce.resource-reservation" || len(examples[0].Machine.Machine.Commands) != 1 {
		t.Fatal("examples must use one generic resource-reservation command")
	}
	for _, example := range examples {
		if issues := model.ValidateStateMachineLock(example.Machine); len(issues) != 0 {
			t.Fatalf("machine invalid: %+v", issues)
		}
		if issues := model.ValidateStateScenario(example.Machine, example.Scenario); len(issues) != 0 {
			t.Fatalf("scenario invalid: %+v", issues)
		}
	}
	*examples[0].Machine.Machine.State[0].Initial.String = "changed"
	examples[0].Machine.Machine.Commands[0].Decision.Packages[0].Package.Behavior.Nodes[0].Field = "changed"
	*examples[0].Scenario.Instances[0].Initial["available"].Integer = 999
	*examples[0].Scenario.Steps[0].Inputs["request"].Record["quantity"].Integer = 999
	if got := *examples[1].Machine.Machine.State[0].Initial.String; got != "tenantDefault" {
		t.Fatal("machine state aliases across examples")
	}
	if got := examples[1].Machine.Machine.Commands[0].Decision.Packages[0].Package.Behavior.Nodes[0].Field; got != "tenantId" {
		t.Fatal("decision graph aliases across examples")
	}
	if got := *examples[0].Scenario.Steps[2].Inputs["request"].Record["quantity"].Integer; got != 2 {
		t.Fatal("repeated command inputs alias")
	}
	if fresh := model.ExampleStateScenarios()[0]; *fresh.Scenario.Instances[0].Initial["available"].Integer != 5 || fresh.Machine.Machine.Commands[0].Decision.Packages[0].Package.Behavior.Nodes[0].Field != "tenantId" {
		t.Fatal("mutation escaped returned example values")
	}
}

func TestStateExamplePureReservationCasesUseIndependentExpectedValues(t *testing.T) {
	bundle := model.ExampleStateScenarios()[0].Machine.Machine.Commands[0].Decision
	if len(bundle.Packages) != 1 || len(bundle.Packages[0].Package.Tests) != 7 {
		t.Fatal("expected the bounded pure reducer and seven explicit cases")
	}
	for _, test := range bundle.Packages[0].Package.Tests {
		t.Run(test.Name, func(t *testing.T) {
			result, err := behavior.Evaluate(context.Background(), bundle, test.Inputs)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(result.Outputs, test.Expected) {
				t.Fatalf("independent expected outputs differ: got %+v, want %+v", result.Outputs, test.Expected)
			}
			if result.Operations != 21 || result.Invocations != 1 {
				t.Fatalf("unexpected pure work: operations=%d invocations=%d", result.Operations, result.Invocations)
			}
		})
	}
}

func TestStateExampleScenariosMatchHandCalculatedRecoveryEvidence(t *testing.T) {
	wantStatuses := []string{"committed", "committed", "replayed", "denied", "rolled_back", "restarted", "committed", "restarted", "replayed", "committed", "denied", "replayed", "id_conflict", "revision_conflict", "restarted", "replayed", "denied", "committed"}
	for domain, example := range model.ExampleStateScenarios() {
		t.Run(example.Scenario.Name, func(t *testing.T) {
			before, _ := json.Marshal(example)
			result, err := state.Run(context.Background(), example.Machine, example.Scenario)
			if err != nil {
				t.Fatal(err)
			}
			if !result.Verified || !result.Passed || result.Operations != 189 || result.Invocations != 9 {
				t.Fatalf("unexpected reference verdict/work: verified=%v passed=%v operations=%d invocations=%d", result.Verified, result.Passed, result.Operations, result.Invocations)
			}
			if len(result.Steps) != len(wantStatuses) || len(result.Instances) != 2 {
				t.Fatal("missing scenario evidence")
			}
			for i, status := range wantStatuses {
				if result.Steps[i].Status != status {
					t.Fatalf("step %d: got %s, want %s", i, result.Steps[i].Status, status)
				}
			}
			primary, partner := result.Instances[0], result.Instances[1]
			if primary.ID != "primary" || *primary.State["available"].Integer != 0 || *primary.State["acceptedSinceRestart"].Integer != 1 || primary.Revision != 3 || len(primary.Receipts) != 5 || len(primary.Effects) != 3 {
				t.Fatalf("primary disagrees with manual final 0/1, rev3, receipts5, effects3: %+v", primary)
			}
			if partner.ID != "partner" || *partner.State["available"].Integer != int64(1-domain) || *partner.State["acceptedSinceRestart"].Integer != 11 || partner.Revision != 2 || len(partner.Receipts) != 3 || len(partner.Effects) != 2 {
				t.Fatalf("partner disagrees with manual final stock/counter, rev2, receipts3, effects2: %+v", partner)
			}
			rollback := result.Steps[4]
			if !rollback.FaultApplied || rollback.Receipt != nil || rollback.BeforeRevision != 1 || rollback.AfterRevision != 1 || *rollback.Before["acceptedSinceRestart"].Integer != 1 || *rollback.After["acceptedSinceRestart"].Integer != 1 {
				t.Fatal("before_commit must roll back without implicitly resetting volatile state")
			}
			lost := result.Steps[6]
			if !lost.ReplyLost || !lost.FaultApplied || lost.Receipt == nil || lost.AfterRevision != 2 || *lost.After["available"].Integer != 2 {
				t.Fatal("lost reply must retain the committed transition")
			}
			for _, i := range []int{2, 8} {
				if result.Steps[i].FaultApplied || result.Steps[i].ReplyLost || result.Steps[i].Decision != nil {
					t.Fatalf("replay at step %d must bypass evaluation and commit fault injection", i)
				}
			}
			cached := result.Steps[8]
			if cached.Receipt == nil || *cached.Receipt.Outputs["acceptedSinceRestart"].Integer != 1 || *cached.After["acceptedSinceRestart"].Integer != 0 {
				t.Fatal("replay must distinguish original volatile output from restarted current state")
			}
			denied := result.Steps[11]
			if denied.Receipt == nil || denied.Receipt.Accepted || denied.Receipt.Revision != 1 || *denied.Receipt.Outputs["remaining"].Integer != 3 || *denied.After["available"].Integer != 0 || denied.AfterRevision != 3 {
				t.Fatal("denied replay must retain historical remaining=3 without restoring it")
			}
			partnerReplay := result.Steps[15]
			if partnerReplay.Receipt == nil || *partnerReplay.Receipt.Outputs["acceptedSinceRestart"].Integer != 11 || *partnerReplay.After["acceptedSinceRestart"].Integer != 10 {
				t.Fatal("partner restart must use its overridden volatile baseline")
			}
			for _, instance := range result.Instances {
				for _, effect := range instance.Effects {
					encoded, _ := json.Marshal([]string{instance.ID, effect.CommandID, "reservation"})
					hash := sha256.Sum256(encoded)
					if effect.ID != hex.EncodeToString(hash[:]) || effect.Effect != "reservation" || effect.InstanceID != instance.ID {
						t.Fatal("effect identity must be derived from instance, command and effect name")
					}
				}
			}
			again, err := state.Run(context.Background(), example.Machine, example.Scenario)
			if err != nil || !reflect.DeepEqual(result, again) {
				t.Fatal("independent runs must restart from declared initial state and reproduce all evidence")
			}
			after, _ := json.Marshal(example)
			if !bytes.Equal(before, after) {
				t.Fatal("running examples mutated their definitions, steps or expectations")
			}
		})
	}
}

func TestStatePortableExamplesMatchCanonicalDefinitions(t *testing.T) {
	examples := model.ExampleStateScenarios()
	for i, path := range []string{"../../examples/state/pharmacy-reservation.run.json", "../../examples/state/library-reservation.run.json"} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var portable model.StateExample
		dec := json.NewDecoder(bytes.NewReader(data))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&portable); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(portable, examples[i]) {
			t.Fatalf("portable example is stale: %s", path)
		}
	}
	for _, file := range []struct {
		path string
		lock bool
	}{{"../../examples/state/resource-reservation.machine.json", false}, {"../../examples/state/resource-reservation.lock.json", true}} {
		data, err := os.ReadFile(file.path)
		if err != nil {
			t.Fatal(err)
		}
		if file.lock {
			var lock model.StateMachineLock
			if err := json.Unmarshal(data, &lock); err != nil || !reflect.DeepEqual(lock, examples[0].Machine) {
				t.Fatalf("portable machine lock is stale: %v", err)
			}
		} else {
			var machine model.StateMachine
			if err := json.Unmarshal(data, &machine); err != nil || !reflect.DeepEqual(machine, examples[0].Machine.Machine) {
				t.Fatalf("portable machine definition is stale: %v", err)
			}
		}
	}
}
