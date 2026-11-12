package simulation

import (
	"container/heap"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"sort"

	"software-studio/internal/model"
)

// The infrastructure engine is separate from the v1 engine so legacy models
// retain their original timing, queue order and timeout semantics.
type continuumEvent struct {
	at, order int64
	kind      string
	ref       int
	request   int
	attempt   int
}

func (v continuumEvent) environment() bool { return v.kind == "host" || v.kind == "link" }

type continuumQueue []continuumEvent

func (q continuumQueue) Len() int { return len(q) }
func (q continuumQueue) Less(i, j int) bool {
	if q[i].at != q[j].at {
		return q[i].at < q[j].at
	}
	if q[i].environment() != q[j].environment() {
		return q[i].environment()
	}
	return q[i].order < q[j].order
}
func (q continuumQueue) Swap(i, j int) { q[i], q[j] = q[j], q[i] }
func (q *continuumQueue) Push(v any)   { *q = append(*q, v.(continuumEvent)) }
func (q *continuumQueue) Pop() any {
	old := *q
	v := old[len(old)-1]
	*q = old[:len(old)-1]
	return v
}

type continuumHost struct {
	model   model.Host
	up      bool
	active  int
	waiting []int // work IDs, in arrival order
	metrics model.HostMetrics
	wattMs  float64
}

type continuumLink struct {
	model   model.NetworkLink
	up      bool
	active  int // transmitting packet ID; -1 when idle
	waiting []int
	metrics model.LinkMetrics
}

type continuumWork struct {
	node, request, attempt int
	state                  string // queued, active, finished, interrupted
}

type continuumPacket struct {
	from, to, edge, link, request, attempt int
	response                               bool
	state                                  string // queued, transmitting, propagating, delivered, dropped
}

type continuumEngine struct {
	project    model.Project
	out        model.Run
	path       []model.Node
	appLinks   []model.Edge
	routes     []workflowRoute // schema 3 only: one stable exclusive route per request
	dbWrites   [][]int         // schema 3 only: durable writes by node and request
	nodeHosts  []int
	pathLinks  []int // physical link per application edge; -1 for same host
	nodeActive []int
	hosts      []continuumHost
	links      []continuumLink
	requests   []requestState
	work       []continuumWork
	packets    []continuumPacket
	latencies  []int64
	queue      continuumQueue
	rng        *rand.Rand
	now, order int64
	err        error
}

func runContinuum(ctx context.Context, project model.Project) (model.Run, error) {
	encoded, err := json.Marshal(project)
	if err != nil {
		return model.Run{}, fmt.Errorf("encode project: %w", err)
	}
	hash := sha256.Sum256(encoded)
	e := continuumEngine{
		project: project, rng: rand.New(rand.NewSource(project.Scenario.Seed)),
		out: model.Run{ProjectID: project.ID, Project: project, ModelHash: hex.EncodeToString(hash[:]), EngineVersion: Version, Seed: project.Scenario.Seed,
			Events: []model.TraceEvent{}, Verdicts: []model.Verdict{}, Metrics: model.Metrics{Requests: project.Scenario.RequestCount}},
	}
	e.initialize()
	for len(e.queue) > 0 && e.err == nil {
		if err := ctx.Err(); err != nil {
			return model.Run{}, err
		}
		v := heap.Pop(&e.queue).(continuumEvent)
		// Cancelled work and dropped packets no longer consume simulation time.
		// Client timeout events remain part of the observation horizon.
		if e.obsolete(v) {
			continue
		}
		e.advance(v.at)
		e.handle(v)
	}
	if e.err != nil {
		return model.Run{}, e.err
	}
	if err := ctx.Err(); err != nil {
		return model.Run{}, err
	}
	e.out.Metrics.Failed = project.Scenario.RequestCount - e.out.Metrics.Completed
	e.out.Metrics.DurationMs = e.now
	if len(e.latencies) > 0 {
		sort.Slice(e.latencies, func(i, j int) bool { return e.latencies[i] < e.latencies[j] })
		e.out.Metrics.P95LatencyMs = e.latencies[(95*len(e.latencies)+99)/100-1]
		var sum int64
		for _, latency := range e.latencies {
			sum += latency
		}
		e.out.Metrics.AverageLatencyMs = float64(sum) / float64(len(e.latencies))
	}
	e.out.Verdicts = verdicts(project.Requirements, e.out.Metrics)
	if project.Infrastructure != nil {
		metrics := &model.InfrastructureMetrics{Hosts: []model.HostMetrics{}, Links: []model.LinkMetrics{}}
		for _, h := range e.hosts {
			h.metrics.EnergyWh = h.wattMs / 3600000
			metrics.Hosts = append(metrics.Hosts, h.metrics)
			metrics.EnergyWh += h.metrics.EnergyWh
		}
		for _, l := range e.links {
			metrics.Links = append(metrics.Links, l.metrics)
		}
		e.out.Infrastructure = metrics
	}
	return e.out, nil
}

