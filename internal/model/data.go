package model

// Data contracts describe values and complete fixture constraints, not database
// effects. Field order is part of the typed JSON digest encoding.
type DataField struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type DataRecord struct {
	Name   string      `json:"name"`
	Fields []DataField `json:"fields"`
}

type DataEntity struct {
	Name       string     `json:"name"`
	Record     string     `json:"record"`
	PrimaryKey []string   `json:"primaryKey"`
	UniqueKeys [][]string `json:"uniqueKeys"`
	Position   Position   `json:"position"`
}

type DataRelationship struct {
	Name       string   `json:"name"`
	FromEntity string   `json:"fromEntity"`
	FromFields []string `json:"fromFields"`
	ToEntity   string   `json:"toEntity"`
	ToFields   []string `json:"toFields"`
	MinSources int      `json:"minSources"`
	MaxSources int      `json:"maxSources"`
}

type DataContract struct {
	SchemaVersion int                `json:"schemaVersion"`
	ID            string             `json:"id"`
	Version       string             `json:"version"`
	Name          string             `json:"name"`
	Description   string             `json:"description"`
	Author        string             `json:"author"`
	License       string             `json:"license"`
	Records       []DataRecord       `json:"records"`
	Entities      []DataEntity       `json:"entities"`
	Relationships []DataRelationship `json:"relationships"`
}

type DataContractLock struct {
	Contract DataContract `json:"contract"`
	Digest   string       `json:"digest"`
}

type DataTypeRef struct {
	ContractID string `json:"contractId"`
	Version    string `json:"version"`
	Digest     string `json:"digest"`
	Record     string `json:"record"`
}

type DataFixture struct {
	Name string                             `json:"name"`
	Rows map[string][]map[string]TypedValue `json:"rows"`
}

type DataCheckResult struct {
	EngineVersion   string            `json:"engineVersion"`
	Valid           bool              `json:"valid"`
	Issues          []ValidationIssue `json:"issues"`
	IssuesTruncated bool              `json:"issuesTruncated"`
	EntityCount     int               `json:"entityCount"`
	RowCount        int               `json:"rowCount"`
}

type DataChange struct {
	Path    string `json:"path"`
	Kind    string `json:"kind"`
	Message string `json:"message"`
}

type DataCompatibility struct {
	Compatible bool         `json:"compatible"`
	Direction  string       `json:"direction"`
	Changes    []DataChange `json:"changes"`
}

type DataExample struct {
	Contract DataContractLock `json:"contract"`
	Fixture  DataFixture      `json:"fixture"`
}
