package source

import (
	"encoding/json"
	sourceexamples "software-studio/examples/source"
	"strings"
	"testing"
)

func TestM1Source(t *testing.T) {
	r := Compile(sourceexamples.M1)
	if !r.Valid || r.Design == nil {
		t.Fatal(r.Diagnostics)
	}
	d := r.Design
	if d.ID != "learning.m1-reservation" || len(d.Components) != 2 || len(d.ExecutionProviders) != 3 || len(d.Components[0].WorkflowMessages) != 1 {
		t.Fatal(d)
	}
	if d.Components[0].WorkflowMessages[0].Response["accepted"] != "accepted" {
		t.Fatal("response mapping")
	}
	if pos := Locate(r.Locations, "components[0].bindings[1].resourceId"); pos.Line != 13 {
		t.Fatal("incorrect source mapping", pos)
	}
	if pos := Locate(r.Locations, "components[1].definition.version"); pos.Line != 24 {
		t.Fatal(pos)
	}
	a, _ := json.Marshal(r)
	b, _ := json.Marshal(Compile(sourceexamples.M1))
	if string(a) != string(b) {
		t.Fatal("nondeterministic compilation")
	}
	upper := strings.Replace(sourceexamples.M1, "system \"learning", "SYSTEM \"learning", 1)
	if !Compile(upper).Valid {
		t.Fatal("keywords must be case insensitive")
	}
}
func TestSourceRejectsMalformedAndUnsupported(t *testing.T) {
	for name, input := range map[string]string{
		"empty":                   "",
		"missing semicolon":       strings.Replace(sourceexamples.M1, "end system;", "end system", 1),
		"trailing source":         sourceexamples.M1 + " process begin end;",
		"missing library version": strings.Replace(sourceexamples.M1, "version \"2.0.0\"", "version \"latest\"", 1),
		"duplicate instance":      strings.Replace(sourceexamples.M1, "\"ledger\" : component", "\"stock\" : component", 1),
		"unknown provider":        strings.Replace(sourceexamples.M1, "\"state/stock\" => \"owned-state\"", "\"state/stock\" => \"missing\"", 1),
		"generic":                 strings.Replace(sourceexamples.M1, "port map (", "generic map (", 1),
		"duplicate binding":       strings.Replace(sourceexamples.M1, "\"messages/notices\" =>", "\"state/stock\" =>", 1),
		"duplicate response":      strings.Replace(sourceexamples.M1, "response map (\"accepted\" => \"accepted\")", "response map (\"accepted\" => \"accepted\", \"accepted\" => \"accepted\")", 1),
		"duplicate name":          strings.Replace(sourceexamples.M1, "name \"Stock owner\";", "name \"Stock owner\"; name \"Other\";", 1),
		"unterminated":            "system \"abc",
		"bad escape":              "system \"a\\x01\"",
		"trailing comma":          strings.Replace(sourceexamples.M1, "\"workflow/flow\" => \"workflow-model\"", "\"workflow/flow\" => \"workflow-model\",", 1),
		"too big":                 strings.Repeat(" ", MaxBytes+1),
		"token limit":             strings.Repeat("is ", 8193),
		"invalid utf8":            string([]byte{255}),
	} {
		t.Run(name, func(t *testing.T) {
			r := Compile(input)
			if r.Valid || r.Design != nil || len(r.Diagnostics) == 0 {
				t.Fatal("invalid source accepted", r)
			}
			if r.Diagnostics[0].Line < 1 || r.Diagnostics[0].Column < 1 {
				t.Fatal("no source location")
			}
		})
	}
}
func TestSourceConnectionsEntriesAndEscapes(t *testing.T) {
	input := strings.Replace(sourceexamples.M1, "name \"Stock owner\";", "name \"Stock \\"+"\"owner\\\"\";\n entry \"stock\" => \"reserve\";", 1)
	// JSON escaping is independent of comments and punctuation inside quoted names.
	input = strings.Replace(input, "end system;", `assume "offline" : "No live I/O -- modeled only";
 connect "copy" : "stock"."input"."outputs"."request" => "ledger"."ledger"."command.recordReservation.inputs"."notice";
end system;`, 1)
	r := Compile(input)
	if !r.Valid {
		t.Fatal(r.Diagnostics, input)
	}
	if len(r.Design.Connections) != 1 || len(r.Design.Components[0].StateEntries) != 1 || r.Design.Components[0].Name != `Stock "owner"` {
		t.Fatal(r.Design)
	}
}
func FuzzCompile(f *testing.F) {
	f.Add(sourceexamples.M1)
	f.Add("")
	f.Add("system")
	f.Add("\"\\\"")
	f.Fuzz(func(t *testing.T, input string) {
		r := Compile(input)
		if r.Valid && r.Design == nil {
			t.Fatal("valid without model")
		}
		if !r.Valid && len(r.Diagnostics) == 0 {
			t.Fatal("invalid without diagnosis")
		}
	})
}
