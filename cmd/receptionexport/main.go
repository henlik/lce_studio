// receptionexport writes canonical portable fixtures and every reusable direct
// dependency. It does not execute scenarios to manufacture their expectations.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"software-studio/internal/model"
)

func main() {
	root := "examples/reception"
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	write := func(name string, value any) {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			panic(err)
		}
		body, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			panic(err)
		}
		if err = os.WriteFile(path, append(body, '\n'), 0644); err != nil {
			panic(err)
		}
		fmt.Println(path)
	}
	hospital := model.HospitalReceptionWorkflow()
	write("hospital-reception.workflow.json", hospital.Workflow)
	write("hospital-reception.lock.json", hospital)
	desk := model.ServiceDeskWorkflow()
	write("service-desk.workflow.json", desk.Workflow)
	write("service-desk.lock.json", desk)
	names := []string{"routine-existing", "new-patient", "no-affiliation", "fever-release", "fever-review", "stale-release", "emergency-before-ticket", "emergency-after-reservation", "no-doctor", "registry-unavailable", "ambiguous-identity", "affiliation-unknown", "ecu-unavailable", "payment-failure", "missing-observations", "reservation-denied", "clinician-emergency", "initial-screening", "service-desk"}
	examples := model.ExampleWorkflowScenarios()
	if len(names) != len(examples) {
		panic("scenario filename mapping needs update")
	}
	for i, example := range examples {
		write("scenarios/"+names[i]+".run.json", example)
	}
	write("data/reception-values.lock.json", model.ReceptionDataContract())
	seen := map[string]bool{}
	writeBundle := func(bundle model.ComponentBundle) {
		key := bundle.Root.PackageID
		if !seen[key] {
			write("decisions/"+key+".bundle.json", bundle)
			seen[key] = true
		}
	}
	for _, bundle := range model.ReceptionDecisionBundles() {
		writeBundle(bundle)
	}
	for _, owner := range model.ReceptionStateMachines() {
		write("owners/"+owner.Machine.ID+".lock.json", owner)
		for _, command := range owner.Machine.Commands {
			writeBundle(command.Decision)
		}
	}
}