func (e *continuumEngine) initialize() {
	e.initializeApplication()
	if e.project.Infrastructure == nil {
		// Schema 3 without infrastructure has independent per-node resources.
		// These private scheduler slots have no physical identity or estimates.
		for i, n := range e.path {
			e.nodeHosts = append(e.nodeHosts, i)
			e.hosts = append(e.hosts, continuumHost{model: model.Host{CPUCores: n.Config.Capacity}, up: true})
		}
		for range e.appLinks {
			e.pathLinks = append(e.pathLinks, -1)
		}
	} else {
		e.initializeInfrastructure()
	}
	e.nodeActive = make([]int, len(e.path))
	e.requests = make([]requestState, e.project.Scenario.RequestCount)
	for i := range e.requests {
		e.requests[i] = requestState{id: fmt.Sprintf("request-%03d", i+1), startedAt: int64(i) * e.project.Scenario.IntervalMs}
		e.schedule(e.requests[i].startedAt, "send", 0, i, 1)
	}
	if e.project.SchemaVersion == 3 {
		e.initializeRoutes()
	}
}

func (e *continuumEngine) initializeApplication() {
	if e.project.SchemaVersion == 3 {
		// Keep the client at index zero; other graph nodes need not be in path order.
		for _, n := range e.project.Nodes {
			if n.Type == "client" {
				e.path = append(e.path, n)
			}
		}
		for _, n := range e.project.Nodes {
			if n.Type != "client" {
				e.path = append(e.path, n)
			}
		}
		e.appLinks = append(e.appLinks, e.project.Edges...)
		return
	}
	byID := make(map[string]model.Node)
	bySource := make(map[string]model.Edge)
	client := ""
	for _, n := range e.project.Nodes {
		byID[n.ID] = n
		if n.Type == "client" {
			client = n.ID
		}
	}
	for _, l := range e.project.Edges {
		bySource[l.Source] = l
	}
	for id := client; id != ""; {
		e.path = append(e.path, byID[id])
		l, ok := bySource[id]
		if !ok {
			break
		}
		e.appLinks = append(e.appLinks, l)
		id = l.Target
	}
}

