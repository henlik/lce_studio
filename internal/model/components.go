package model

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var componentVersion = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)
var componentDigest = regexp.MustCompile(`^[0-9a-f]{64}$`)

// IntegerParameter declares a bounded, editable parameter and its initial value.
type IntegerParameter struct {
	Default int64 `json:"default"`
	Min     int64 `json:"min"`
	Max     int64 `json:"max"`
}

type ComponentConfig struct {
	ProcessingMs IntegerParameter `json:"processingMs"`
	Capacity     IntegerParameter `json:"capacity"`
	MemoryMB     IntegerParameter `json:"memoryMb"`
	Deduplicate  bool             `json:"deduplicate"`
}

// ComponentPackage is a declarative profile of an existing bounded kernel.
// Its field order and integer types define the typed JSON digest encoding.
type ComponentPackage struct {
	SchemaVersion int             `json:"schemaVersion"`
	ID            string          `json:"id"`
	Version       string          `json:"version"`
	Name          string          `json:"name"`
	Description   string          `json:"description"`
	Author        string          `json:"author"`
	License       string          `json:"license"`
	Kernel        string          `json:"kernel"`
	Capabilities  []string        `json:"capabilities"`
	Config        ComponentConfig `json:"config"`
}

type PackageLock struct {
	Package ComponentPackage `json:"package"`
	Digest  string           `json:"digest"`
}

type ComponentRef struct {
	PackageID string `json:"packageId"`
	Version   string `json:"version"`
	Digest    string `json:"digest"`
}

// ValidateComponentPackage checks a standalone declarative component manifest.
func ValidateComponentPackage(p ComponentPackage) []ValidationIssue {
	issues := make([]ValidationIssue, 0)
	add := func(path, message string) { issues = append(issues, ValidationIssue{path, message}) }
	text := func(path, value string, max int, required bool) {
		if (required && strings.TrimSpace(value) == "") || len(value) > max || !utf8.ValidString(value) || strings.ContainsRune(value, 0) {
			add(path, fmt.Sprintf("Must be valid text%s of at most %d bytes.", map[bool]string{true: " (not blank)", false: ""}[required], max))
		}
		for _, r := range value {
			if unicode.IsControl(r) && r != '\n' && r != '\t' && r != '\r' {
				add(path, "Unsupported control character in text.")
				break
			}
		}
	}
	if p.SchemaVersion != 1 {
		add("schemaVersion", "Only component package schema version 1 is supported.")
	}
	if !safeID.MatchString(p.ID) || !strings.Contains(p.ID, ".") {
		add("id", "Use a namespaced safe identifier containing a dot, such as example.queued-worker.")
	}
	if len(p.Version) > 32 || !componentVersion.MatchString(p.Version) {
		add("version", "Use an exact three-part decimal version, at most 32 bytes, without leading zeroes, ranges or suffixes.")
	}
	text("name", p.Name, 200, true)
	text("description", p.Description, 4000, false)
	text("author", p.Author, 200, true)
	text("license", p.License, 200, true)
	if p.Kernel != "service" && p.Kernel != "database" && p.Kernel != "router" {
		add("kernel", "Supported component kernels are service, database and router.")
	}
	if len(p.Capabilities) != 2 || p.Capabilities[0] != "design" || p.Capabilities[1] != "simulate" {
		add("capabilities", `Capabilities must be exactly ["design","simulate"] in this order.`)
	}
	for _, field := range []struct {
		name      string
		parameter IntegerParameter
		low, high int64
	}{
		{"processingMs", p.Config.ProcessingMs, 0, 60000},
		{"capacity", p.Config.Capacity, 1, 100},
		{"memoryMb", p.Config.MemoryMB, 1, 1048576},
	} {
		path := "config." + field.name
		if field.parameter.Min < field.low || field.parameter.Min > field.high {
			add(path+".min", fmt.Sprintf("Minimum must be from %d to %d.", field.low, field.high))
		}
		if field.parameter.Max < field.low || field.parameter.Max > field.high {
			add(path+".max", fmt.Sprintf("Maximum must be from %d to %d.", field.low, field.high))
		}
		if field.parameter.Min > field.parameter.Max {
			add(path, "Minimum cannot exceed maximum.")
		}
		if field.parameter.Default < field.parameter.Min || field.parameter.Default > field.parameter.Max {
			add(path+".default", "Default must be within the declared interval.")
		}
	}
	if p.Kernel != "database" && p.Config.Deduplicate {
		add("config.deduplicate", "Deduplication is supported only by database kernels.")
	}
	return issues
}

