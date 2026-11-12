package model

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

func validateInfrastructureDraft(p Project) []ValidationIssue {
	infra := p.Infrastructure
	issues := make([]ValidationIssue, 0)
	add := func(path, message string) { issues = append(issues, ValidationIssue{path, message}) }
	bound := func(path string, value, low, high int64) {
		if value < low || value > high {
			add(path, fmt.Sprintf("Must be an integer from %d to %d.", low, high))
		}
	}
	finite := func(path string, value, low, high float64) {
		if math.IsNaN(value) || math.IsInf(value, 0) || value < low || value > high {
			add(path, fmt.Sprintf("Must be finite and between %g and %g.", low, high))
		}
	}
	name := func(path, value string) {
		if strings.TrimSpace(value) == "" || len(value) > 200 || !utf8.ValidString(value) || strings.ContainsRune(value, 0) {
			add(path, "Must be nonblank valid text of at most 200 bytes.")
		}
		for _, r := range value {
			if unicode.IsControl(r) && r != '\n' && r != '\t' && r != '\r' {
				add(path, "Unsupported control character in text.")
				break
			}
		}
	}
	if infra.Sites == nil {
		add("infrastructure.sites", "Must be an array (use [] for an empty draft).")
	}
	if infra.Hosts == nil {
		add("infrastructure.hosts", "Must be an array (use [] for an empty draft).")
	}
	if infra.Links == nil {
		add("infrastructure.links", "Must be an array (use [] for an empty draft).")
	}
	if infra.Placements == nil {
		add("infrastructure.placements", "Must be an array (use [] for an empty draft).")
	}
	for _, group := range []struct {
		path       string
		count, max int
	}{
		{"infrastructure.sites", len(infra.Sites), 30},
		{"infrastructure.hosts", len(infra.Hosts), 30},
		{"infrastructure.links", len(infra.Links), 60},
		{"infrastructure.placements", len(infra.Placements), 30},
	} {
		if group.count > group.max {
			add(group.path, fmt.Sprintf("At most %d entries are supported.", group.max))
		}
	}
	bound("infrastructure.messageBytes", infra.MessageBytes, 1, 10485760)
	// Bound direct callers before inspecting possibly unbounded collections.
	if len(infra.Sites) > 30 || len(infra.Hosts) > 30 || len(infra.Links) > 60 || len(infra.Placements) > 30 {
		return issues
	}
	seen := make(map[string]bool)
	nodes := make(map[string]bool)
	for _, n := range p.Nodes {
		seen[n.ID] = true
		nodes[n.ID] = true
	}
	id := func(path, value string) {
		if !safeID.MatchString(value) {
			add(path, "Use 1–128 letters, numbers, dots, underscores or dashes, starting with a letter or number.")
		}
		if seen[value] {
			add(path, "Infrastructure IDs must be unique across sites, hosts and links and must not collide with application node IDs.")
		}
		seen[value] = true
	}
	sites := make(map[string]bool)
	for i, site := range infra.Sites {
		path := fmt.Sprintf("infrastructure.sites[%d]", i)
		id(path+".id", site.ID)
		name(path+".name", site.Name)
		finite(path+".latitude", site.Latitude, -90, 90)
		finite(path+".longitude", site.Longitude, -180, 180)
		sites[site.ID] = true
	}
	hosts := make(map[string]bool)
	for i, host := range infra.Hosts {
		path := fmt.Sprintf("infrastructure.hosts[%d]", i)
		id(path+".id", host.ID)
		name(path+".name", host.Name)
		if !sites[host.SiteID] {
			add(path+".siteId", "Site must refer to an existing site.")
		}
		if host.Tier != "device" && host.Tier != "edge" && host.Tier != "regional" && host.Tier != "cloud" {
			add(path+".tier", "Supported tiers are device, edge, regional and cloud.")
		}
		bound(path+".cpuCores", int64(host.CPUCores), 1, 128)
		bound(path+".memoryMb", host.MemoryMB, 1, 1048576)
		finite(path+".idleWatts", host.IdleWatts, 0, 1000000)
		finite(path+".activeWatts", host.ActiveWatts, 0, 1000000)
		if host.ActiveWatts < host.IdleWatts {
			add(path+".activeWatts", "Active power must be greater than or equal to idle power.")
		}
		issues = append(issues, validateOutages(path+".outages", host.Outages)...)
		hosts[host.ID] = true
	}
	pairs := make(map[string]bool)
	for i, link := range infra.Links {
		path := fmt.Sprintf("infrastructure.links[%d]", i)
		id(path+".id", link.ID)
		if !hosts[link.SourceHostID] {
			add(path+".sourceHostId", "Source must refer to an existing host.")
		}
		if !hosts[link.TargetHostID] {
			add(path+".targetHostId", "Target must refer to an existing host.")
		}
		if link.SourceHostID == link.TargetHostID {
			add(path+".targetHostId", "A physical link must connect two different hosts.")
		}
		pair := hostPair(link.SourceHostID, link.TargetHostID)
		if pairs[pair] {
			add(path, "Only one undirected physical link per host pair is supported.")
		}
		pairs[pair] = true
		bound(path+".latencyMs", link.LatencyMs, 0, 60000)
		bound(path+".bandwidthKbps", link.BandwidthKbps, 1, 10000000)
		issues = append(issues, validateOutages(path+".outages", link.Outages)...)
	}
	placed := make(map[string]bool)
	for i, placement := range infra.Placements {
		path := fmt.Sprintf("infrastructure.placements[%d]", i)
		if !nodes[placement.NodeID] {
			add(path+".nodeId", "Placement must refer to an existing application node.")
		}
		if !hosts[placement.HostID] {
			add(path+".hostId", "Placement must refer to an existing host.")
		}
		if placed[placement.NodeID] {
			add(path+".nodeId", "An application node may have only one placement.")
		}
		placed[placement.NodeID] = true
	}
	return issues
}

