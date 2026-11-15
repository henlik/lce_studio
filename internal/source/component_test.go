package source

import (
	"context"
	"encoding/json"
	sourceexamples "software-studio/examples/source"
	"strings"
	"testing"
)

func TestCompleteStockComponent(t *testing.T) {
	r := Compile(sourceexamples.StockReservation)
	if !r.Valid || r.Component == nil || r.Testbench == nil {
		t.Fatal(r.Diagnostics)
	}
	if len(r.Testbench.Cases) != 9 || len(r.Component.Machine.Machine.Commands) != 1 || len(r.Component.Data.Contract.Records) != 4 {
		t.Fatal("incomplete artifact pack")
	}
	a, _ := json.Marshal(r)
	b, _ := json.Marshal(Compile(sourceexamples.StockReservation))
	if string(a) != string(b) {
		t.Fatal("nondeterministic compilation")
	}
	report, err := RunTestbench(context.Background(), sourceexamples.StockReservation)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range report.Cases {
		if !c.Passed {
			t.Errorf("%s: %+v, final=%+v", c.Name, c.Diagnostics, c.Result.Expectations)
		}
	}
	if !report.Passed {
		t.Fatal("testbench failed")
	}
	if report.Source != sourceexamples.StockReservation || len(report.SourceHash) != 64 {
		t.Fatal("missing provenance")
	}
}

func TestTestbenchIndependentOracles(t *testing.T) {
	for name, change := range map[string][2]string{
		"status":     {"expect committed outputs", "expect denied outputs"},
		"outputs":    {"outputs (accepted => true, remaining => 3)", "outputs (accepted => true, remaining => 4)"},
		"state":      {"available => 0) revision 2", "available => 1) revision 2"},
		"revision":   {"revision 2 receipts 3 effects 2", "revision 3 receipts 3 effects 2"},
		"receipts":   {"revision 2 receipts 3 effects 2", "revision 2 receipts 4 effects 2"},
		"effects":    {"revision 2 receipts 3 effects 2", "revision 2 receipts 3 effects 3"},
		"payload":    {"quantity => 2, remaining => 3) from", "quantity => 2, remaining => 4) from"},
		"lost reply": {"expect committed reply_lost;", "expect committed;"},
	} {
		t.Run(name, func(t *testing.T) {
			text := strings.Replace(sourceexamples.StockReservation, change[0], change[1], 1)
			report, err := RunTestbench(context.Background(), text)
			if err != nil || report.Passed {
				t.Fatal("incorrect oracle passed", err)
			}
			found := false
			for _, c := range report.Cases {
				for _, d := range c.Diagnostics {
					found = true
					if d.Line <= 1 {
						t.Fatal("unmapped failure", d)
					}
				}
			}
			if !found {
				t.Fatal("missing diagnostics")
			}
		})
	}
}

func TestComponentRejectsInvalidSource(t *testing.T) {
	for name, change := range map[string][2]string{
		"wrong type":      {"quantity > 0", "quantity and true"},
		"unknown state":   {"state.available - taken", "state.missing - taken"},
		"duplicate write": {"return remaining := next.available;", "set available := 1; return remaining := next.available;"},
		"missing return":  {"return remaining := next.available;", ""},
		"duplicate local": {"let taken := select(accepted, quantity, 0);", "let taken := 1; let taken := 2;"},
		"literal range":   {"available : integer := 5", "available : integer := 9007199254740992"},
		"state type":      {"available : integer := 5", "available : boolean := 5"},
		"event argument":  {"quantity => taken, remaining => next.available", "quantity => taken"},
		"fault":           {"fault before_commit", "fault unknown"},
		"target":          {"for \"learning.stock-reservation\"", "for \"missing\""},
		"test input":      {"quantity => 2)", "quantity => true)"},
		"deep expression": {"quantity > 0", strings.Repeat("(", 40) + "quantity > 0" + strings.Repeat(")", 40)},
		"empty tests":     {"testbench \"Stock reservation acceptance\" for \"learning.stock-reservation\" is", "testbench \"Stock reservation acceptance\" for \"learning.stock-reservation\" is end testbench;"},
	} {
		t.Run(name, func(t *testing.T) {
			r := Compile(strings.Replace(sourceexamples.StockReservation, change[0], change[1], 1))
			if r.Valid || r.Component != nil || r.Design != nil || r.Testbench != nil || len(r.Diagnostics) == 0 {
				t.Fatal("accepted invalid component", r.Diagnostics)
			}
		})
	}
}

func TestTestbenchCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := RunTestbench(ctx, sourceexamples.StockReservation); err != context.Canceled {
		t.Fatal(err)
	}
}

func TestGenericCommandsAndVolatileState(t *testing.T) {
	text := `component "learning.counter" version "1.0.0" is
 state count : integer := 0 durable;
 state ready : boolean := false volatile;
 state title : string := "idle" durable;
 command update (enabled : boolean, label : string) returns (value : integer) is
   accept when enabled or not state.ready;
   set count := state.count + 1;
   set ready := true;
   set title := label;
   return value := next.count;
 end command;
 command read () returns (value : integer, label : string, live : boolean) is
   accept when true;
   return value := state.count;
   return label := state.title;
   return live := state.ready;
 end command;
end component;
testbench "Generic lowering" for "learning.counter" is
 case "Independent commands, durable and volatile values" is
  call update (enabled => true, label => "working") id "updateA" revision 0 expect committed outputs (accepted => true, value => 1);
  restart;
  call read () id "readA" revision 1 expect committed outputs (accepted => true, value => 1, label => "working", live => false);
  expect state (count => 1, ready => false, title => "working") revision 2 receipts 2 effects 0;
 end case;
end testbench;`
	r := Compile(text)
	if !r.Valid {
		t.Fatal(r.Diagnostics)
	}
	report, err := RunTestbench(context.Background(), text)
	if err != nil || !report.Passed {
		t.Fatal(err, report.Cases)
	}
}
