// Package simulation executes bounded pipeline and exclusive-workflow models.
// It uses virtual milliseconds, FIFO resource queues and a stable (time, sequence)
// event ordering. Network jitter is a seeded uniform integer in [0, jitterMs].
package simulation

import (
	"container/heap"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"sort"

	"software-studio/internal/model"
)

const (
	Version              = "0.3.0"
	MaxTraceEvents       = 100000
	MaxTimeMs      int64 = 3600000
)

// ErrLimit means the bounded experiment could not be completed. No partial run
// should be presented as a passing experiment when this error is returned.
var ErrLimit = errors.New("simulation limit exceeded")

type scheduledEvent struct {
	at                     int64
	order                  uint64
	kind                   string
	node, request, attempt int
}
type eventQueue []scheduledEvent

func (q eventQueue) Len() int { return len(q) }
func (q eventQueue) Less(i, j int) bool {
	if q[i].at == q[j].at {
		return q[i].order < q[j].order
	}
	return q[i].at < q[j].at
}
func (q eventQueue) Swap(i, j int) { q[i], q[j] = q[j], q[i] }
func (q *eventQueue) Push(v any)   { *q = append(*q, v.(scheduledEvent)) }
func (q *eventQueue) Pop() any     { old := *q; n := len(old); v := old[n-1]; *q = old[:n-1]; return v }

type requestState struct {
	id            string
	startedAt     int64
	latestAttempt int
	terminal      bool
	completed     bool
	dropped       bool
	writes        int
}
type job struct{ request, attempt int }
type resource struct {
	active  int
	waiting []job
}
type engine struct {
	project   model.Project
	out       model.Run
	path      []model.Node
	links     []model.Edge
	resources []resource
	requests  []requestState
	latencies []int64
	queue     eventQueue
	now       int64
	order     uint64
	rng       *rand.Rand
	err       error
}

// Run computes an experiment without real-time sleeps. The input is never
// mutated. IDs/timestamps belonging to storage are intentionally left to the API.
func Run(ctx context.Context, project model.Project) (model.Run, error) {
	if err := ctx.Err(); err != nil {
		return model.Run{}, err
	}
	if issues := model.Validate(project); len(issues) > 0 {
		return model.Run{}, fmt.Errorf("invalid project: %s: %s", issues[0].Path, issues[0].Message)
	}
	if project.SchemaVersion >= 2 {
		return runContinuum(ctx, project)
	}
	encoded, err := json.Marshal(project)
	if err != nil {
		return model.Run{}, fmt.Errorf("encode project: %w", err)
	}
	hash := sha256.Sum256(encoded)
	e := engine{
		project: project, rng: rand.New(rand.NewSource(project.Scenario.Seed)),
		out: model.Run{ProjectID: project.ID, ModelHash: hex.EncodeToString(hash[:]), EngineVersion: Version, Seed: project.Scenario.Seed, Project: project, Events: []model.TraceEvent{}, Verdicts: []model.Verdict{}, Metrics: model.Metrics{Requests: project.Scenario.RequestCount}},
	}
	byID := map[string]model.Node{}
	bySource := map[string]model.Edge{}
	client := ""
	for _, n := range project.Nodes {
		byID[n.ID] = n
		if n.Type == "client" {
			client = n.ID
		}
	}
	for _, edge := range project.Edges {
		bySource[edge.Source] = edge
	}
	for id := client; id != ""; {
		e.path = append(e.path, byID[id])
		link, ok := bySource[id]
		if !ok {
			break
		}
		e.links = append(e.links, link)
		id = link.Target
	}
	e.resources = make([]resource, len(e.path))
	e.requests = make([]requestState, project.Scenario.RequestCount)
	for i := range e.requests {
		e.requests[i] = requestState{id: fmt.Sprintf("request-%03d", i+1), startedAt: int64(i) * project.Scenario.IntervalMs}
		e.schedule(e.requests[i].startedAt, "send", 0, i, 1)
	}
	for len(e.queue) > 0 && e.err == nil {
		if err := ctx.Err(); err != nil {
			return model.Run{}, err
		}
		event := heap.Pop(&e.queue).(scheduledEvent)
		e.now = event.at
		e.handle(event)
	}
	if e.err != nil {
		return model.Run{}, e.err
	}
	e.out.Metrics.Failed = project.Scenario.RequestCount - e.out.Metrics.Completed
	if len(e.latencies) > 0 {
		sort.Slice(e.latencies, func(i, j int) bool { return e.latencies[i] < e.latencies[j] })
		// Nearest-rank percentile, computed in integers to avoid rounding at a boundary.
		e.out.Metrics.P95LatencyMs = e.latencies[(95*len(e.latencies)+99)/100-1]
		var total int64
		for _, latency := range e.latencies {
			total += latency
		}
		e.out.Metrics.AverageLatencyMs = float64(total) / float64(len(e.latencies))
	}
	e.out.Verdicts = verdicts(project.Requirements, e.out.Metrics)
	return e.out, nil
}