func (e *continuumEngine) initializeInfrastructure() {
	infra := e.project.Infrastructure
	hostIDs := make(map[string]int)
	for i, h := range infra.Hosts {
		hostIDs[h.ID] = i
		e.hosts = append(e.hosts, continuumHost{model: h, up: true, metrics: model.HostMetrics{HostID: h.ID}})
		for _, outage := range h.Outages {
			e.schedule(outage.StartMs, "host", i, -1, 0)
			e.schedule(outage.EndMs, "host", i, -1, 0)
		}
	}
	for i, l := range infra.Links {
		e.links = append(e.links, continuumLink{model: l, up: true, active: -1, metrics: model.LinkMetrics{LinkID: l.ID}})
		for _, outage := range l.Outages {
			e.schedule(outage.StartMs, "link", i, -1, 0)
			e.schedule(outage.EndMs, "link", i, -1, 0)
		}
	}
	placements := make(map[string]int)
	for _, p := range infra.Placements {
		placements[p.NodeID] = hostIDs[p.HostID]
	}
	for _, n := range e.path {
		e.nodeHosts = append(e.nodeHosts, placements[n.ID])
	}
	nodeIDs := make(map[string]int, len(e.path))
	for i, n := range e.path {
		nodeIDs[n.ID] = i
	}
	for _, appLink := range e.appLinks {
		from, to := nodeIDs[appLink.Source], nodeIDs[appLink.Target]
		physical := -1
		if e.nodeHosts[from] != e.nodeHosts[to] {
			a, b := e.hosts[e.nodeHosts[from]].model.ID, e.hosts[e.nodeHosts[to]].model.ID
			for j, l := range e.links {
				if (l.model.SourceHostID == a && l.model.TargetHostID == b) || (l.model.SourceHostID == b && l.model.TargetHostID == a) {
					physical = j
					break
				}
			}
		}
		e.pathLinks = append(e.pathLinks, physical)
	}
}

func (e *continuumEngine) schedule(at int64, kind string, ref, request, attempt int) {
	if e.err != nil {
		return
	}
	if at < e.now || at > MaxTimeMs {
		e.err = fmt.Errorf("%w: event at %d ms exceeds the %d ms experiment horizon", ErrLimit, at, MaxTimeMs)
		return
	}
	e.order++
	heap.Push(&e.queue, continuumEvent{at: at, order: e.order, kind: kind, ref: ref, request: request, attempt: attempt})
}

func (e *continuumEngine) obsolete(v continuumEvent) bool {
	switch v.kind {
	case "finish":
		return e.work[v.ref].state != "active"
	case "serialized":
		return e.packets[v.ref].state != "transmitting"
	case "deliver":
		return e.packets[v.ref].state != "propagating"
	}
	return false
}

func (e *continuumEngine) advance(at int64) {
	if e.project.Infrastructure == nil {
		e.now = at
		return
	}
	dt := at - e.now
	for i := range e.hosts {
		h := &e.hosts[i]
		if h.up {
			h.metrics.UptimeMs += dt
			h.metrics.BusyCoreMs += int64(h.active) * dt
			watts := h.model.IdleWatts + (h.model.ActiveWatts-h.model.IdleWatts)*float64(h.active)/float64(h.model.CPUCores)
			h.wattMs += watts * float64(dt)
		}
	}
	e.now = at
}

func (e *continuumEngine) trace(kind string, node, request, attempt, host, link int, message string) {
	if e.err != nil {
		return
	}
	if len(e.out.Events) >= MaxTraceEvents {
		e.err = fmt.Errorf("%w: more than %d trace events", ErrLimit, MaxTraceEvents)
		return
	}
	v := model.TraceEvent{Sequence: len(e.out.Events) + 1, TimeMs: e.now, Kind: kind, Attempt: attempt, Message: message}
	if node >= 0 {
		v.NodeID = e.path[node].ID
	}
	if request >= 0 {
		v.RequestID = e.requests[request].id
	}
	if host >= 0 && e.project.Infrastructure != nil {
		v.HostID = e.hosts[host].model.ID
	}
	if link >= 0 {
		v.LinkID = e.links[link].model.ID
	}
	e.out.Events = append(e.out.Events, v)
}

func available(outages []model.Outage, at int64) bool {
	for _, outage := range outages {
		if outage.StartMs <= at && at < outage.EndMs {
			return false
		}
	}
	return true
}

