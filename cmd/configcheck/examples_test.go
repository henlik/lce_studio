package main

import (
	"context"
	"fmt"
	"path/filepath"
	"software-studio/internal/configuration"
	"strings"
	"testing"
)

// Read the actual files linked by the tutorial, not generated substitutes.
func TestDocumentedConfigurationExamples(t *testing.T) {
	for _, tc := range []struct {
		dir       string
		artifacts int
		tests     int
		passed    bool
	}{
		{"basics", 1, 3, true},
		{"counter", 2, 4, true},
		{"hierarchy", 4, 4, true},
		{"reuse", 2, 2, true},
		{"inspection", 56, 43, true},
		{"expected-failure", 1, 1, false},
	} {
		t.Run(tc.dir, func(t *testing.T) {
			p, err := loadModules(filepath.Join("..", "..", "examples", "configuration", tc.dir, "main.lce"))
			if err != nil {
				t.Fatal(err)
			}
			c := configuration.Compile(p)
			if !c.Valid || len(c.Artifacts) != tc.artifacts || len(c.Experiments) != tc.tests {
				t.Fatalf("unexpected documented structure: %d artifacts, %d tests, diagnostics %+v", len(c.Artifacts), len(c.Experiments), c.Diagnostics)
			}
			r, err := configuration.Run(context.Background(), p)
			if err != nil || r.Passed != tc.passed || len(r.Cases) != tc.tests {
				t.Fatalf("unexpected documented verdict: %v %+v", err, r)
			}
			for _, test := range r.Cases {
				if test.Error != "" {
					t.Fatalf("a tutorial must run, not fail with an execution error: %s", test.Error)
				}
			}
			if !tc.passed {
				a := r.Cases[0].Assertions
				if len(a) != 1 || !a[0].Observed || a[0].Passed || fmt.Sprint(a[0].Expected) != "24" || fmt.Sprint(a[0].Actual) != "23" {
					t.Fatalf("negative control failed for the wrong reason: %+v", a)
				}
				p.Files["echo/testbench.lce"] = strings.Replace(p.Files["echo/testbench.lce"], `"outputs.result.integer" = 24`, `"outputs.result.integer" = 23`, 1)
				fixed, err := configuration.Run(context.Background(), p)
				if err != nil || !fixed.Passed {
					t.Fatalf("documented repair did not pass: %v %+v", err, fixed)
				}
			}
		})
	}
}