func (e *engine) schedule(at int64, kind string, node, request, attempt int) {
	if e.err != nil {
		return
	}
	if at < e.now || at > MaxTimeMs {
		e.err = fmt.Errorf("%w: event at %d ms exceeds the %d ms experiment horizon", ErrLimit, at, MaxTimeMs)
		return
	}
	e.order++
	heap.Push(&e.queue, scheduledEvent{at: at, order: e.order, kind: kind, node: node, request: request, attempt: attempt})
}
func (e *engine) trace(kind string, node, request, attempt int, message string) {
	if e.err != nil {
		return
	}
	if len(e.out.Events) >= MaxTraceEvents {
		e.err = fmt.Errorf("%w: more than %d trace events; reduce requests, retries or pipeline length", ErrLimit, MaxTraceEvents)
		return
	}
	e.out.Events = append(e.out.Events, model.TraceEvent{Sequence: len(e.out.Events) + 1, TimeMs: e.now, Kind: kind, NodeID: e.path[node].ID, RequestID: e.requests[request].id, Attempt: attempt, Message: message})
	e.out.Metrics.DurationMs = e.now
}
func (e *engine) delay(link int) int64 {
	delay := e.links[link].LatencyMs
	if jitter := e.project.Scenario.JitterMs; jitter > 0 {
		delay += e.rng.Int63n(jitter + 1)
	}
	return delay
}

