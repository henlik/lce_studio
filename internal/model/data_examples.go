package model

// ExampleDataFixtures returns independently owned healthcare and education
// datasets using the exact same domain-neutral data contract.
func ExampleDataFixtures() []DataExample {
	contract := DataContract{
		SchemaVersion: 1, ID: "lce.organization-registry", Version: "1.0.0", Name: "Organization registry",
		Description: "Required scalar organization, subject and resource records with explicit entity identity and tenant references. Synthetic fixtures demonstrate data integrity, not production authorization or tenancy enforcement.",
		Author:      "LCE Studio contributors", License: "MIT",
		Records: []DataRecord{
			{Name: "Organization", Fields: []DataField{{Name: "id", Type: "string"}, {Name: "name", Type: "string"}}},
			{Name: "Subject", Fields: []DataField{{Name: "id", Type: "string"}, {Name: "tenantId", Type: "string"}, {Name: "role", Type: "string"}, {Name: "active", Type: "boolean"}}},
			{Name: "Resource", Fields: []DataField{{Name: "id", Type: "string"}, {Name: "tenantId", Type: "string"}, {Name: "scope", Type: "string"}}},
		},
		Entities: []DataEntity{
			{Name: "Organizations", Record: "Organization", PrimaryKey: []string{"id"}, UniqueKeys: [][]string{}, Position: Position{X: 370, Y: 140}},
			{Name: "Subjects", Record: "Subject", PrimaryKey: []string{"id"}, UniqueKeys: [][]string{}, Position: Position{X: 40, Y: 70}},
			{Name: "Resources", Record: "Resource", PrimaryKey: []string{"id"}, UniqueKeys: [][]string{}, Position: Position{X: 700, Y: 70}},
		},
		Relationships: []DataRelationship{
			{Name: "SubjectOrganization", FromEntity: "Subjects", FromFields: []string{"tenantId"}, ToEntity: "Organizations", ToFields: []string{"id"}, MinSources: 0, MaxSources: 0},
			{Name: "ResourceOrganization", FromEntity: "Resources", FromFields: []string{"tenantId"}, ToEntity: "Organizations", ToFields: []string{"id"}, MinSources: 0, MaxSources: 0},
		},
	}
	digest, err := DataContractDigest(contract)
	if err != nil {
		panic(err)
	}
	text := func(s string) TypedValue { return TypedValue{Type: "string", String: &s} }
	boolean := func(b bool) TypedValue { return TypedValue{Type: "boolean", Boolean: &b} }
	result := make([]DataExample, 0, 2)
	for _, domain := range []struct{ name, organization, role, scope string }{{"Healthcare registry", "Example hospital", "clinician", "record:read"}, {"Education registry", "Example school", "teacher", "student:read"}} {
		// Construct all field maps separately so editing one returned example can
		// never mutate the other domain's data or shared contract value.
		lock := DataContractLock{Contract: cloneDataContract(contract), Digest: digest}
		fixture := DataFixture{Name: domain.name, Rows: map[string][]map[string]TypedValue{
			"Organizations": {{"id": text("tenant-alpha"), "name": text(domain.organization)}, {"id": text("tenant-beta"), "name": text("Independent organization")}},
			"Subjects":      {{"id": text("subject-1"), "tenantId": text("tenant-alpha"), "role": text(domain.role), "active": boolean(true)}, {"id": text("subject-2"), "tenantId": text("tenant-beta"), "role": text(domain.role), "active": boolean(false)}},
			"Resources":     {{"id": text("resource-1"), "tenantId": text("tenant-alpha"), "scope": text(domain.scope)}, {"id": text("resource-2"), "tenantId": text("tenant-beta"), "scope": text(domain.scope)}},
		}}
		result = append(result, DataExample{Contract: lock, Fixture: fixture})
	}
	return result
}

func cloneDataContract(c DataContract) DataContract {
	out := c
	out.Records = make([]DataRecord, len(c.Records))
	for i, r := range c.Records {
		out.Records[i] = r
		out.Records[i].Fields = append([]DataField{}, r.Fields...)
	}
	out.Entities = make([]DataEntity, len(c.Entities))
	for i, e := range c.Entities {
		out.Entities[i] = e
		out.Entities[i].PrimaryKey = append([]string{}, e.PrimaryKey...)
		out.Entities[i].UniqueKeys = make([][]string, len(e.UniqueKeys))
		for j, key := range e.UniqueKeys {
			out.Entities[i].UniqueKeys[j] = append([]string{}, key...)
		}
	}
	out.Relationships = make([]DataRelationship, len(c.Relationships))
	for i, r := range c.Relationships {
		out.Relationships[i] = r
		out.Relationships[i].FromFields = append([]string{}, r.FromFields...)
		out.Relationships[i].ToFields = append([]string{}, r.ToFields...)
	}
	return out
}