func (e *continuumEngine) handle(v continuumEvent) {
	switch v.kind {
	case "host":
		e.hostTransition(v.ref)
	case "link":
		e.linkTransition(v.ref)
	case "send":
		r := &e.requests[v.request]
		if r.terminal {
			return
		}
		r.latestAttempt = v.attempt
		e.out.Metrics.Attempts++
		kind := "request_sent"
		if v.attempt > 1 {
			kind = "retry_sent"
		}
		e.trace(kind, 0, v.request, v.attempt, e.nodeHosts[0], -1, fmt.Sprintf("Send attempt %d of %d.", v.attempt, e.project.Scenario.MaxRetries+1))
		e.schedule(e.now+e.project.Scenario.TimeoutMs, "timeout", 0, v.request, v.attempt)
		e.forward(0, v.request, v.attempt)
	case "timeout":
		r := &e.requests[v.request]
		if r.terminal || r.latestAttempt != v.attempt {
			return
		}
		e.trace("timeout", 0, v.request, v.attempt, e.nodeHosts[0], -1, "No successful response before the attempt deadline.")
		if v.attempt <= e.project.Scenario.MaxRetries {
			e.schedule(e.now, "send", 0, v.request, v.attempt+1)
		} else {
			r.terminal = true
			e.trace("failed", 0, v.request, v.attempt, e.nodeHosts[0], -1, "Retry budget exhausted. In-flight work is not cancelled by the client.")
		}
	case "finish":
		e.finish(v.ref)
	case "serialized":
		p := &e.packets[v.ref]
		l := &e.links[p.link]
		l.active = -1
		p.state = "propagating"
		delay := l.model.LatencyMs + e.applicationDelay(p.edge)
		e.schedule(e.now+delay, "deliver", v.ref, p.request, p.attempt)
		e.pumpLink(p.link)
	case "deliver":
		e.deliver(v.ref)
	}
}

func (e *continuumEngine) hostTransition(host int) {
	h := &e.hosts[host]
	up := available(h.model.Outages, e.now)
	if up == h.up {
		return
	}
	h.up = up
	if up {
		e.trace("host_power_restored", -1, -1, 0, host, -1, "Power restored; interrupted volatile work is not automatically replayed.")
		return
	}
	e.trace("host_power_lost", -1, -1, 0, host, -1, "Power lost; queued and active processing is interrupted. Committed database state persists.")
	for i := range e.work {
		w := &e.work[i]
		if e.nodeHosts[w.node] == host && (w.state == "queued" || w.state == "active") {
			if w.state == "active" {
				e.nodeActive[w.node]--
				h.active--
			}
			w.state = "interrupted"
			h.metrics.InterruptedJobs++
			e.trace("processing_interrupted", w.node, w.request, w.attempt, host, -1, "Volatile processing discarded by host power loss; the client may retry.")
		}
	}
	h.waiting = nil
	for i := range e.packets {
		p := e.packets[i]
		if e.nodeHosts[p.from] == host || e.nodeHosts[p.to] == host {
			e.dropPacket(i, "Transfer dropped because an endpoint lost power.")
		}
	}
	for i := range e.links {
		e.pumpLink(i)
	}
}

func (e *continuumEngine) linkTransition(link int) {
	l := &e.links[link]
	up := available(l.model.Outages, e.now)
	if l.up == up {
		return
	}
	l.up = up
	if up {
		e.trace("link_restored", -1, -1, 0, -1, link, "Network link restored; failed transfers require a new client attempt.")
		return
	}
	e.trace("link_lost", -1, -1, 0, -1, link, "Network link unavailable; queued and in-flight transfers are dropped.")
	for i := range e.packets {
		if e.packets[i].link == link {
			e.dropPacket(i, "Transfer dropped by the network outage.")
		}
	}
	l.waiting = nil
}

func (e *continuumEngine) applicationDelay(edge int) int64 {
	delay := e.appLinks[edge].LatencyMs
	if jitter := e.project.Scenario.JitterMs; jitter > 0 {
		delay += e.rng.Int63n(jitter + 1)
	}
	return delay
}