func (e *engine) handle(v scheduledEvent) {
	r := &e.requests[v.request]
	s := e.project.Scenario
	switch v.kind {
	case "send":
		if r.terminal {
			return
		}
		r.latestAttempt = v.attempt
		e.out.Metrics.Attempts++
		kind := "request_sent"
		if v.attempt > 1 {
			kind = "retry_sent"
		}
		e.trace(kind, 0, v.request, v.attempt, fmt.Sprintf("Send attempt %d of %d.", v.attempt, s.MaxRetries+1))
		// Equal-time events run in scheduling order. A timeout already scheduled
		// for a deadline therefore wins over a response scheduled later at it.
		e.schedule(e.now+s.TimeoutMs, "timeout", 0, v.request, v.attempt)
		e.schedule(e.now+e.delay(0), "arrive", 1, v.request, v.attempt)
	case "arrive":
		e.trace("request_arrived", v.node, v.request, v.attempt, "Request arrived at "+e.path[v.node].Name+".")
		resource := &e.resources[v.node]
		if resource.active < e.path[v.node].Config.Capacity {
			e.start(v.node, job{v.request, v.attempt})
		} else {
			resource.waiting = append(resource.waiting, job{v.request, v.attempt})
			e.trace("queued", v.node, v.request, v.attempt, fmt.Sprintf("Waiting for capacity; queue length %d.", len(resource.waiting)))
		}
	case "finish":
		e.resources[v.node].active--
		if v.node == len(e.path)-1 {
			if e.path[v.node].Config.Deduplicate && r.writes > 0 {
				e.out.Metrics.Deduplicated++
				e.trace("deduplicated", v.node, v.request, v.attempt, "Existing request ID found; no additional write.")
			} else {
				r.writes++
				e.out.Metrics.Writes++
				if r.writes > 1 {
					e.out.Metrics.DuplicateWrites++
					e.trace("duplicate_write", v.node, v.request, v.attempt, "Duplicate operation committed for this request ID.")
				} else {
					e.trace("write", v.node, v.request, v.attempt, "Operation committed for this request ID.")
				}
			}
			if s.DropFirstResponse && !r.dropped {
				r.dropped = true
				e.trace("response_dropped", v.node, v.request, v.attempt, "First successful response lost; committed work remains.")
			} else {
				e.trace("response_sent", v.node, v.request, v.attempt, "Successful response starts the return path.")
				e.schedule(e.now+e.delay(v.node-1), "return", v.node-1, v.request, v.attempt)
			}
		} else {
			e.trace("processed", v.node, v.request, v.attempt, "Processing finished; forwarding request.")
			e.schedule(e.now+e.delay(v.node), "arrive", v.node+1, v.request, v.attempt)
		}
		resource := &e.resources[v.node]
		if len(resource.waiting) > 0 {
			next := resource.waiting[0]
			resource.waiting = resource.waiting[1:]
			e.start(v.node, next)
		}
	case "return":
		if v.node == 0 {
			if r.terminal {
				e.trace("response_ignored", 0, v.request, v.attempt, "Late response ignored because the client request is already complete or expired.")
			} else {
				r.terminal = true
				r.completed = true
				e.out.Metrics.Completed++
				latency := e.now - r.startedAt
				e.latencies = append(e.latencies, latency)
				e.trace("completed", 0, v.request, v.attempt, fmt.Sprintf("Client received success after %d ms including retries.", latency))
			}
		} else {
			e.trace("response_forwarded", v.node, v.request, v.attempt, "Forwarding response toward the client without additional processing.")
			e.schedule(e.now+e.delay(v.node-1), "return", v.node-1, v.request, v.attempt)
		}
	case "timeout":
		if r.terminal || r.latestAttempt != v.attempt {
			return
		}
		e.trace("timeout", 0, v.request, v.attempt, "No successful response before the attempt deadline.")
		if v.attempt <= s.MaxRetries {
			e.schedule(e.now, "send", 0, v.request, v.attempt+1)
		} else {
			r.terminal = true
			e.trace("failed", 0, v.request, v.attempt, "Retry budget exhausted. In-flight server work is not cancelled.")
		}
	}
}

func (e *engine) start(node int, j job) {
	e.resources[node].active++
	e.trace("processing", node, j.request, j.attempt, fmt.Sprintf("Processing for %d ms (%d/%d slots in use).", e.path[node].Config.ProcessingMs, e.resources[node].active, e.path[node].Config.Capacity))
	e.schedule(e.now+e.path[node].Config.ProcessingMs, "finish", node, j.request, j.attempt)
}

func verdicts(requirements []model.Requirement, m model.Metrics) []model.Verdict {
	result := make([]model.Verdict, 0, len(requirements))
	for _, r := range requirements {
		v := model.Verdict{RequirementID: r.ID, Text: r.Text, Rule: r.Rule}
		switch r.Rule {
		case "no_duplicates":
			v.Actual = float64(m.DuplicateWrites)
			v.Expected = 0
			v.Passed = m.DuplicateWrites == 0
			v.Message = fmt.Sprintf("%d duplicate writes; expected zero. This rule alone does not assert completion.", m.DuplicateWrites)
		case "all_complete":
			v.Actual = float64(m.Completed)
			v.Expected = float64(m.Requests)
			v.Passed = m.Completed == m.Requests
			v.Message = fmt.Sprintf("%d/%d requests received success before the client exhausted retries.", m.Completed, m.Requests)
		case "latency_p95":
			v.Actual = float64(m.P95LatencyMs)
			v.Expected = float64(r.TargetMs)
			v.Passed = m.Completed == m.Requests && m.P95LatencyMs <= r.TargetMs
			v.Message = fmt.Sprintf("Completed-request p95 is %d ms; threshold %d ms. Completion: %d/%d (all must complete).", m.P95LatencyMs, r.TargetMs, m.Completed, m.Requests)
		}
		result = append(result, v)
	}
	return result
}
