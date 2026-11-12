package model_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base32"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"software-studio/internal/behavior"
	"software-studio/internal/delivery"
	"software-studio/internal/model"
)

func TestDeliveryExamplesShareExactNetworkAndLegacySource(t *testing.T) {
	examples := model.ExampleDeliveryScenarios()
	if len(examples) != 2 || !reflect.DeepEqual(examples[0].Model, examples[1].Model) {
		t.Fatal("pharmacy and library must reuse one exact delivery model")
	}
	machines := examples[0].Model.Model.Machines
	if len(machines) != 2 || !reflect.DeepEqual(machines[0], model.ExampleStateScenarios()[0].Machine) {
		t.Fatal("source must be the unchanged exact v0.6 reservation machine")
	}
	if !reflect.DeepEqual(machines[0].Machine.DataContracts, machines[1].Machine.DataContracts) {
		t.Fatal("recipient must use the source's exact reservation-notice data contract")
	}
	for _, example := range examples {
		if issues := model.ValidateDeliveryModelLock(example.Model); len(issues) != 0 {
			t.Fatalf("invalid delivery model: %+v", issues)
		}
		if issues := model.ValidateDeliveryScenario(example.Model, example.Scenario); len(issues) != 0 {
			t.Fatalf("invalid delivery scenario: %+v", issues)
		}
	}
	*examples[0].Model.Model.Machines[0].Machine.State[0].Initial.String = "changed"
	*examples[0].Scenario.Steps[0].Inputs["request"].Record["quantity"].Integer = 999
	*examples[0].Scenario.ExpectedMessages[0].Accepted = false
	if *examples[1].Model.Model.Machines[0].Machine.State[0].Initial.String != "tenantDefault" || *examples[0].Scenario.Steps[8].Inputs["request"].Record["quantity"].Integer != 2 || !*examples[0].Scenario.ExpectedMessages[1].Accepted {
		t.Fatal("returned definition, command or expectation pointers alias")
	}
	fresh := model.ExampleDeliveryScenarios()[0]
	if *fresh.Model.Model.Machines[0].Machine.State[0].Initial.String != "tenantDefault" || *fresh.Scenario.Steps[0].Inputs["request"].Record["quantity"].Integer != 2 {
		t.Fatal("example mutation escaped the returned artifact")
	}
}

func TestDeliveryLedgerPureBoundaryCases(t *testing.T) {
	bundle := model.ExampleDeliveryScenarios()[0].Model.Model.Machines[1].Machine.Commands[0].Decision
	if len(bundle.Packages) != 1 || len(bundle.Packages[0].Package.Tests) != 8 {
		t.Fatal("expected one generic ledger reducer with eight independent cases")
	}
	for _, test := range bundle.Packages[0].Package.Tests {
		t.Run(test.Name, func(t *testing.T) {
			result, err := behavior.Evaluate(context.Background(), bundle, test.Inputs)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(result.Outputs, test.Expected) || result.Operations != 22 || result.Invocations != 1 {
				t.Fatalf("ledger result disagrees with independent expected values/work: %+v", result)
			}
		})
	}
	// The total guard prevents quantity overflow. The volatile counter retains
	// ordinary safe-integer semantics and can still produce a decision error.
	inputs := map[string]model.TypedValue{}
	for name, value := range bundle.Packages[0].Package.Tests[0].Inputs {
		inputs[name] = value
	}
	limit := model.MaxBehaviorInteger
	inputs["acceptedSinceRestart"] = model.TypedValue{Type: "integer", Integer: &limit}
	if _, err := behavior.Evaluate(context.Background(), bundle, inputs); err == nil || !strings.Contains(err.Error(), "safe integer range") {
		t.Fatalf("accepted counter overflow must remain an explicit evaluation error: %v", err)
	}
}

