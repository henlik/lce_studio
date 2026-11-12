package model_test

import (
	"context"
	"software-studio/internal/behavior"
	"software-studio/internal/model"
	"software-studio/internal/workflow"
	"testing"
)

func TestReceptionReusableParts(t *testing.T) {
	for _, bundle := range model.ReceptionDecisionBundles() {
		if issues := model.ValidateComponentBundle(bundle); len(issues) > 0 {
			t.Fatalf("%s: %+v", bundle.Root.PackageID, issues)
		}
		if len(bundle.Packages[0].Package.Tests) == 0 {
			continue
		}
		result, err := behavior.Test(context.Background(), bundle)
		if err != nil || !result.Passed {
			t.Fatalf("%s: %v %+v", bundle.Root.PackageID, err, result)
		}
	}
	for _, lock := range model.ReceptionStateMachines() {
		if issues := model.ValidateStateMachineLock(lock); len(issues) > 0 {
			t.Fatalf("%s: %+v", lock.Machine.ID, issues)
		}
		for _, command := range lock.Machine.Commands {
			if len(command.Decision.Packages[0].Package.Tests) == 0 {
				continue
			}
			result, err := behavior.Test(context.Background(), command.Decision)
			if err != nil || !result.Passed {
				t.Fatalf("%s.%s: %v %+v", lock.Machine.ID, command.Name, err, result)
			}
		}
	}
}
func TestReceptionWorkflowExamples(t *testing.T) {
	for _, example := range model.ExampleWorkflowScenarios() {
		t.Run(example.Scenario.Name, func(t *testing.T) {
			if issues := model.ValidateWorkflowScenario(example.Workflow, example.Scenario); len(issues) > 0 {
				t.Fatalf("invalid: %+v", issues)
			}
			result, err := workflow.Run(context.Background(), example.Workflow, example.Scenario)
			if err != nil || !result.Verified || !result.Passed {
				t.Fatalf("%v: %+v", err, result)
			}
		})
	}
}
