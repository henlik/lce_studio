package store

import (
	"context"
	"encoding/json"
	"errors"
	"software-studio/internal/configuration"
	"software-studio/internal/model"
	"software-studio/internal/testutil"
	"sync"
	"testing"
)

func configPackage(t *testing.T) configuration.Package {
	t.Helper()
	x := configuration.NewExporter()
	for _, e := range model.ExampleDataFixtures() {
		if _, err := x.Add("data", e.Contract.Contract); err != nil {
			t.Fatal(err)
		}
	}
	p, err := x.Project()
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := configuration.Pack(context.Background(), "test.release", "1.0.0", p, false)
	if err != nil {
		t.Fatal(err)
	}
	return pkg
}
func TestConfigurationAtomicInstallConflictReopenAndConcurrency(t *testing.T) {
	ctx := context.Background()
	url := testutil.DatabaseURL(t)
	db, e := Open(url)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	pkg := configPackage(t)
	// A conflict at a later artifact must leave earlier new artifacts absent.
	last := pkg.Artifacts[len(pkg.Artifacts)-1]
	var contract model.DataContract
	if e = json.Unmarshal(last.Body, &contract); e != nil {
		t.Fatal(e)
	}
	contract.Name = "Conflicting version"
	if _, _, e = db.InstallDataContract(ctx, contract); e != nil {
		t.Fatal(e)
	}
	if _, e = db.InstallConfiguration(ctx, pkg); !errors.Is(e, ErrConfigurationConflict) {
		t.Fatalf("expected conflict, got %v", e)
	}
	var count int
	_ = db.db.QueryRow(`SELECT count(*) FROM data_contracts`).Scan(&count)
	if count != 1 {
		t.Fatalf("partial install: %d", count)
	}
	_ = db.db.QueryRow(`SELECT count(*) FROM configuration_packages`).Scan(&count)
	if count != 0 {
		t.Fatal("release saved despite rollback")
	}
	if _, e = db.db.Exec(`DELETE FROM data_contracts`); e != nil {
		t.Fatal(e)
	}
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, e := db.InstallConfiguration(ctx, pkg); errs <- e }()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	_ = db.db.QueryRow(`SELECT count(*) FROM configuration_packages`).Scan(&count)
	if count != 1 {
		t.Fatal("idempotence", count)
	}
	reopened, e := Open(url)
	if e != nil {
		t.Fatal(e)
	}
	defer reopened.Close()
	stored, e := reopened.ConfigurationPackage(ctx, pkg.ID, pkg.Version)
	if e != nil || configuration.Hash(stored) != configuration.Hash(pkg) {
		t.Fatal("package did not survive reopen", e)
	}
}
