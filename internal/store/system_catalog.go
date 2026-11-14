package store

import (
	"context"
	"encoding/json"
	"strings"

	"software-studio/internal/model"
	"software-studio/internal/system"
)

// SystemCatalog checks typed content digests rather than trusting catalog rows.
// Table names come only from this fixed list, never from request parameters.
func (s *Store) SystemCatalog(ctx context.Context) ([]system.CatalogEntry, error) {
	entries := system.BuiltinCatalog()
	for _, catalog := range []struct{ kind, table string }{{"component", "component_packages"}, {"data", "data_contracts"}, {"state", "state_machines"}, {"messages", "delivery_models"}, {"workflow", "workflow_packages"}} {
		rows, err := s.db.QueryContext(ctx, "SELECT id,version,body,digest FROM "+catalog.table+" ORDER BY id COLLATE \"C\",version COLLATE \"C\"")
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			entry := system.CatalogEntry{Kind: catalog.kind, Source: "installed-catalog", Ready: true}
			var body []byte
			if err := rows.Scan(&entry.ID, &entry.Version, &body, &entry.Digest); err != nil {
				rows.Close()
				return nil, err
			}
			if strings.HasPrefix(entry.ID, "builtin.") {
				continue
			}
			var identity struct{ ID, Version, Name string }
			if err := json.Unmarshal(body, &identity); err != nil {
				entry.Issue = "Definition cannot be decoded"
			} else if identity.ID != entry.ID || identity.Version != entry.Version {
				entry.Issue = "Stored identity does not match definition"
			}
			entry.Name = identity.Name
			actual, issues, err := systemCatalogDigest(catalog.kind, body)
			if err != nil {
				entry.Issue = "Definition cannot be decoded"
			} else if actual != entry.Digest {
				entry.Issue = "Content digest mismatch"
			} else if len(issues) > 0 {
				entry.Issue = "Definition is invalid: " + issues[0].Message
			}
			entry.Ready = entry.Issue == ""
			entries = append(entries, entry)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
	}
	return entries, nil
}

func systemCatalogDigest(kind string, body []byte) (string, []model.ValidationIssue, error) {
	switch kind {
	case "component":
		var p model.ComponentPackage
		if err := json.Unmarshal(body, &p); err != nil {
			return "", nil, err
		}
		digest, err := model.ComponentDigest(p)
		return digest, model.ValidateComponentPackage(p), err
	case "data":
		var p model.DataContract
		if err := json.Unmarshal(body, &p); err != nil {
			return "", nil, err
		}
		digest, err := model.DataContractDigest(p)
		return digest, model.ValidateDataContractLock(model.DataContractLock{Contract: p, Digest: digest}), err
	case "state":
		var p model.StateMachine
		if err := json.Unmarshal(body, &p); err != nil {
			return "", nil, err
		}
		digest, err := model.StateMachineDigest(p)
		return digest, model.ValidateStateMachineLock(model.StateMachineLock{Machine: p, Digest: digest}), err
	case "messages":
		var p model.DeliveryModel
		if err := json.Unmarshal(body, &p); err != nil {
			return "", nil, err
		}
		digest, err := model.DeliveryModelDigest(p)
		return digest, model.ValidateDeliveryModelLock(model.DeliveryModelLock{Model: p, Digest: digest}), err
	default:
		var p model.WorkflowDefinition
		if err := json.Unmarshal(body, &p); err != nil {
			return "", nil, err
		}
		digest, err := model.WorkflowDigest(p)
		return digest, model.ValidateWorkflowLock(model.WorkflowLock{Workflow: p, Digest: digest}), err
	}
}

func (s *Store) ResolveSystem(ctx context.Context, design system.SystemDesign) (system.Resolution, error) {
	entries, err := s.SystemCatalog(ctx)
	if err != nil {
		return system.Resolution{}, err
	}
	return system.Resolve(design, entries), nil
}
