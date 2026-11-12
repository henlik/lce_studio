package workflow

import (
	"fmt"
	"software-studio/internal/model"
)

// Each journal fragment is measured once before its ownership copy. The outer
// budget never serializes a growing journal merely to discover it is too large.
type projection struct {
	status, outcome, node                                        string
	variables, outputs                                           int
	tasks, taskBytes, activeTaskBytes                            int
	steps, stepBytes, receipts, receiptBytes, events, eventBytes int
	operations, invocations                                      int
	owner                                                        *host
	ownerBytes                                                   int
}

func (e *engine) project() projection {
	p := projection{status: e.result.Status, outcome: e.result.Outcome, node: e.result.CurrentNode,
		variables: size(e.result.Variables), outputs: size(e.result.Outputs),
		tasks: len(e.result.Tasks), taskBytes: e.taskBytes, steps: len(e.result.Steps), stepBytes: e.stepBytes,
		receipts: len(e.result.Receipts), receiptBytes: e.receiptBytes, events: len(e.result.Events), eventBytes: e.eventBytes,
		operations: e.result.Operations, invocations: e.result.Invocations}
	if e.active >= 0 {
		p.activeTaskBytes = e.taskSizes[e.active]
	}
	return p
}

func (e *engine) admit(p projection) error {
	if err := e.ctx.Err(); err != nil {
		return err
	}
	header := e.result
	header.Status, header.Outcome, header.CurrentNode = p.status, p.outcome, p.node
	header.Operations, header.Invocations = p.operations, p.invocations
	header.Variables, header.Outputs = map[string]model.TypedValue{}, map[string]model.TypedValue{}
	header.Task = nil
	header.Tasks = []Task{}
	header.Owners = nil
	header.Receipts = []EventReceipt{}
	header.Events = []EventResult{}
	header.Steps = []StepResult{}
	// Owners are always an array, including when there are no owners.
	base := size(header) - len("null") + len("[]")
	total := base + p.variables - 2 + p.outputs - 2 + p.taskBytes + commas(p.tasks) + p.stepBytes + commas(p.steps) + p.receiptBytes + commas(p.receipts) + p.eventBytes + commas(p.events)
	if p.activeTaskBytes > 0 {
		total += len(",\"task\":") + p.activeTaskBytes
	}
	for _, owner := range e.hosts {
		if owner == p.owner {
			total += p.ownerBytes
		} else {
			total += owner.bytes
		}
	}
	total += commas(len(e.definition.Owners))
	// While processing a first-time event, its fixed protocol envelope is
	// reserved before any task/context/state copies. It is replaced by the
	// exact historical receipt once automatic routing reaches a suspension.
	total += e.pendingEventBytes
	if total > MaxResultBytes {
		return fmt.Errorf("%w: projected evidence exceeds %d bytes", ErrLimit, MaxResultBytes)
	}
	return nil
}

func (e *engine) appendStep(step StepResult, p projection) error {
	n := size(step)
	p.steps++
	p.stepBytes += n
	if err := e.admit(p); err != nil {
		return err
	}
	step.Assignments = copyValues(step.Assignments)
	e.result.Steps = append(e.result.Steps, step)
	e.stepBytes += n
	return nil
}

func mapEntrySize(name string, valueBytes int) int { return size(name) + 1 + valueBytes }
