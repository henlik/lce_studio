package model

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

const MaxDataContractBytes = 64 * 1024
const MaxDataFixtureBytes = 512 * 1024
const MaxDataFixtureRows = 256
const MaxDataIssueDetails = 200

func dataTupleName(fields []string) string {
	encoded, _ := json.Marshal(fields)
	return string(encoded)
}

func ValidateDataContract(c DataContract) []ValidationIssue {
	issues := make([]ValidationIssue, 0)
	add := func(path, message string) { issues = append(issues, ValidationIssue{path, message}) }
	if c.SchemaVersion != 1 {
		add("schemaVersion", "Only data contract schema version 1 is supported.")
	}
	if !safeID.MatchString(c.ID) || !strings.Contains(c.ID, ".") {
		add("id", "Use a namespaced safe identifier containing a dot.")
	}
	if len(c.Version) > 32 || !componentVersion.MatchString(c.Version) {
		add("version", "Use an exact three-part decimal version of at most 32 bytes.")
	}
	for _, field := range []struct {
		name, value string
		limit       int
		required    bool
	}{
		{"name", c.Name, 200, true}, {"description", c.Description, 4000, false}, {"author", c.Author, 200, true}, {"license", c.License, 200, true},
	} {
		if !behaviorTextValid(field.value, field.limit, field.required) {
			add(field.name, fmt.Sprintf("Must be valid text%s of at most %d bytes.", map[bool]string{true: " (not blank)", false: ""}[field.required], field.limit))
		}
	}
	if c.Records == nil || len(c.Records) < 1 || len(c.Records) > 32 {
		add("records", "Records must be an array containing 1–32 definitions.")
	}
	if c.Entities == nil || len(c.Entities) > 32 {
		add("entities", "Entities must be a nonnull array of at most 32 definitions.")
	}
	if c.Relationships == nil || len(c.Relationships) > 64 {
		add("relationships", "Relationships must be a nonnull array of at most 64 definitions.")
	}
	if len(issues) != 0 {
		return issues
	}
	records := make(map[string]map[string]string, len(c.Records))
	for i, record := range c.Records {
		at := fmt.Sprintf("records[%d]", i)
		if !behaviorName.MatchString(record.Name) {
			add(at+".name", "Record names require 1–64 letters, digits or underscores, starting with a letter.")
		}
		if _, exists := records[record.Name]; exists {
			add(at+".name", "Record names must be unique.")
		}
		fields := make(map[string]string)
		records[record.Name] = fields
		if record.Fields == nil || len(record.Fields) < 1 || len(record.Fields) > 16 {
			add(at+".fields", "Fields must be an array containing 1–16 definitions.")
			continue
		}
		for j, field := range record.Fields {
			path := fmt.Sprintf("%s.fields[%d]", at, j)
			if !behaviorName.MatchString(field.Name) {
				add(path+".name", "Field names require 1–64 letters, digits or underscores, starting with a letter.")
			}
			if _, exists := fields[field.Name]; exists {
				add(path+".name", "Field names must be unique within a record.")
			}
			if !scalarType(field.Type) {
				add(path+".type", "Record fields must be required boolean, integer or string values.")
			}
			fields[field.Name] = field.Type
		}
	}
	entities := make(map[string]DataEntity, len(c.Entities))
	entityFields := make(map[string]map[string]string, len(c.Entities))
	keys := make(map[string]map[string]bool, len(c.Entities))
	checkTuple := func(fields []string, available map[string]string, path string) bool {
		valid := true
		if fields == nil || len(fields) < 1 || len(fields) > 4 {
			add(path, "A key tuple must contain 1–4 distinct field names.")
			return false
		}
		seen := make(map[string]bool, len(fields))
		for i, field := range fields {
			at := fmt.Sprintf("%s[%d]", path, i)
			if seen[field] {
				add(at, "A field cannot appear twice in the same key tuple.")
				valid = false
			}
			seen[field] = true
			if _, exists := available[field]; !exists {
				add(at, "Key field must exist in the entity's record.")
				valid = false
			}
		}
		return valid
	}
	for i, entity := range c.Entities {
		at := fmt.Sprintf("entities[%d]", i)
		if !behaviorName.MatchString(entity.Name) {
			add(at+".name", "Entity names require 1–64 letters, digits or underscores, starting with a letter.")
		}
		if _, exists := entities[entity.Name]; exists {
			add(at+".name", "Entity names must be unique.")
		}
		entities[entity.Name] = entity
		fields, found := records[entity.Record]
		if !found {
			add(at+".record", "Entity must reference a declared record.")
		}
		entityFields[entity.Name] = fields
		entityKeys := make(map[string]bool)
		keys[entity.Name] = entityKeys
		if checkTuple(entity.PrimaryKey, fields, at+".primaryKey") {
			entityKeys[dataTupleName(entity.PrimaryKey)] = true
		}
		if entity.UniqueKeys == nil || len(entity.UniqueKeys) > 8 {
			add(at+".uniqueKeys", "Unique keys must be a nonnull array with at most eight tuples.")
		} else {
			for j, key := range entity.UniqueKeys {
				path := fmt.Sprintf("%s.uniqueKeys[%d]", at, j)
				if checkTuple(key, fields, path) {
					name := dataTupleName(key)
					if entityKeys[name] {
						add(path, "Primary and unique key tuples must not repeat.")
					}
					entityKeys[name] = true
				}
			}
		}
		if math.IsNaN(entity.Position.X) || math.IsInf(entity.Position.X, 0) || math.Abs(entity.Position.X) > 1e6 {
			add(at+".position.x", "Position must be finite within -1000000..1000000.")
		}
		if math.IsNaN(entity.Position.Y) || math.IsInf(entity.Position.Y, 0) || math.Abs(entity.Position.Y) > 1e6 {
			add(at+".position.y", "Position must be finite within -1000000..1000000.")
		}
	}
	names := make(map[string]bool)
	for i, relation := range c.Relationships {
		at := fmt.Sprintf("relationships[%d]", i)
		if !behaviorName.MatchString(relation.Name) {
			add(at+".name", "Relationship names require 1–64 letters, digits or underscores, starting with a letter.")
		}
		if names[relation.Name] {
			add(at+".name", "Relationship names must be unique.")
		}
		names[relation.Name] = true
		if _, found := entities[relation.FromEntity]; !found {
			add(at+".fromEntity", "Source must name a declared entity.")
		}
		if _, found := entities[relation.ToEntity]; !found {
			add(at+".toEntity", "Target must name a declared entity.")
		}
		fromOK := checkTuple(relation.FromFields, entityFields[relation.FromEntity], at+".fromFields")
		toOK := checkTuple(relation.ToFields, entityFields[relation.ToEntity], at+".toFields")
		if len(relation.FromFields) != len(relation.ToFields) {
			add(at, "Source and target key tuples must have the same length.")
		} else if fromOK && toOK {
			for j, field := range relation.FromFields {
				if entityFields[relation.FromEntity][field] != entityFields[relation.ToEntity][relation.ToFields[j]] {
					add(fmt.Sprintf("%s.fromFields[%d]", at, j), "Source and target key fields must have the same scalar type in tuple order.")
				}
			}
		}
		if toOK && !keys[relation.ToEntity][dataTupleName(relation.ToFields)] {
			add(at+".toFields", "Target tuple must exactly name a declared primary or unique key.")
		}
		if relation.MinSources != 0 && relation.MinSources != 1 {
			add(at+".minSources", "Minimum sources per target must be 0 or 1.")
		}
		if relation.MaxSources != 0 && relation.MaxSources != 1 {
			add(at+".maxSources", "Maximum sources per target must be 0 (unbounded) or 1.")
		}
	}
	if len(issues) == 0 {
		encoded, err := json.Marshal(c)
		if err != nil || len(encoded) > MaxDataContractBytes {
			add("", "Data contract typed JSON must not exceed 64 KiB.")
		}
	}
	return issues
}

func DataContractDigest(c DataContract) (string, error) {
	encoded, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func ValidateDataContractLock(lock DataContractLock) []ValidationIssue {
	issues := prefixBehaviorIssues("contract", ValidateDataContract(lock.Contract))
	if !componentDigest.MatchString(lock.Digest) {
		issues = append(issues, ValidationIssue{"digest", "A data lock requires a lowercase SHA-256 digest."})
	} else if len(issues) == 0 {
		digest, err := DataContractDigest(lock.Contract)
		if err != nil || digest != lock.Digest {
			issues = append(issues, ValidationIssue{"digest", "Digest does not match the exact embedded data contract."})
		}
	}
	return issues
}

func ValidateDataTypeRef(ref DataTypeRef) []ValidationIssue {
	issues := ValidateComponentReference(ComponentRef{PackageID: ref.ContractID, Version: ref.Version, Digest: ref.Digest})
	for i := range issues {
		if issues[i].Path == "packageId" {
			issues[i].Path = "contractId"
		}
	}
	if !behaviorName.MatchString(ref.Record) {
		issues = append(issues, ValidationIssue{"record", "Reference must name a valid record identifier."})
	}
	return issues
}
