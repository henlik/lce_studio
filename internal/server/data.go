package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"software-studio/internal/model"
	"software-studio/internal/store"
)

func (s *Server) dataExamples(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}
	respond(w, http.StatusOK, model.ExampleDataFixtures())
}

func (s *Server) structuredPrimitives(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}
	respond(w, http.StatusOK, model.StructuredPrimitiveSpecs())
}

func (s *Server) structuredExamples(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}
	respond(w, http.StatusOK, model.ExampleStructuredComponentBundles())
}

func (s *Server) dataPrepare(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPost) {
		return
	}
	var contract model.DataContract
	if !decode(w, r, &contract) {
		return
	}
	if issues := model.ValidateDataContract(contract); len(issues) > 0 {
		fail(w, 422, "Data contract contains invalid definitions.", issues)
		return
	}
	digest, err := model.DataContractDigest(contract)
	if err != nil {
		fail(w, 500, "Could not encode data contract.", nil)
		return
	}
	respond(w, 200, model.DataContractLock{Contract: contract, Digest: digest})
}

func (s *Server) dataValidateLock(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPost) {
		return
	}
	var lock model.DataContractLock
	if !decode(w, r, &lock) {
		return
	}
	issues := model.ValidateDataContractLock(lock)
	if issues == nil {
		issues = []model.ValidationIssue{}
	}
	var canonical *model.DataContractLock
	if len(issues) == 0 {
		canonical = &lock
	}
	respond(w, 200, struct {
		Valid  bool                    `json:"valid"`
		Issues []model.ValidationIssue `json:"issues"`
		Lock   *model.DataContractLock `json:"lock,omitempty"`
	}{len(issues) == 0, issues, canonical})
}

func validDataLock(w http.ResponseWriter, lock model.DataContractLock) bool {
	if issues := model.ValidateDataContractLock(lock); len(issues) > 0 {
		fail(w, 422, "Data contract lock is invalid.", issues)
		return false
	}
	return true
}

func (s *Server) dataContracts(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		records, err := s.store.DataContracts(r.Context())
		if s.storageError(w, err, "Data catalog") {
			return
		}
		respond(w, 200, records)
	case http.MethodPost:
		var lock model.DataContractLock
		if !decode(w, r, &lock) || !validDataLock(w, lock) {
			return
		}
		record, created, err := s.store.InstallDataContract(r.Context(), lock.Contract)
		if errors.Is(err, store.ErrDataContractConflict) {
			fail(w, 409, "This data contract ID and version are already installed with different content. Publish a new version instead.", nil)
			return
		}
		if errors.Is(err, store.ErrDataCatalogFull) {
			fail(w, 409, "The data catalog supports at most 200 installed versions.", nil)
			return
		}
		if s.storageError(w, err, "Data catalog") {
			return
		}
		status := http.StatusOK
		if created {
			status = http.StatusCreated
		}
		respond(w, status, record)
	default:
		w.Header().Set("Allow", "GET, POST")
		fail(w, 405, "Method not allowed.", nil)
	}
}

func (s *Server) dataContract(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}
	q := r.URL.Query()
	ref := model.ComponentRef{PackageID: q.Get("id"), Version: q.Get("version"), Digest: q.Get("digest")}
	if issues := model.ValidateComponentReference(ref); len(issues) > 0 {
		fail(w, 422, "Invalid exact data contract reference.", issues)
		return
	}
	record, err := s.store.DataContract(r.Context(), ref.PackageID, ref.Version, ref.Digest)
	if s.storageError(w, err, "Data contract") {
		return
	}
	respond(w, 200, record)
}

type dataCheckRequest struct {
	Contract model.DataContractLock `json:"contract"`
	Fixture  model.DataFixture      `json:"fixture"`
}

func dataExecutionError(w http.ResponseWriter, ctx context.Context, err error) {
	if ctx.Err() != nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		fail(w, 408, "Data validation was cancelled or exceeded its execution time limit.", nil)
		return
	}
	fail(w, 422, err.Error(), nil)
}

