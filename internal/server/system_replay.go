package server

import (
	"context"
	"errors"
	"net/http"
	"software-studio/internal/store"
	"strings"
	"time"
)

func (s *Server) systemReplayAPI(w http.ResponseWriter, r *http.Request, path string) {
	handle := func(err error) bool {
		if errors.Is(err, store.ErrNotFound) {
			fail(w, 404, "Saved execution or replay verification not found", nil)
			return true
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			fail(w, 408, "Replay interrupted; check previous verifications before retrying", nil)
			return true
		}
		return s.storageError(w, err, "Replay verification")
	}
	if strings.HasPrefix(path, "/execution-replays/") {
		if !method(w, r, http.MethodGet) {
			return
		}
		result, err := s.store.SystemReplay(r.Context(), strings.TrimPrefix(path, "/execution-replays/"))
		if handle(err) {
			return
		}
		respond(w, 200, result)
		return
	}
	id := strings.TrimSuffix(strings.TrimPrefix(path, "/executions/"), "/replays")
	if id == "" || strings.Contains(id, "/") {
		fail(w, 404, "Saved execution not found", nil)
		return
	}
	if r.Method == http.MethodGet {
		rows, err := s.store.SystemReplays(r.Context(), id)
		if handle(err) {
			return
		}
		respond(w, 200, rows)
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "GET, POST")
		fail(w, 405, "Method not allowed", nil)
		return
	}
	// No supplied design, input or report can replace the server's saved snapshot.
	var body *struct{}
	if !decode(w, r, &body) {
		return
	}
	if body == nil {
		fail(w, 400, "Replay requires an empty JSON object", nil)
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
	replayID, err := newID("replay")
	if err != nil {
		fail(w, 500, "Could not identify replay", nil)
		return
	}
	result, err := s.store.ReplaySystemExecution(ctx, replayID, id)
	if handle(err) {
		return
	}
	respond(w, 200, result)
}
