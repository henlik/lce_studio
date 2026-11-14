package conformance

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"software-studio/internal/system"
	"software-studio/internal/testutil"
)

func TestModelScenariosSatisfyContract(t *testing.T) {
	for _, s := range scenarios {
		t.Run(s.id, func(t *testing.T) {
			if got := aggregate(system.Evaluate(system.ReceptionExample().Contract, modelTrace(s))); got != "satisfied" {
				t.Fatalf("model verdict %s", got)
			}
		})
	}
}
func TestUnsupportedDesignRejected(t *testing.T) {
	d := system.ReceptionExample()
	d.Components[0].Definition.Version = "99.0.0"
	if SupportsDesign(d) {
		t.Fatal("unimplemented component version accepted")
	}
	if _, err := Run(context.Background(), nil, d); err != ErrUnsupportedDesign {
		t.Fatalf("got %v", err)
	}
}
func TestAggregateDoesNotHideViolation(t *testing.T) {
	v := []system.Verdict{{Status: "inconclusive"}, {Status: "violated"}}
	if aggregate(v) != "violated" {
		t.Fatal("violation hidden")
	}
	if aggregate(nil) != "inconclusive" {
		t.Fatal("empty evidence reported success")
	}
}
func TestPostgresConformance(t *testing.T) {
	db, err := sql.Open("pgx", testutil.DatabaseURL(t))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	report, err := Run(context.Background(), db, system.ReceptionExample())
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Scenarios) != 7 {
		t.Fatalf("got %d scenarios", len(report.Scenarios))
	}
	for _, s := range report.Scenarios {
		if !s.Passed {
			t.Errorf("scenario %s: expected %s, model=%+v, implementation=%+v", s.ID, s.ExpectedStatus, s.Model.Verdicts, s.Implementation.Verdicts)
		}
	}
	if !report.Passed {
		t.Fatal("conformance fixture failed")
	}

}
func TestTenantScopedDurability(t *testing.T) {
	db, err := sql.Open("pgx", testutil.DatabaseURL(t))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec("CREATE TABLE encounters (tenant_id text NOT NULL, request_id text NOT NULL, encounter_id text PRIMARY KEY, UNIQUE(tenant_id,request_id))")
	if err != nil {
		t.Fatal(err)
	}
	svc := intake{db: db, table: "encounters", events: &events{}}
	a, err := svc.receive(context.Background(), "hospital-a", "same-request")
	if err != nil {
		t.Fatal(err)
	}
	b, err := svc.receive(context.Background(), "hospital-b", "same-request")
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("request identity leaked across tenants")
	}
	restarted := intake{db: db, table: "encounters", events: &events{}}
	again, err := restarted.receive(context.Background(), "hospital-a", "same-request")
	if err != nil {
		t.Fatal(err)
	}
	if a != again {
		t.Fatal("durable deduplication lost")
	}
}

func TestSupportsDesignPreservesUserLabels(t *testing.T) {
	d := system.ReceptionExample()
	d.Components[0].Name = "My intake client"
	d.DeploymentPlans[0].Name = "My sandbox"
	if !SupportsDesign(d) {
		t.Fatal("display label changed execution support")
	}
	if d.Components[0].Name != "My intake client" || d.DeploymentPlans[0].Name != "My sandbox" {
		t.Fatal("support check mutated caller design")
	}
}

func TestInvalidContractRejectedBeforeDatabase(t *testing.T) {
	d := system.ReceptionExample()
	d.Contract.Claims[0].Kind = "execute_arbitrary_code"
	if _, err := Run(context.Background(), nil, d); err == nil {
		t.Fatal("unsupported contract accepted")
	}
}
