package simulation

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"sort"
)

// A validated schema-3 DAG has one exclusive route for each logical request.
// Routing uses no scheduler state or jitter RNG: every attempt follows these
// exact edges, so reversing them also handles graph convergence unambiguously.
type workflowRoute struct {
	nodes     []int
	edges     []int
	positions []int // position of a node in this route, or -1 when unvisited
}

func (e *continuumEngine) initializeRoutes() {
	nodeIDs := make(map[string]int, len(e.path))
	for i, n := range e.path {
		nodeIDs[n.ID] = i
	}
	outgoing := make([][]int, len(e.path))
	for i, edge := range e.appLinks {
		node := nodeIDs[edge.Source]
		outgoing[node] = append(outgoing[node], i)
	}
	for _, edges := range outgoing {
		sort.Slice(edges, func(i, j int) bool { return e.appLinks[edges[i]].ID < e.appLinks[edges[j]].ID })
	}
	e.dbWrites = make([][]int, len(e.path))
	for node, n := range e.path {
		if n.Type == "database" {
			e.dbWrites[node] = make([]int, len(e.requests))
		}
	}
	for request := range e.requests {
		route := workflowRoute{positions: make([]int, len(e.path))}
		for i := range route.positions {
			route.positions[i] = -1
		}
		for node := 0; ; {
			route.positions[node] = len(route.nodes)
			route.nodes = append(route.nodes, node)
			if e.path[node].Type == "database" {
				break
			}
			edge := outgoing[node][0]
			if e.path[node].Type == "router" {
				edge = e.chooseRoute(node, request, outgoing[node])
			}
			route.edges = append(route.edges, edge)
			node = nodeIDs[e.appLinks[edge].Target]
		}
		e.routes = append(e.routes, route)
	}
}

func (e *continuumEngine) chooseRoute(node, request int, outgoing []int) int {
	total := uint64(0)
	for _, edge := range outgoing {
		total += e.routeWeight(edge)
	}
	key := fmt.Sprintf("%d\n%s\n%s", e.project.Scenario.Seed, e.requests[request].id, e.path[node].ID)
	hash := sha256.Sum256([]byte(key))
	choice := binary.BigEndian.Uint64(hash[:8]) % total
	for _, edge := range outgoing {
		weight := e.routeWeight(edge)
		if choice < weight {
			return edge
		}
		choice -= weight
	}
	panic("validated routing weights did not select an edge")
}

func (e *continuumEngine) routeWeight(edge int) uint64 {
	weight := e.appLinks[edge].RouteWeight
	if weight == 0 {
		return 1
	}
	return uint64(weight)
}

func (e *continuumEngine) forward(node, request, attempt int) {
	if e.project.SchemaVersion != 3 {
		e.transfer(node, node+1, node, request, attempt, false)
		return
	}
	route := e.routes[request]
	position := route.positions[node]
	edge := route.edges[position]
	if e.path[node].Type == "router" {
		before := len(e.out.Events)
		e.trace("route_selected", node, request, attempt, e.nodeHosts[node], -1,
			fmt.Sprintf("Selected exclusive route %s to %s; retries retain this choice.", e.appLinks[edge].ID, e.appLinks[edge].Target))
		if len(e.out.Events) > before {
			e.out.Events[before].EdgeID = e.appLinks[edge].ID
		}
	}
	e.transfer(node, route.nodes[position+1], edge, request, attempt, false)
}

func (e *continuumEngine) backward(node, request, attempt int) {
	if e.project.SchemaVersion != 3 {
		e.transfer(node, node-1, node-1, request, attempt, true)
		return
	}
	route := e.routes[request]
	position := route.positions[node]
	e.transfer(node, route.nodes[position-1], route.edges[position-1], request, attempt, true)
}
