package server

import (
	"context"
	"errors"
	"net/http"
	"software-studio/internal/store"
	"software-studio/internal/system"
	"software-studio/internal/systemrun"
	"strings"
	"time"
)

func (s *Server) systemExecutionAPI(w http.ResponseWriter, r *http.Request, path string) {
	if !s.systemStoreReady(w) {
		return
	}
	if path == "/executions" {
		if !method(w, r, http.MethodGet) {
			return
		}
		list, err := s.store.SystemExecutions(r.Context(), r.URL.Query().Get("systemId"))
		if s.storageError(w, err, "System execution history") {
			return
		}
		respond(w, 200, list)
		return
	}
	if strings.HasPrefix(path, "/executions/") {
		if !method(w, r, http.MethodGet) {
			return
		}
		e, err := s.store.SystemExecution(r.Context(), strings.TrimPrefix(path, "/executions/"))
		if errors.Is(err, store.ErrNotFound) {
			fail(w, 404, "Execution not found", nil)
			return
		}
		if s.storageError(w, err, "System execution") {
			return
		}
		respond(w, 200, e)
		return
	}
	if !method(w, r, http.MethodPost) {
		return
	}
	var body struct {
		Design   system.SystemDesign `json:"design"`
		Inputs   []systemrun.Inputs  `json:"inputs"`
		Scenario *systemrun.Scenario `json:"scenario,omitempty"`
	}
	if !decode(w, r, &body) {
		return
	}
	if body.Scenario != nil && (len(body.Inputs) > 0 || path == "/execution/prepare") {
		fail(w, 400, "Submit either a scenario or single-run inputs; preparation accepts the design only", nil)
		return
	}
	select {
	case s.runs <- struct{}{}:
		defer func() { <-s.runs }()
	default:
		fail(w, 429, "Execution capacity is busy", nil)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	handle := func(err error) bool {
		if err == nil {
			return false
		}
		var invalid *systemrun.Invalid
		if errors.As(err, &invalid) {
			fail(w, 422, invalid.Error(), invalid.Issues)
			return true
		}
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			fail(w, 408, "Execution interrupted; no successful report was returned", nil)
			return true
		}
		return false
	}
	if path == "/execution/prepare" {
		p, err := s.store.PrepareSystemExecution(ctx, body.Design)
		if handle(err) || s.storageError(w, err, "Execution preparation") {
			return
		}
		respond(w, 200, p)
		return
	}
	id, err := newID("execution")
	if err != nil {
		fail(w, 500, "Could not identify execution", nil)
		return
	}
	var e store.SystemExecution
	if body.Scenario != nil {
		e, err = s.store.ExecuteSystemScenario(ctx, id, body.Design, *body.Scenario)
	} else {
		e, err = s.store.ExecuteSystem(ctx, id, body.Design, body.Inputs)
	}
	if handle(err) {
		return
	}
	if errors.Is(err, store.ErrPureExecution) {
		fail(w, 422, err.Error(), nil)
		return
	}
	if s.storageError(w, err, "System execution") {
		return
	}
	respond(w, 200, e)
}
