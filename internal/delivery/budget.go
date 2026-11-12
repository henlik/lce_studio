package delivery

import (
	"encoding/json"
	"fmt"
)

func jsonSize(value any) (int, error) { encoded, err := json.Marshal(value); return len(encoded), err }
func commas(count int) int {
	if count == 0 {
		return 0
	}
	return count - 1
}

// Message and state fragments are individually bounded before these counters
// are used. Never encode the accumulated trace or clone amplified snapshots to
// discover an over-budget result afterward.
func (e *engine) checkBudget(extraStepBytes, extraSteps int, replaceHost *host, hostBytes int, messageIndex, messageBytes int, operations, invocations int) error {
	header := e.result
	header.Operations += operations
	header.Invocations += invocations
	header.Instances, header.Messages, header.Steps = emptyInstances(), []Message{}, []StepResult{}
	header.Expectations, header.MessageExpectations = emptyExpectations(), []MessageExpectationResult{}
	total, err := jsonSize(header)
	if err != nil {
		return err
	}
	total += e.stepBytes + extraStepBytes + commas(len(e.result.Steps)+extraSteps) + commas(len(e.model.Instances))
	for _, h := range e.hosts {
		if h == replaceHost {
			total += hostBytes
		} else {
			total += h.bytes
		}
	}
	messageCount := len(e.messages)
	for i, n := range e.messageBytes {
		if i == messageIndex {
			total += messageBytes
		} else {
			total += n
		}
	}
	if messageIndex == len(e.messages) {
		total += messageBytes
		messageCount++
	}
	total += commas(messageCount)
	for _, expected := range e.result.Expectations {
		n, err := jsonSize(expected)
		if err != nil {
			return err
		}
		total += n
	}
	for _, expected := range e.result.MessageExpectations {
		n, err := jsonSize(expected)
		if err != nil {
			return err
		}
		total += n
	}
	total += commas(len(e.result.Expectations)) + commas(len(e.result.MessageExpectations))
	if total > MaxResultBytes {
		return fmt.Errorf("%w: serialized result exceeds %d bytes", ErrLimit, MaxResultBytes)
	}
	return nil
}

func (e *engine) retain(step StepResult, candidate *Message, index int) error {
	if err := e.ctx.Err(); err != nil {
		return err
	}
	n, err := jsonSize(step)
	if err != nil {
		return err
	}
	messageBytes := 0
	if candidate != nil {
		messageBytes, err = jsonSize(candidate)
		if err != nil {
			return err
		}
	} else {
		index = -1
	}
	if err := e.checkBudget(n, 1, nil, 0, index, messageBytes, 0, 0); err != nil {
		return err
	}
	if candidate != nil {
		e.publishMessage(cloneMessage(*candidate), index, messageBytes)
	}
	e.result.Steps = append(e.result.Steps, step)
	e.stepBytes += n
	return e.ctx.Err()
}

func (e *engine) publishMessage(message Message, index, bytes int) {
	if index == len(e.messages) {
		e.byName[message.Name] = index
		e.byIntentRoute[intentRoute{message.SourceIntentID, message.RouteID}] = index
		e.messages = append(e.messages, message)
		e.messageBytes = append(e.messageBytes, bytes)
	} else {
		e.messages[index], e.messageBytes[index] = message, bytes
	}
}
