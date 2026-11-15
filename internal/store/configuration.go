package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"software-studio/internal/configuration"
	"time"
)

var ErrConfigurationConflict = errors.New("immutable configuration package or artifact conflict")
var ErrConfigurationCapacity = errors.New("configuration catalog capacity exceeded")

type ConfigurationInstallation struct {
	ID        string `json:"id"`
	Version   string `json:"version"`
	Digest    string `json:"digest"`
	Installed int    `json:"installed"`
	Existing  int    `json:"existing"`
}

var configurationTables = map[string]string{"behavior": "component_packages", "data": "data_contracts", "state": "state_machines", "messages": "delivery_models", "workflow": "workflow_packages", "definition": "component_definitions"}

// A source package installs its entire catalog closure and release snapshot in a
// single transaction. System/timing designs remain immutable inside the release;
// opening one in an existing canvas is an explicit draft operation.
func (s *Store) InstallConfiguration(ctx context.Context, p configuration.Package) (ConfigurationInstallation, error) {
	c, e := configuration.VerifyPackage(p)
	out := ConfigurationInstallation{ID: p.ID, Version: p.Version, Digest: p.Digest}
	if e != nil {
		return out, e
	}
	if p.Report != nil {
		if _, e = configuration.Replay(ctx, p); e != nil {
			return out, e
		}
	}
	encoded, e := json.Marshal(p)
	if e != nil {
		return out, e
	}
	if len(encoded) > 32<<20 {
		return out, fmt.Errorf("configuration package exceeds 32 MiB")
	}
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	// Match the existing installers' advisory locks, always in ascending order.
	for _, key := range []int{2, 3, 4, 5, 6, 20, 31} {
		if _, e = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(1279477075,$1)`, key); e != nil {
			return out, e
		}
	}
	var old string
	var previousBody []byte
	e = tx.QueryRowContext(ctx, `SELECT digest,body FROM configuration_packages WHERE id=$1 AND version=$2`, p.ID, p.Version).Scan(&old, &previousBody)
	if e == nil && (old != p.Digest || !json.Valid(previousBody)) {
		return out, ErrConfigurationConflict
	}
	if e != nil && !errors.Is(e, sql.ErrNoRows) {
		return out, e
	}
	releaseExists := e == nil
	if releaseExists {
		var previous configuration.Package
		if json.Unmarshal(previousBody, &previous) != nil || configuration.PackageDigest(previous) != p.Digest {
			return out, ErrConfigurationConflict
		}
	}
	counts := map[string]int{}
	missing := []configuration.Artifact{}
	for _, a := range c.Artifacts {
		table, ok := configurationTables[a.Kind]
		if !ok {
			continue
		}
		var digest string
		var body []byte
		e = tx.QueryRowContext(ctx, "SELECT digest,body FROM "+table+" WHERE id=$1 AND version=$2", a.ID, a.Version).Scan(&digest, &body)
		if e == nil {
			var stored any
			if json.Unmarshal(body, &stored) != nil {
				return out, ErrConfigurationConflict
			}
			var expected any
			_ = json.Unmarshal(a.Body, &expected)
			if digest != a.Digest || configuration.Hash(stored) != configuration.Hash(expected) {
				return out, ErrConfigurationConflict
			}
			out.Existing++
			continue
		}
		if !errors.Is(e, sql.ErrNoRows) {
			return out, e
		}
		missing = append(missing, a)
		counts[table]++
	}
	for table, added := range counts {
		var count int
		if e = tx.QueryRowContext(ctx, "SELECT count(*) FROM "+table).Scan(&count); e != nil {
			return out, e
		}
		if count+added > 200 {
			return out, ErrConfigurationCapacity
		}
	}
	if !releaseExists {
		var n int
		if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM configuration_packages`).Scan(&n); e != nil {
			return out, e
		}
		if n >= 200 {
			return out, ErrConfigurationCapacity
		}
	}
	now := time.Now().UTC()
	for _, a := range missing {
		table := configurationTables[a.Kind]
		if a.Kind == "definition" {
			_, e = tx.ExecContext(ctx, "INSERT INTO "+table+"(id,version,digest,body) VALUES($1,$2,$3,$4)", a.ID, a.Version, a.Digest, string(a.Body))
		} else {
			_, e = tx.ExecContext(ctx, "INSERT INTO "+table+"(id,version,digest,body,installed_at) VALUES($1,$2,$3,$4,$5)", a.ID, a.Version, a.Digest, string(a.Body), now)
		}
		if e != nil {
			return out, e
		}
		out.Installed++
	}
	if !releaseExists {
		if _, e = tx.ExecContext(ctx, `INSERT INTO configuration_packages(id,version,digest,body,installed_at) VALUES($1,$2,$3,$4,$5)`, p.ID, p.Version, p.Digest, string(encoded), now); e != nil {
			return out, e
		}
	}
	e = tx.Commit()
	return out, e
}
func (s *Store) ConfigurationPackage(ctx context.Context, id, v string) (configuration.Package, error) {
	var p configuration.Package
	var body []byte
	e := s.db.QueryRowContext(ctx, `SELECT body FROM configuration_packages WHERE id=$1 AND version=$2`, id, v).Scan(&body)
	if errors.Is(e, sql.ErrNoRows) {
		return p, ErrNotFound
	}
	if e != nil {
		return p, e
	}
	e = json.Unmarshal(body, &p)
	if e == nil {
		_, e = configuration.VerifyPackage(p)
	}
	return p, e
}
func (s *Store) ConfigurationPackages(ctx context.Context) ([]ConfigurationInstallation, error) {
	rows, e := s.db.QueryContext(ctx, `SELECT id,version,digest FROM configuration_packages ORDER BY id COLLATE "C",version COLLATE "C"`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []ConfigurationInstallation{}
	for rows.Next() {
		var r ConfigurationInstallation
		if e = rows.Scan(&r.ID, &r.Version, &r.Digest); e != nil {
			return nil, e
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
