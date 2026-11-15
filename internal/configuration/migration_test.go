package configuration

import (
	"context"
	sourceexamples "software-studio/examples/source"
	"strings"
	"testing"
)

func TestMigrationAndPortableEvidence(t *testing.T) {
	p, e := MigrateLegacy(sourceexamples.StockReservation)
	if e != nil {
		t.Fatal(e)
	}
	r, e := Run(context.Background(), p)
	if e != nil || !r.Passed || len(r.Cases) != 9 {
		t.Fatalf("migration: %v %+v", e, r.Cases)
	}
	pkg, e := Pack(context.Background(), "test.stock", "1.0.0", p, true)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = Replay(context.Background(), pkg); e != nil {
		t.Fatal(e)
	}
	comparison, e := Compare(pkg, pkg)
	if e != nil || !comparison.Compatible {
		t.Fatal(e, comparison)
	}
	pkg.Report.Cases[0].Passed = false
	pkg.Digest = PackageDigest(pkg)
	if _, e = Replay(context.Background(), pkg); e == nil {
		t.Fatal("tampered evidence accepted")
	}
	wrong := strings.Replace(sourceexamples.StockReservation, "expect committed", "expect denied", 1)
	p, e = MigrateLegacy(wrong)
	if e != nil {
		t.Fatal(e)
	}
	r, e = Run(context.Background(), p)
	if e != nil || r.Passed {
		t.Fatal("migration lost independent step oracle", e)
	}
}
