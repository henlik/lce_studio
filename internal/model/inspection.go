package model

import "fmt"

// InspectionCase names an independently runnable experiment, not a connection
// between the logical engines and the timed infrastructure simulator.
type InspectionCase struct {
	ID          string `json:"id"`
	Workspace   string `json:"workspace"`
	Name        string `json:"name"`
	Covers      string `json:"covers"`
	Endpoint    string `json:"endpoint"`
	Body        any    `json:"body"`
	VerdictPath string `json:"verdictPath"`
	Expected    any    `json:"expected"`
}
type InspectionPack struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Cases       []InspectionCase `json:"cases"`
}

// ReceptionInspectionStages changes only presentation metadata and identity.
// Existing 1.0.0 packages and their canonical digests remain untouched.
func ReceptionInspectionStages(w WorkflowDefinition) WorkflowDefinition {
	w.Nodes = append([]WorkflowNode(nil), w.Nodes...)
	w.ID = "health.reception-inspection"
	w.Name = "Reception · staged inspection"
	stages := map[string][]string{
		"01 Arrival/Screening":       {"initialScreening", "entryRoute", "screeningGate"},
		"01 Arrival/Queue":           {"issueTicket", "callNext", "markCalled"},
		"02 Identification":          {"lookupIdentity", "classifyLookup", "identityRoute", "registerPatient", "confirmIdentity", "identityReview", "validateIdentity"},
		"03 Affiliation":             {"lookupAffiliation", "classifyAffiliation", "affiliationReview"},
		"04 Encounter":               {"openEncounter"},
		"05 Intake/Observations":     {"recordVitals", "observationReview", "validateObservations", "feverGate"},
		"05 Intake/Clinical release": {"feverReview", "clinicalReview", "classifyClinical", "clinicalGate"},
		"06 Financing":               {"paymentPolicy", "paymentReview", "classifyPayment"},
		"07 Dispatch/Selection":      {"queryWorkforce", "eligibleA", "eligibleB", "selectDoctor", "validateChoice", "waitDoctor"},
		"07 Dispatch/Reservation":    {"reserveChoice", "reserveA", "reserveB"},
		"07 Dispatch/Handoff":        {"physicianAcceptance", "validatePhysician", "acceptChoice", "declineChoice", "acceptA", "acceptB", "declineA", "declineB", "routineComplete"},
		"Emergency/Cleanup":          {"beginEmergency", "cancelTicket", "emergencyReservationGate", "emergencyReleaseChoice", "emergencyReleaseA", "emergencyReleaseB"},
		"Emergency/ECU handoff":      {"ecuAcceptance", "ecuPending", "validateECU", "emergencyComplete"},
		"Dependency recovery":        {"dependencyReview"},
	}
	for i := range w.Nodes {
		w.Nodes[i].Stage = "Unassigned"
		for path, ids := range stages {
			for _, id := range ids {
				if w.Nodes[i].ID == id {
					w.Nodes[i].Stage = path
				}
			}
		}
	}
	return w
}

