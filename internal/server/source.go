package server

import (
	"errors"
	"net/http"
	sourceexamples "software-studio/examples/source"
	"software-studio/internal/source"
	"software-studio/internal/store"
)

func (s *Server) sourceAPI(w http.ResponseWriter, r *http.Request, path string) {
	if path == "/source/example" || path == "/source/component-example" {
		if !method(w, r, http.MethodGet) {
			return
		}
		text := sourceexamples.M1
		if path == "/source/component-example" {
			text = sourceexamples.StockReservation
		}
		respond(w, 200, map[string]string{"language": source.Version, "source": text})
		return
	}
	if !method(w, r, http.MethodPost) {
		return
	}
	var body struct {
		Source string `json:"source"`
	}
	if !decode(w, r, &body) {
		return
	}
	ctx, done, ok := s.componentComputation(w, r)
	if !ok {
		return
	}
	defer done()
	compiled := source.Compile(body.Source)
	if ctx.Err() != nil {
		stateExecutionError(w, ctx, ctx.Err())
		return
	}
	if path == "/source/compile" {
		respond(w, 200, compiled)
		return
	}
	if !compiled.Valid || compiled.Component == nil {
		respond(w, 422, compiled)
		return
	}
	if path == "/source/testbench" {
		report, err := source.RunTestbench(ctx, body.Source)
		if err != nil {
			stateExecutionError(w, ctx, err)
			return
		}
		respond(w, 200, report)
		return
	}
	if !s.systemStoreReady(w) {
		return
	}
	installed, err := s.store.InstallSourceComponent(ctx, body.Source)
	if errors.Is(err, store.ErrDataContractConflict) || errors.Is(err, store.ErrComponentConflict) || errors.Is(err, store.ErrStateMachineConflict) || errors.Is(err, store.ErrDefinitionConflict) || errors.Is(err, store.ErrDataCatalogFull) || errors.Is(err, store.ErrCatalogFull) || errors.Is(err, store.ErrStateCatalogFull) || errors.Is(err, store.ErrDefinitionCapacity) {
		fail(w, 409, "Component installation conflicts with an immutable version or catalog capacity. Existing artifacts were preserved; use a new version for changed source.", nil)
		return
	}
	if s.storageError(w, err, "Source component installation") {
		return
	}
	respond(w, 200, installed)
}