func TestDeliveryExamplesMatchIndependentRecoveryOracle(t *testing.T) {
	wantStatuses := []string{"committed", "dispatched", "dropped", "recipient_unacknowledged", "acknowledgement_lost", "restarted", "restarted", "acknowledged", "replayed", "dispatch_replayed", "committed", "dispatched", "acknowledged", "committed", "dispatched", "acknowledged", "denied", "replayed", "restarted"}
	for domain, example := range model.ExampleDeliveryScenarios() {
		t.Run(example.Scenario.Name, func(t *testing.T) {
			before, _ := json.Marshal(example)
			result, err := delivery.Run(context.Background(), example.Model, example.Scenario)
			if err != nil {
				t.Fatal(err)
			}
			if !result.Verified || !result.Passed || result.EngineVersion != "0.7.0" || result.Operations != 172 || result.Invocations != 8 {
				t.Fatalf("unexpected oracle verdict/work: verified=%v passed=%v version=%s operations=%d invocations=%d", result.Verified, result.Passed, result.EngineVersion, result.Operations, result.Invocations)
			}
			if len(result.Steps) != 19 || len(result.Instances) != 2 || len(result.Messages) != 3 || len(result.Expectations) != 2 || len(result.MessageExpectations) != 3 {
				t.Fatal("missing independent scenario evidence")
			}
			for i, status := range wantStatuses {
				if result.Steps[i].Status != status || result.Steps[i].Index != i {
					t.Fatalf("step %d: got %s, want %s", i, result.Steps[i].Status, status)
				}
				if result.Steps[i].State != nil && result.Steps[i].State.Index != i {
					t.Fatalf("nested state index does not identify outer step %d", i)
				}
			}
			stock, ledger := result.Instances[0], result.Instances[1]
			if stock.ID != "stock" || *stock.State["available"].Integer != 0 || *stock.State["acceptedSinceRestart"].Integer != 2 || stock.Revision != 3 || len(stock.Receipts) != 4 || len(stock.Effects) != 3 {
				t.Fatalf("stock differs from handwritten final 0/2, revision3, receipts4, intents3: %+v", stock)
			}
			if ledger.ID != "ledger" || *ledger.State["totalReserved"].Integer != int64(5-domain) || *ledger.State["acceptedSinceRestart"].Integer != 10 || ledger.Revision != 3 || len(ledger.Receipts) != 3 || len(ledger.Effects) != 0 {
				t.Fatalf("ledger differs from handwritten final total/counter, revision3, receipts3, intents0: %+v", ledger)
			}
			for i, message := range result.Messages {
				if message.Name != []string{"firstReservation", "secondReservation", "thirdReservation"}[i] || message.Status != "acknowledged" || message.Attempts != []int{4, 1, 1}[i] || message.Acknowledgement == nil || !message.Acknowledgement.Accepted {
					t.Fatalf("message %d differs from independent final transport expectation: %+v", i, message)
				}
				encoded, _ := json.Marshal([]string{stock.Effects[i].ID, "reservationToLedger"})
				hash := sha256.Sum256(encoded)
				wantID := "msg_" + strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(hash[:]))
				if message.ID != wantID || message.SourceIntentID != stock.Effects[i].ID || message.Acknowledgement.CommandID != wantID || message.ExpectedRevision != int64(i) || len(message.ID) != 56 {
					t.Fatal("message identity or frozen request changed")
				}
				if !reflect.DeepEqual(message.Inputs["notice"], stock.Effects[i].Payload["notice"]) || !reflect.DeepEqual(*message.Acknowledgement, ledger.Receipts[i]) {
					t.Fatal("envelope or acknowledgement differs from its original owned evidence")
				}
			}
			drop := result.Steps[2]
			if !drop.FaultApplied || drop.State != nil || drop.BeforeAttempts != 0 || drop.AfterAttempts != 1 {
				t.Fatal("transport drop must consume one attempt without recipient execution")
			}
			rollback := result.Steps[3]
			if !rollback.FaultApplied || rollback.State == nil || rollback.State.Status != "rolled_back" || rollback.State.Receipt != nil || *rollback.State.After["totalReserved"].Integer != 0 || *rollback.State.After["acceptedSinceRestart"].Integer != 10 || rollback.AfterAttempts != 2 {
				t.Fatal("recipient rollback must preserve all state and leave message pending")
			}
			lost := result.Steps[4]
			if !lost.FaultApplied || lost.AfterStatus != "pending" || lost.AfterAttempts != 3 || lost.State == nil || lost.State.Status != "committed" || lost.State.Receipt == nil || lost.State.ReplyLost || *lost.State.After["totalReserved"].Integer != int64(2-domain) {
				t.Fatal("acknowledgement loss must be outside the committed state protocol")
			}
			replay := result.Steps[7]
			if replay.State == nil || replay.State.Status != "replayed" || replay.State.Decision != nil || replay.BeforeStatus != "pending" || replay.AfterStatus != "acknowledged" || replay.BeforeAttempts != 3 || replay.AfterAttempts != 4 || *replay.State.After["acceptedSinceRestart"].Integer != 10 || *replay.State.Receipt.Outputs["acceptedSinceRestart"].Integer != 11 {
				t.Fatal("retry must acknowledge the historical receipt while preserving restarted current state")
			}
			if result.Steps[9].State != nil || result.Steps[9].BeforeAttempts != 4 || result.Steps[9].AfterAttempts != 4 {
				t.Fatal("redispatch must not create another attempt or recipient evaluation")
			}
			if result.Steps[16].State == nil || result.Steps[16].State.Receipt.Accepted || len(result.Steps[16].State.Receipt.EffectIDs) != 0 || result.Steps[17].State == nil || result.Steps[17].State.Decision != nil {
				t.Fatal("denied source response must be cached without another intent")
			}
			again, err := delivery.Run(context.Background(), example.Model, example.Scenario)
			if err != nil || !reflect.DeepEqual(result, again) {
				t.Fatal("independent delivery runs must reproduce fresh initialized evidence")
			}
			after, _ := json.Marshal(example)
			if !bytes.Equal(before, after) {
				t.Fatal("delivery mutated the definition, inputs or independent expectations")
			}
		})
	}
}

func TestDeliveryPortableExamplesMatchDefinitions(t *testing.T) {
	examples := model.ExampleDeliveryScenarios()
	for i, path := range []string{"../../examples/delivery/pharmacy-reservation.run.json", "../../examples/delivery/library-reservation.run.json"} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var portable model.DeliveryExample
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&portable); err != nil || !reflect.DeepEqual(portable, examples[i]) {
			t.Fatalf("portable delivery example is stale or invalid: %s: %v", path, err)
		}
	}
	for _, file := range []struct {
		path string
		lock bool
	}{{"../../examples/delivery/reservation-delivery.model.json", false}, {"../../examples/delivery/reservation-delivery.lock.json", true}} {
		data, err := os.ReadFile(file.path)
		if err != nil {
			t.Fatal(err)
		}
		if file.lock {
			var lock model.DeliveryModelLock
			if err := json.Unmarshal(data, &lock); err != nil || !reflect.DeepEqual(lock, examples[0].Model) {
				t.Fatalf("portable delivery lock is stale: %v", err)
			}
		} else {
			var definition model.DeliveryModel
			if err := json.Unmarshal(data, &definition); err != nil || !reflect.DeepEqual(definition, examples[0].Model.Model) {
				t.Fatalf("portable delivery model is stale: %v", err)
			}
		}
	}
}