// ComponentDigest hashes the typed JSON encoding in the exact struct field order
// above. It is not RFC8785 canonicalization and does not establish publisher trust.
// ValidateComponentPackage must be called separately before accepting a package.
func ComponentDigest(p ComponentPackage) (string, error) {
	encoded, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

// ExampleComponentPackages returns independent copies of the bundled profiles.
func ExampleComponentPackages() []ComponentPackage {
	profile := func(id, name, description, kernel string, memory, processing, capacity int64, deduplicate bool) ComponentPackage {
		return ComponentPackage{
			SchemaVersion: 1, ID: id, Version: "1.0.0", Name: name,
			Description: description, Author: "LCE Studio contributors", License: "MIT", Kernel: kernel,
			Capabilities: []string{"design", "simulate"},
			Config: ComponentConfig{
				ProcessingMs: IntegerParameter{Default: processing, Min: 0, Max: 60000},
				Capacity:     IntegerParameter{Default: capacity, Min: 1, Max: 100},
				MemoryMB:     IntegerParameter{Default: memory, Min: 16, Max: 8192},
				Deduplicate:  deduplicate,
			},
		}
	}
	return []ComponentPackage{
		profile("lce.queued-worker", "Queued worker", "A bounded FIFO processing stage with configurable concurrency and resident memory.", "service", 128, 20, 2, false),
		profile("lce.durable-store", "Durable store", "A terminal store with optional request-ID deduplication; committed writes survive the modelled host outage.", "database", 256, 10, 1, true),
		profile("lce.weighted-router", "Weighted router", "An exclusive weighted choice that follows one stable route per logical request across retries.", "router", 64, 5, 2, false),
	}
}

// BranchingExampleProject reuses a worker profile on two exclusive paths.
func BranchingExampleProject() Project {
	p := ContinuumExampleProject()
	p.SchemaVersion = 3
	p.ID = "exclusive-branching"
	p.Name = "Reusable components and exclusive routes"
	p.Description = "One router selects an edge or cloud processing path per request. Both branches reuse the same worker and store packages. Retries keep their original route; weights are reproducible choices, not business rules."
	packages := ExampleComponentPackages()
	p.Packages = make([]PackageLock, len(packages))
	for i, pkg := range packages {
		digest, err := ComponentDigest(pkg)
		if err != nil {
			panic(fmt.Sprintf("encode bundled component: %v", err))
		}
		p.Packages[i] = PackageLock{Package: pkg, Digest: digest}
	}
	instance := func(id, name string, x, y float64, index int) Node {
		lock := p.Packages[index]
		config := lock.Package.Config
		return Node{ID: id, Name: name, Type: lock.Package.Kernel, Position: Position{X: x, Y: y},
			Config:    NodeConfig{ProcessingMs: config.ProcessingMs.Default, Capacity: int(config.Capacity.Default), MemoryMB: config.MemoryMB.Default, Deduplicate: config.Deduplicate},
			Component: &ComponentRef{PackageID: lock.Package.ID, Version: lock.Package.Version, Digest: lock.Digest}}
	}
	p.Nodes = []Node{
		{ID: "client", Name: "Request client", Type: "client", Position: Position{60, 190}, Config: NodeConfig{Capacity: 1, MemoryMB: 64}},
		instance("router", "Exclusive route", 300, 190, 2),
		instance("worker-edge", "Edge worker", 540, 60, 0),
		instance("store-edge", "Edge store", 780, 60, 1),
		instance("worker-cloud", "Cloud worker", 540, 320, 0),
		instance("store-cloud", "Cloud store", 780, 320, 1),
	}
	p.Edges = []Edge{
		{ID: "client-router", Source: "client", Target: "router", LatencyMs: 2},
		{ID: "router-edge", Source: "router", Target: "worker-edge", LatencyMs: 2, RouteWeight: 3},
		{ID: "router-cloud", Source: "router", Target: "worker-cloud", LatencyMs: 2, RouteWeight: 1},
		{ID: "edge-store", Source: "worker-edge", Target: "store-edge", LatencyMs: 2},
		{ID: "cloud-store", Source: "worker-cloud", Target: "store-cloud", LatencyMs: 2},
	}
	p.Infrastructure.Placements = []Placement{
		{NodeID: "client", HostID: "device-host"},
		{NodeID: "router", HostID: "edge-host"},
		{NodeID: "worker-edge", HostID: "edge-host"},
		{NodeID: "store-edge", HostID: "edge-host"},
		{NodeID: "worker-cloud", HostID: "cloud-host"},
		{NodeID: "store-cloud", HostID: "cloud-host"},
	}
	p.Scenario.TimeoutMs = 600
	p.Requirements[2].Text = "95% of requests complete within 600 ms."
	p.Requirements[2].TargetMs = 600
	return p
}
