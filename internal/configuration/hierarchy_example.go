package configuration

// HierarchyProject is an authoring exercise, not a clinical reception protocol.
// Each level has its own independent expectations and a local testbench file.
func HierarchyProject() Project {
	header := func(module string) string {
		return "language \"1.0\";\nmodule \"example.hierarchy." + module + "\" version \"1.0.0\";\n"
	}
	p := Project{Language: Version, Entry: "main.lce", Files: map[string]string{}}
	p.Files["main.lce"] = header("project") + `-- Composition exercise: ticket and identity propagation only.
import "reception/component.lce" as reception;
import "reception/testbench.lce" as receptionTests;
import "reception/components/queue/testbench.lce" as queueTests;
import "reception/components/queue/components/ticket/testbench.lce" as ticketTests;
import "reception/components/identification/testbench.lce" as identificationTests;
`
	p.Files["reception/component.lce"] = header("reception") + `import "components/queue/component.lce" as queue;
import "components/identification/component.lce" as identification;
behavior Reception {
 name = "Reception composition";
 behavior = {
  inputs = [{name="ticket";type="integer";},{name="patientId";type="string";}];
  outputs = [{name="ticket";type="integer";},{name="patientId";type="string";}];
  nodes = [
   {id="queue";component=ref(queue.Queue);inputs={ticket={input="ticket";};};},
   {id="identification";component=ref(identification.Identification);inputs={patientId={input="patientId";};};}
  ];
  bindings = {ticket={node="queue";port="ticket";};patientId={node="identification";port="patientId";};};
 };
}
`
	p.Files["reception/components/queue/component.lce"] = header("queue") + `import "components/ticket/component.lce" as ticket;
behavior Queue {
 name = "Queue management";
 behavior = {
  inputs=[{name="ticket";type="integer";}];
  outputs=[{name="ticket";type="integer";}];
  nodes=[{id="selection";component=ref(ticket.Ticket);inputs={ticket={input="ticket";};};}];
  bindings={ticket={node="selection";port="ticket";};};
 };
}
`
	p.Files["reception/components/queue/components/ticket/component.lce"] = header("ticket") + `behavior Ticket {
 name="Ticket selection";
 behavior={
  inputs=[{name="ticket";type="integer";}];
  outputs=[{name="ticket";type="integer";}];
  nodes=[{id="selection";op="identity";inputs={value={input="ticket";};};}];
  bindings={ticket={node="selection";port="result";};};
 };
}
`
	p.Files["reception/components/identification/component.lce"] = header("identification") + `behavior Identification {
 name="Patient identification";
 behavior={
  inputs=[{name="patientId";type="string";}];
  outputs=[{name="patientId";type="string";}];
  nodes=[{id="identity";op="identity";inputs={value={input="patientId";};};}];
  bindings={patientId={node="identity";port="result";};};
 };
}
`
	tests := []struct{ dir, module, symbol, inputs, expect string }{
		{"reception", "reception", "Reception", `ticket=integer(17);patientId=string("PAT-001");`, `"outputs.ticket.integer"=17;"outputs.patientId.string"="PAT-001";`},
		{"reception/components/queue", "queue", "Queue", `ticket=integer(42);`, `"outputs.ticket.integer"=42;`},
		{"reception/components/queue/components/ticket", "ticket", "Ticket", `ticket=integer(7);`, `"outputs.ticket.integer"=7;`},
		{"reception/components/identification", "identification", "Identification", `patientId=string("PAT-002");`, `"outputs.patientId.string"="PAT-002";`},
	}
	for _, t := range tests {
		p.Files[t.dir+"/testbench.lce"] = header(t.module+"Tests") + "import \"component.lce\" as subject;\n\n-- Expected values are specified independently of execution.\nexperiment " + t.symbol + "Example {\n target=target(subject." + t.symbol + ");\n scenario={inputs={" + t.inputs + "};};\n expect={" + t.expect + "};\n}\n"
	}
	return p
}