func validateOutages(path string, outages []Outage) []ValidationIssue {
	issues := make([]ValidationIssue, 0)
	if outages == nil {
		return append(issues, ValidationIssue{path, "Must be an array (use [] when there are no outages)."})
	}
	if len(outages) > 20 {
		return append(issues, ValidationIssue{path, "At most 20 outage windows are supported."})
	}
	for i, outage := range outages {
		if outage.StartMs < 0 || outage.StartMs >= 3600000 {
			issues = append(issues, ValidationIssue{fmt.Sprintf("%s[%d].startMs", path, i), "Start must be an integer from 0 to 3599999 ms."})
		}
		if outage.EndMs <= outage.StartMs || outage.EndMs > 3600000 {
			issues = append(issues, ValidationIssue{fmt.Sprintf("%s[%d].endMs", path, i), "End must be after start and at most 3600000 ms."})
		}
	}
	// Sort a copy: validating a model must never reorder the user's input.
	ordered := append([]Outage(nil), outages...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].StartMs < ordered[j].StartMs })
	for i := 1; i < len(ordered); i++ {
		if ordered[i].StartMs < ordered[i-1].EndMs {
			issues = append(issues, ValidationIssue{path, "Outage windows must not overlap; touching half-open windows are allowed."})
			break
		}
	}
	return issues
}

func validateInfrastructureExecution(p Project) []ValidationIssue {
	issues := make([]ValidationIssue, 0)
	infra := p.Infrastructure
	placements := make(map[string]string)
	for _, placement := range infra.Placements {
		placements[placement.NodeID] = placement.HostID
	}
	reserved := make(map[string]int64)
	for i, node := range p.Nodes {
		host, ok := placements[node.ID]
		if !ok {
			issues = append(issues, ValidationIssue{fmt.Sprintf("nodes[%d]", i), "Every application node needs a host placement before simulation."})
			continue
		}
		reserved[host] += ResidentMemoryMB(node.Config)
	}
	for i, host := range infra.Hosts {
		if reserved[host.ID] > host.MemoryMB {
			issues = append(issues, ValidationIssue{fmt.Sprintf("infrastructure.hosts[%d].memoryMb", i), fmt.Sprintf("Placed applications reserve %d MiB but host capacity is %d MiB.", reserved[host.ID], host.MemoryMB)})
		}
	}
	links := make(map[string]bool)
	for _, link := range infra.Links {
		links[hostPair(link.SourceHostID, link.TargetHostID)] = true
	}
	for i, edge := range p.Edges {
		source, sourcePlaced := placements[edge.Source]
		target, targetPlaced := placements[edge.Target]
		if sourcePlaced && targetPlaced && source != target && !links[hostPair(source, target)] {
			issues = append(issues, ValidationIssue{fmt.Sprintf("edges[%d]", i), "Nodes on different hosts require one direct physical link; multihop routing is not supported."})
		}
	}
	return issues
}

func hostPair(a, b string) string {
	if a > b {
		a, b = b, a
	}
	return a + "\x00" + b
}
