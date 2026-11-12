package model

import "fmt"

func workflowAdjacency(p Project) (map[string][]string, map[string][]string) {
	out, in := make(map[string][]string), make(map[string][]string)
	for _, edge := range p.Edges {
		out[edge.Source] = append(out[edge.Source], edge.Target)
		in[edge.Target] = append(in[edge.Target], edge.Source)
	}
	return out, in
}

// Incomplete drafts remain editable, but a malformed graph is never accepted.
func validateWorkflowDraft(p Project) []ValidationIssue {
	issues := make([]ValidationIssue, 0)
	pairs := make(map[[2]string]bool)
	for i, edge := range p.Edges {
		path := fmt.Sprintf("edges[%d]", i)
		if edge.Source == edge.Target {
			issues = append(issues, ValidationIssue{path, "Workflow self-links are not supported."})
		}
		pair := [2]string{edge.Source, edge.Target}
		if pairs[pair] {
			issues = append(issues, ValidationIssue{path, "Only one application edge may connect the same source and target."})
		}
		pairs[pair] = true
	}
	out, _ := workflowAdjacency(p)
	state := make(map[string]int)
	var cyclic func(string) bool
	cyclic = func(id string) bool {
		if state[id] == 1 {
			return true
		}
		if state[id] == 2 {
			return false
		}
		state[id] = 1
		for _, next := range out[id] {
			if cyclic(next) {
				return true
			}
		}
		state[id] = 2
		return false
	}
	for _, node := range p.Nodes {
		if cyclic(node.ID) {
			issues = append(issues, ValidationIssue{"edges", "Workflow cycles are not supported."})
			break
		}
	}
	return issues
}

func validateWorkflowExecution(p Project) []ValidationIssue {
	issues := make([]ValidationIssue, 0)
	add := func(path, message string) { issues = append(issues, ValidationIssue{path, message}) }
	out, in := workflowAdjacency(p)
	clients, processing, databases := 0, 0, 0
	client := ""
	stores := make([]string, 0)
	for i, node := range p.Nodes {
		path := fmt.Sprintf("nodes[%d]", i)
		switch node.Type {
		case "client":
			clients++
			client = node.ID
			if len(in[node.ID]) != 0 || len(out[node.ID]) != 1 {
				add(path, "A client needs no incoming edges and exactly one outgoing edge.")
			}
		case "service":
			processing++
			if len(in[node.ID]) < 1 || len(out[node.ID]) != 1 {
				add(path, "A service needs one or more incoming edges and exactly one outgoing edge.")
			}
		case "router":
			processing++
			if len(in[node.ID]) < 1 || len(out[node.ID]) < 2 {
				add(path, "An exclusive router needs one or more incoming edges and at least two outgoing edges.")
			}
		case "database":
			databases++
			stores = append(stores, node.ID)
			if len(in[node.ID]) < 1 || len(out[node.ID]) != 0 {
				add(path, "A terminal database needs one or more incoming edges and no outgoing edges.")
			}
		}
	}
	if clients != 1 {
		add("nodes", "A workflow requires exactly one client.")
	}
	if processing == 0 {
		add("nodes", "A workflow requires at least one service or router.")
	}
	if databases == 0 {
		add("nodes", "A workflow requires at least one terminal database.")
	}
	reachable := func(seeds []string, adjacency map[string][]string) map[string]bool {
		seen := make(map[string]bool)
		queue := append([]string(nil), seeds...)
		for len(queue) > 0 {
			id := queue[0]
			queue = queue[1:]
			if seen[id] {
				continue
			}
			seen[id] = true
			queue = append(queue, adjacency[id]...)
		}
		return seen
	}
	if clients == 1 && len(reachable([]string{client}, out)) != len(p.Nodes) {
		add("edges", "Every workflow node must be reachable from the client.")
	}
	if len(reachable(stores, in)) != len(p.Nodes) {
		add("edges", "Every workflow node must lie on a path to a terminal database.")
	}
	return issues
}