func StudioInspectionPack() InspectionPack {
	pack := InspectionPack{ID: "health-ecosystem-inspection-v1", Name: "Regional care ecosystem · inspection lab", Description: "Synthetic reception, shared pharmacy reservations and constrained deployment, with education and service-desk controls for cross-domain reuse. Each exercise runs independently; no live clinical adapters, authentication enforcement or cross-engine scheduler is implied."}
	add := func(workspace, name, covers, endpoint string, body any, path string, expected any) {
		pack.Cases = append(pack.Cases, InspectionCase{ID: fmt.Sprintf("%s-%02d", workspace, len(pack.Cases)+1), Workspace: workspace, Name: name, Covers: covers, Endpoint: endpoint, Body: body, VerdictPath: path, Expected: expected})
	}
	for _, e := range ExampleWorkflowScenarios() {
		if e.Workflow.Workflow.ID == "health.hospital-reception" {
			e.Workflow.Workflow = ReceptionInspectionStages(e.Workflow.Workflow)
			digest, err := WorkflowDigest(e.Workflow.Workflow)
			if err != nil {
				panic(err)
			}
			e.Workflow.Digest = digest
		}
		add("workflows", e.Scenario.Name, "Stages, tasks, branches, interrupts, exact decisions, owned commands and handwritten expectations", "/workflow/runs", e, "result.passed", true)
	}
	for _, e := range ExampleDataFixtures() {
		add("data", e.Fixture.Name, "Records, keys, tenant references and relationships", "/data/validate", e, "result.valid", true)
	}
	invalid := ExampleDataFixtures()[0]
	invalid.Fixture.Name = "Broken patient affiliation reference"
	invalid.Fixture.Rows["Subjects"][0]["tenantId"] = reservationText("missing-organization")
	add("data", invalid.Fixture.Name, "Invalid reference highlighting; rejection is the expected result", "/data/validate", invalid, "result.valid", false)
	for _, e := range append(ExampleComponentBundles(), ExampleStructuredComponentBundles()...) {
		for _, p := range e.Packages {
			if p.Package.ID == e.Root.PackageID && len(p.Package.Tests) > 0 {
				add("components", p.Package.Name, "Primitive operations, recursive composition, typed bindings and definition tests", "/components/test", map[string]any{"bundle": e}, "passed", true)
			}
		}
	}
	for _, e := range ExampleStateScenarios() {
		add("state", e.Scenario.Name, "Independent owners, revision conflicts, deduplication, crash recovery, durable and volatile state", "/state/runs", e, "result.passed", true)
	}
	for _, e := range ExampleDeliveryScenarios() {
		add("messages", e.Scenario.Name, "Typed routes, frozen payloads, lost acknowledgements, retry, rollback and receipts", "/delivery/runs", e, "result.passed", true)
	}
	for _, p := range []Project{ExampleProject(), BranchingExampleProject(), ContinuumExampleProject()} {
		add("design", p.Name, "Requirements, timed routing, retries, deduplication and trace replay; continuum adds geography, CPU, memory, bandwidth and energy", "/runs", map[string]any{"project": p}, "metrics.completed", p.Scenario.RequestCount)
	}
	outage := ContinuumExampleProject()
	outage.ID = "inspection-power-loss"
	outage.Name = "Reception edge power interrupted"
	outage.Infrastructure.Hosts[1].Outages = []Outage{{StartMs: 0, EndMs: 100000}}
	add("deployment", outage.Name, "Power outage and exhausted retries; zero completions expected", "/runs", map[string]any{"project": outage}, "metrics.completed", 0)

	network := ContinuumExampleProject()
	network.ID = "inspection-link-loss"
	network.Name = "Reception local network interrupted"
	network.Infrastructure.Links[0].Outages = []Outage{{StartMs: 0, EndMs: 100000}}
	add("deployment", network.Name, "Link outage and exhausted retries; zero completions expected", "/runs", map[string]any{"project": network}, "metrics.completed", 0)
	cloud := ContinuumExampleProject()
	cloud.ID = "inspection-cloud-placement"
	cloud.Name = "Reception services on remote cloud"
	cloud.Infrastructure.Placements[1].HostID = "cloud-host"
	cloud.Infrastructure.Placements[2].HostID = "cloud-host"
	add("deployment", cloud.Name, "Same workload on remote geography; explicit WAN latency and bandwidth", "/runs", map[string]any{"project": cloud}, "metrics.completed", cloud.Scenario.RequestCount)
	cpu := ContinuumExampleProject()
	cpu.ID = "inspection-one-core"
	cpu.Name = "Reception burst on one edge CPU"
	cpu.Infrastructure.Hosts[1].CPUCores = 1
	cpu.Scenario.IntervalMs = 1
	cpu.Scenario.TimeoutMs = 1000
	add("deployment", cpu.Name, "CPU contention and admission queue for a one-millisecond arrival burst", "/runs", map[string]any{"project": cpu}, "metrics.completed", cpu.Scenario.RequestCount)
	return pack
}