func (e *continuumEngine) transfer(from, to, edge, request, attempt int, response bool) {
	link := e.pathLinks[edge]
	id := len(e.packets)
	e.packets = append(e.packets, continuumPacket{from: from, to: to, edge: edge, link: link, request: request, attempt: attempt, response: response, state: "queued"})
	if link >= 0 {
		e.links[link].metrics.Transfers++
	}
	if !e.hosts[e.nodeHosts[from]].up || !e.hosts[e.nodeHosts[to]].up || (link >= 0 && !e.links[link].up) {
		e.dropPacket(id, "Transfer cannot start while an endpoint or physical link is unavailable.")
		return
	}
	if link < 0 {
		e.packets[id].state = "propagating"
		e.schedule(e.now+e.applicationDelay(edge), "deliver", id, request, attempt)
		return
	}
	e.links[link].waiting = append(e.links[link].waiting, id)
	e.trace("network_queued", from, request, attempt, e.nodeHosts[from], link, "Transfer admitted to the shared physical-link FIFO.")
	e.pumpLink(link)
}

func (e *continuumEngine) pumpLink(link int) {
	l := &e.links[link]
	if !l.up || l.active >= 0 {
		return
	}
	for len(l.waiting) > 0 {
		id := l.waiting[0]
		l.waiting = l.waiting[1:]
		p := &e.packets[id]
		if p.state != "queued" {
			continue
		}
		if !e.hosts[e.nodeHosts[p.from]].up || !e.hosts[e.nodeHosts[p.to]].up {
			e.dropPacket(id, "Transfer endpoint unavailable when transmission becomes eligible.")
			continue
		}
		p.state = "transmitting"
		l.active = id
		bits := e.project.Infrastructure.MessageBytes * 8
		serialization := (bits + l.model.BandwidthKbps - 1) / l.model.BandwidthKbps
		e.trace("network_transmitting", p.from, p.request, p.attempt, e.nodeHosts[p.from], link, fmt.Sprintf("Serializing %d bytes for %d ms on shared bandwidth.", e.project.Infrastructure.MessageBytes, serialization))
		e.schedule(e.now+serialization, "serialized", id, p.request, p.attempt)
		return
	}
}

func (e *continuumEngine) dropPacket(id int, message string) {
	p := &e.packets[id]
	if p.state == "delivered" || p.state == "dropped" {
		return
	}
	p.state = "dropped"
	if p.link >= 0 {
		l := &e.links[p.link]
		l.metrics.DroppedTransfers++
		if l.active == id {
			l.active = -1
		}
	}
	e.trace("transfer_dropped", p.to, p.request, p.attempt, e.nodeHosts[p.to], p.link, message)
}

func (e *continuumEngine) deliver(id int) {
	p := &e.packets[id]
	if !e.hosts[e.nodeHosts[p.from]].up || !e.hosts[e.nodeHosts[p.to]].up || (p.link >= 0 && !e.links[p.link].up) {
		e.dropPacket(id, "Transfer unavailable at delivery.")
		return
	}
	p.state = "delivered"
	if p.link >= 0 {
		e.links[p.link].metrics.BytesTransferred += e.project.Infrastructure.MessageBytes
	}
	node, request, attempt := p.to, p.request, p.attempt
	host := e.nodeHosts[node]
	if !p.response {
		e.trace("request_arrived", node, request, attempt, host, p.link, "Request arrived at "+e.path[node].Name+".")
		workID := len(e.work)
		e.work = append(e.work, continuumWork{node: node, request: request, attempt: attempt, state: "queued"})
		e.hosts[host].waiting = append(e.hosts[host].waiting, workID)
		e.pumpHost(host)
		if e.work[workID].state == "queued" {
			message := "Waiting for node capacity or a shared host CPU core."
			if e.project.Infrastructure == nil {
				message = "Waiting for node processing capacity."
			}
			e.trace("queued", node, request, attempt, host, -1, message)
		}
		return
	}
	r := &e.requests[request]
	if node > 0 {
		e.trace("response_forwarded", node, request, attempt, host, p.link, "Forwarding response without additional CPU processing.")
		e.backward(node, request, attempt)
	} else if r.terminal {
		e.trace("response_ignored", 0, request, attempt, host, p.link, "Late response ignored because the client already completed or exhausted retries.")
	} else {
		r.terminal = true
		r.completed = true
		e.out.Metrics.Completed++
		latency := e.now - r.startedAt
		e.latencies = append(e.latencies, latency)
		e.trace("completed", 0, request, attempt, host, p.link, fmt.Sprintf("Client received success after %d ms including retries.", latency))
	}
}

