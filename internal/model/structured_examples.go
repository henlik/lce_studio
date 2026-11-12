package model

import "fmt"

// ExampleStructuredComponentBundles adapts one domain-neutral record contract
// to the unchanged scalar access policy in two independently owned examples.
func ExampleStructuredComponentBundles() []ComponentBundle {
	text := func(s string) TypedValue { return TypedValue{Type: "string", String: &s} }
	boolean := func(b bool) TypedValue { return TypedValue{Type: "boolean", Boolean: &b} }
	input := func(name string) BehaviorBinding { return BehaviorBinding{Input: name} }
	output := func(node, port string) BehaviorBinding { return BehaviorBinding{Node: node, Port: port} }
	literal := func(s string) BehaviorBinding { v := text(s); return BehaviorBinding{Literal: &v} }
	bundles := make([]ComponentBundle, 0, 2)
	for i, domain := range []struct{ id, name, role, scope string }{
		{"example.structured-health-access", "Structured health access", "clinician", "record:read"},
		{"example.structured-school-access", "Structured school access", "teacher", "student:read"},
	} {
		data := ExampleDataFixtures()[i].Contract
		ref := func(record string) *DataTypeRef {
			return &DataTypeRef{ContractID: data.Contract.ID, Version: data.Contract.Version, Digest: data.Digest, Record: record}
		}
		legacy := ExampleComponentBundles()[i]
		policy := legacy.Packages[1]
		p := ComponentPackage{SchemaVersion: 3, ID: domain.id, Version: "1.0.0", Name: domain.name,
			Description: "Extracts explicit subject and resource fields from exact typed records, then reuses the unchanged shared scalar access policy. Synthetic examples demonstrate composition, not production authorization enforcement.",
			Author:      "LCE Studio contributors", License: "MIT", Kernel: "composite", Capabilities: []string{"design", "evaluate"}, DataContracts: []DataContractLock{data}}
		p.Behavior = &Behavior{
			Inputs:  []BehaviorPort{{Name: "subject", Type: "record", RecordRef: ref("Subject")}, {Name: "resource", Type: "record", RecordRef: ref("Resource")}},
			Outputs: []BehaviorPort{{Name: "permitted", Type: "boolean"}},
			Nodes: []BehaviorNode{
				{ID: "subjectTenant", Op: "record_get", Field: "tenantId", Position: Position{X: 60, Y: 40}, Inputs: map[string]BehaviorBinding{"value": input("subject")}},
				{ID: "role", Op: "record_get", Field: "role", Position: Position{X: 60, Y: 200}, Inputs: map[string]BehaviorBinding{"value": input("subject")}},
				{ID: "active", Op: "record_get", Field: "active", Position: Position{X: 60, Y: 360}, Inputs: map[string]BehaviorBinding{"value": input("subject")}},
				{ID: "resourceTenant", Op: "record_get", Field: "tenantId", Position: Position{X: 400, Y: 40}, Inputs: map[string]BehaviorBinding{"value": input("resource")}},
				{ID: "scope", Op: "record_get", Field: "scope", Position: Position{X: 400, Y: 200}, Inputs: map[string]BehaviorBinding{"value": input("resource")}},
				{ID: "policy", Component: &ComponentRef{PackageID: policy.Package.ID, Version: policy.Package.Version, Digest: policy.Digest}, Position: Position{X: 740, Y: 160}, Inputs: map[string]BehaviorBinding{
					"subjectTenant": output("subjectTenant", "result"), "resourceTenant": output("resourceTenant", "result"), "role": output("role", "result"),
					"requiredRole": literal(domain.role), "requestedScope": output("scope", "result"), "allowedScope": literal(domain.scope), "active": output("active", "result"),
				}},
			},
			Bindings: map[string]BehaviorBinding{"permitted": output("policy", "permitted")},
		}
		for j, name := range []string{"Permit", "Wrong tenant", "Wrong role", "Wrong scope", "Inactive subject"} {
			subject := TypedValue{Type: "record", RecordRef: ref("Subject"), Record: map[string]TypedValue{"id": text("subject-1"), "tenantId": text("tenant-alpha"), "role": text(domain.role), "active": boolean(true)}}
			resource := TypedValue{Type: "record", RecordRef: ref("Resource"), Record: map[string]TypedValue{"id": text("resource-1"), "tenantId": text("tenant-alpha"), "scope": text(domain.scope)}}
			switch j {
			case 1:
				resource.Record["tenantId"] = text("tenant-beta")
			case 2:
				subject.Record["role"] = text("visitor")
			case 3:
				resource.Record["scope"] = text("other:write")
			case 4:
				subject.Record["active"] = boolean(false)
			}
			p.Tests = append(p.Tests, BehaviorCase{Name: name, Inputs: map[string]TypedValue{"subject": subject, "resource": resource}, Expected: map[string]TypedValue{"permitted": boolean(j == 0)}})
		}
		bundle, issues := PrepareComponentBundle(p, legacy.Packages[1:])
		if len(issues) != 0 {
			panic(fmt.Sprintf("invalid structured composite example: %+v", issues))
		}
		bundles = append(bundles, bundle)
	}
	return bundles
}