func (s *Server) dataCheck(w http.ResponseWriter, r *http.Request, save bool) {
	if !method(w, r, http.MethodPost) {
		return
	}
	var body dataCheckRequest
	if !decode(w, r, &body) || !validDataLock(w, body.Contract) {
		return
	}
	ctx, done, ok := s.componentComputation(w, r)
	if !ok {
		return
	}
	defer done()
	if err := ctx.Err(); err != nil {
		dataExecutionError(w, ctx, err)
		return
	}
	result, err := model.CheckDataFixture(body.Contract, body.Fixture)
	if err != nil {
		dataExecutionError(w, ctx, err)
		return
	}
	if err := ctx.Err(); err != nil {
		dataExecutionError(w, ctx, err)
		return
	}
	if !save {
		respond(w, 200, struct {
			Result  model.DataCheckResult `json:"result"`
			Fixture model.DataFixture     `json:"fixture"`
		}{result, body.Fixture})
		return
	}
	id, err := newID("data-validation")
	if err != nil {
		fail(w, 500, "Could not create data validation ID.", nil)
		return
	}
	record := store.DataValidationRecord{ID: id, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), Contract: body.Contract, Fixture: body.Fixture, Result: result}
	if err = s.store.SaveDataValidation(ctx, record); err != nil {
		if ctx.Err() != nil {
			dataExecutionError(w, ctx, err)
		} else {
			s.storageError(w, err, "Data validation")
		}
		return
	}
	respond(w, 201, record)
}

func (s *Server) dataValidations(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}
	records, err := s.store.DataValidations(r.Context())
	if s.storageError(w, err, "Data validations") {
		return
	}
	respond(w, 200, records)
}

func (s *Server) dataValidation(w http.ResponseWriter, r *http.Request, id string) {
	if !validID(id) {
		fail(w, 404, "Data validation not found.", nil)
		return
	}
	if !method(w, r, http.MethodGet) {
		return
	}
	record, err := s.store.DataValidation(r.Context(), id)
	if s.storageError(w, err, "Data validation") {
		return
	}
	respond(w, 200, record)
}

func (s *Server) dataCompare(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPost) {
		return
	}
	var body struct {
		Old model.DataContractLock `json:"old"`
		New model.DataContractLock `json:"new"`
	}
	if !decode(w, r, &body) || !validDataLock(w, body.Old) || !validDataLock(w, body.New) {
		return
	}
	comparison, err := model.CompareDataContracts(body.Old, body.New)
	if err != nil {
		fail(w, 422, err.Error(), nil)
		return
	}
	respond(w, 200, comparison)
}

func (s *Server) dataValues(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPost) {
		return
	}
	var body struct {
		Contracts []model.DataContractLock    `json:"contracts"`
		Values    map[string]model.TypedValue `json:"values"`
	}
	if !decode(w, r, &body) {
		return
	}
	if body.Contracts == nil || len(body.Contracts) > 8 {
		fail(w, 422, "Contracts must be an array containing at most eight locks.", nil)
		return
	}
	issues := make([]model.ValidationIssue, 0)
	seen := make(map[string]string)
	for i, lock := range body.Contracts {
		for _, issue := range model.ValidateDataContractLock(lock) {
			issues = append(issues, model.ValidationIssue{Path: fmt.Sprintf("contracts[%d].%s", i, issue.Path), Message: issue.Message})
		}
		key := lock.Contract.ID + "\x00" + lock.Contract.Version
		if digest, ok := seen[key]; ok && digest != lock.Digest {
			issues = append(issues, model.ValidationIssue{Path: fmt.Sprintf("contracts[%d]", i), Message: "One contract ID and version cannot have conflicting digests."})
		}
		seen[key] = lock.Digest
	}
	if len(issues) == 0 {
		if body.Values == nil {
			issues = append(issues, model.ValidationIssue{Path: "values", Message: "Values must be an object; use {} when empty."})
		} else {
			for _, issue := range model.ValidateStructuredValueMap(body.Values, body.Contracts) {
				path := "values"
				if issue.Path != "" {
					path += "." + issue.Path
				}
				issues = append(issues, model.ValidationIssue{Path: path, Message: issue.Message})
			}
		}
	}
	var canonical *map[string]model.TypedValue
	if len(issues) == 0 {
		canonical = &body.Values
	}
	respond(w, 200, struct {
		Valid  bool                         `json:"valid"`
		Issues []model.ValidationIssue      `json:"issues"`
		Values *map[string]model.TypedValue `json:"values,omitempty"`
	}{len(issues) == 0, issues, canonical})
}
