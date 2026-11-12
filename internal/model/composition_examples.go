package model

import "fmt"

// ExampleComponentBundles demonstrates one unchanged nested policy in two domains.
// All values are synthetic; this pure decision model is not an IAM boundary.
func ExampleComponentBundles() []ComponentBundle {
	boolean := func(value bool) TypedValue { return TypedValue{Type: "boolean", Boolean: &value} }
	stringValue := func(value string) TypedValue { return TypedValue{Type: "string", String: &value} }
	input := func(name string) BehaviorBinding { return BehaviorBinding{Input: name} }
	output := func(node, port string) BehaviorBinding { return BehaviorBinding{Node: node, Port: port} }
	literal := func(value string) BehaviorBinding { v := stringValue(value); return BehaviorBinding{Literal: &v} }
	ref := func(lock PackageLock) *ComponentRef {
		return &ComponentRef{PackageID: lock.Package.ID, Version: lock.Package.Version, Digest: lock.Digest}
	}
	lock := func(p ComponentPackage) PackageLock {
		digest, err := ComponentDigest(p)
		if err != nil {
			panic(fmt.Sprintf("encode bundled composite: %v", err))
		}
		return PackageLock{Package: p, Digest: digest}
	}
	base := func(id, name, description string) ComponentPackage {
		return ComponentPackage{SchemaVersion: 2, ID: id, Version: "1.0.0", Name: name, Description: description,
			Author: "LCE Studio contributors", License: "MIT", Kernel: "composite", Capabilities: []string{"design", "evaluate"}}
	}
	tenant := base("lce.same-tenant", "Same tenant", "Compares two explicit tenant identifiers as exact strings; it does not authenticate either identity.")
	tenant.Behavior = &Behavior{
		Inputs:   []BehaviorPort{{Name: "subjectTenant", Type: "string"}, {Name: "resourceTenant", Type: "string"}},
		Outputs:  []BehaviorPort{{Name: "sameTenant", Type: "boolean"}},
		Nodes:    []BehaviorNode{{ID: "compare", Op: "equal", Position: Position{X: 240, Y: 120}, Inputs: map[string]BehaviorBinding{"left": input("subjectTenant"), "right": input("resourceTenant")}}},
		Bindings: map[string]BehaviorBinding{"sameTenant": output("compare", "result")},
	}
	tenant.Tests = []BehaviorCase{
		{Name: "Same tenant", Inputs: map[string]TypedValue{"subjectTenant": stringValue("tenant-alpha"), "resourceTenant": stringValue("tenant-alpha")}, Expected: map[string]TypedValue{"sameTenant": boolean(true)}},
		{Name: "Different tenant", Inputs: map[string]TypedValue{"subjectTenant": stringValue("tenant-alpha"), "resourceTenant": stringValue("tenant-beta")}, Expected: map[string]TypedValue{"sameTenant": boolean(false)}},
	}
	tenantLock := lock(tenant)
	policy := base("lce.access-policy", "Explicit access policy", "Permits only when tenant, required role, requested scope and active status all match. This small pure decision is not production authorization enforcement.")
	policy.Behavior = &Behavior{
		Inputs:  []BehaviorPort{{Name: "subjectTenant", Type: "string"}, {Name: "resourceTenant", Type: "string"}, {Name: "role", Type: "string"}, {Name: "requiredRole", Type: "string"}, {Name: "requestedScope", Type: "string"}, {Name: "allowedScope", Type: "string"}, {Name: "active", Type: "boolean"}},
		Outputs: []BehaviorPort{{Name: "permitted", Type: "boolean"}},
		Nodes: []BehaviorNode{
			{ID: "tenant", Component: ref(tenantLock), Position: Position{X: 80, Y: 40}, Inputs: map[string]BehaviorBinding{"subjectTenant": input("subjectTenant"), "resourceTenant": input("resourceTenant")}},
			{ID: "role", Op: "equal", Position: Position{X: 80, Y: 180}, Inputs: map[string]BehaviorBinding{"left": input("role"), "right": input("requiredRole")}},
			{ID: "scope", Op: "equal", Position: Position{X: 80, Y: 320}, Inputs: map[string]BehaviorBinding{"left": input("requestedScope"), "right": input("allowedScope")}},
			{ID: "tenantRole", Op: "and", Position: Position{X: 420, Y: 110}, Inputs: map[string]BehaviorBinding{"left": output("tenant", "sameTenant"), "right": output("role", "result")}},
			{ID: "tenantRoleScope", Op: "and", Position: Position{X: 760, Y: 190}, Inputs: map[string]BehaviorBinding{"left": output("tenantRole", "result"), "right": output("scope", "result")}},
			{ID: "decision", Op: "and", Position: Position{X: 1100, Y: 220}, Inputs: map[string]BehaviorBinding{"left": output("tenantRoleScope", "result"), "right": input("active")}},
		},
		Bindings: map[string]BehaviorBinding{"permitted": output("decision", "result")},
	}
	testCases := func(role, scope string, includePolicy bool) []BehaviorCase {
		tests := make([]BehaviorCase, 0, 5)
		for i, name := range []string{"Permit", "Wrong tenant", "Wrong role", "Wrong scope", "Inactive subject"} {
			values := map[string]TypedValue{"subjectTenant": stringValue("tenant-alpha"), "resourceTenant": stringValue("tenant-alpha"), "role": stringValue(role), "requestedScope": stringValue(scope), "active": boolean(true)}
			if includePolicy {
				values["requiredRole"], values["allowedScope"] = stringValue(role), stringValue(scope)
			}
			switch i {
			case 1:
				values["resourceTenant"] = stringValue("tenant-beta")
			case 2:
				values["role"] = stringValue("visitor")
			case 3:
				values["requestedScope"] = stringValue("other:write")
			case 4:
				values["active"] = boolean(false)
			}
			tests = append(tests, BehaviorCase{Name: name, Inputs: values, Expected: map[string]TypedValue{"permitted": boolean(i == 0)}})
		}
		return tests
	}
	policy.Tests = testCases("reader", "data:read", true)
	policyLock := lock(policy)
	bundles := make([]ComponentBundle, 0, 2)
	for _, domain := range []struct{ id, name, role, scope string }{
		{"example.health-record-access", "Health record access", "clinician", "record:read"},
		{"example.school-record-access", "School record access", "teacher", "student:read"},
	} {
		p := base(domain.id, domain.name, "Reuses the exact shared access-policy component with explicit domain role and scope literals. Synthetic inputs demonstrate composition, not real authentication or security enforcement.")
		p.Behavior = &Behavior{
			Inputs:  []BehaviorPort{{Name: "subjectTenant", Type: "string"}, {Name: "resourceTenant", Type: "string"}, {Name: "role", Type: "string"}, {Name: "requestedScope", Type: "string"}, {Name: "active", Type: "boolean"}},
			Outputs: []BehaviorPort{{Name: "permitted", Type: "boolean"}},
			Nodes: []BehaviorNode{{ID: "policy", Component: ref(policyLock), Position: Position{X: 320, Y: 160}, Inputs: map[string]BehaviorBinding{
				"subjectTenant": input("subjectTenant"), "resourceTenant": input("resourceTenant"), "role": input("role"), "requiredRole": literal(domain.role), "requestedScope": input("requestedScope"), "allowedScope": literal(domain.scope), "active": input("active"),
			}}},
			Bindings: map[string]BehaviorBinding{"permitted": output("policy", "permitted")},
		}
		p.Tests = testCases(domain.role, domain.scope, false)
		bundle, issues := PrepareComponentBundle(p, []PackageLock{policyLock, tenantLock})
		if len(issues) != 0 {
			panic(fmt.Sprintf("invalid bundled composite: %+v", issues))
		}
		bundles = append(bundles, bundle)
	}
	return bundles
}