func (e *continuumEngine) pumpHost(host int) {
	h := &e.hosts[host]
	for h.up && h.active < h.model.CPUCores {
		eligible := -1
		for i, id := range h.waiting {
			w := e.work[id]
			if w.state == "queued" && e.nodeActive[w.node] < e.path[w.node].Config.Capacity {
				eligible = i
				break
			}
		}
		if eligible < 0 {
			return
		}
		id := h.waiting[eligible]
		h.waiting = append(h.waiting[:eligible], h.waiting[eligible+1:]...)
		w := &e.work[id]
		w.state = "active"
		h.active++
		e.nodeActive[w.node]++
		message := fmt.Sprintf("Processing for %d ms; host CPU %d/%d, node slots %d/%d.", e.path[w.node].Config.ProcessingMs, h.active, h.model.CPUCores, e.nodeActive[w.node], e.path[w.node].Config.Capacity)
		if e.project.Infrastructure == nil {
			message = fmt.Sprintf("Processing for %d ms; node slots %d/%d.", e.path[w.node].Config.ProcessingMs, e.nodeActive[w.node], e.path[w.node].Config.Capacity)
		}
		e.trace("processing", w.node, w.request, w.attempt, host, -1, message)
		e.schedule(e.now+e.path[w.node].Config.ProcessingMs, "finish", id, w.request, w.attempt)
	}
}

func (e *continuumEngine) finish(id int) {
	w := &e.work[id]
	w.state = "finished"
	node, request, attempt := w.node, w.request, w.attempt
	host := e.nodeHosts[node]
	e.nodeActive[node]--
	e.hosts[host].active--
	r := &e.requests[request]
	if e.path[node].Type == "database" {
		writes := r.writes
		if e.project.SchemaVersion == 3 {
			writes = e.dbWrites[node][request]
		}
		if e.path[node].Config.Deduplicate && writes > 0 {
			e.out.Metrics.Deduplicated++
			e.trace("deduplicated", node, request, attempt, host, -1, "Durable request ID found; no additional write.")
		} else {
			writes++
			if e.project.SchemaVersion == 3 {
				e.dbWrites[node][request] = writes
			} else {
				r.writes = writes
			}
			e.out.Metrics.Writes++
			if writes > 1 {
				e.out.Metrics.DuplicateWrites++
				e.trace("duplicate_write", node, request, attempt, host, -1, "Duplicate operation committed for this request ID.")
			} else {
				e.trace("write", node, request, attempt, host, -1, "Operation committed; write and deduplication state survive host power loss.")
			}
		}
		if e.project.Scenario.DropFirstResponse && !r.dropped {
			r.dropped = true
			e.trace("response_dropped", node, request, attempt, host, -1, "First successful response lost; committed work remains.")
		} else {
			e.trace("response_sent", node, request, attempt, host, -1, "Successful response starts its return path.")
			e.backward(node, request, attempt)
		}
	} else {
		e.trace("processed", node, request, attempt, host, -1, "Processing finished; forwarding request.")
		e.forward(node, request, attempt)
	}
	e.pumpHost(host)
}
