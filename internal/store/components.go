package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"software-studio/internal/model"
)

const MaxComponentVersions = 200

var (
	ErrComponentConflict = errors.New("component package version already contains different content")
	ErrCatalogFull       = errors.New("component catalog contains the maximum number of installed versions")
	ErrNotComposite      = errors.New("component reference is not a composite package")
)

// ComponentRecord is an immutable package version together with its installation
// evidence. The timestamp is not part of the package content digest.
type ComponentRecord struct {
	Package     model.ComponentPackage `json:"package"`
	Digest      string                 `json:"digest"`
	InstalledAt string                 `json:"installedAt"`
}

type rowScanner interface{ Scan(...any) error }

func scanComponent(row rowScanner) (ComponentRecord, error) {
	var record ComponentRecord
	var body []byte
	var installed time.Time
	if err := row.Scan(&body, &record.Digest, &installed); err != nil {
		return record, err
	}
	if err := json.Unmarshal(body, &record.Package); err != nil {
		return record, err
	}
	record.InstalledAt = installed.UTC().Format(time.RFC3339Nano)
	return record, nil
}

// Components returns the complete bounded catalog, including every installed
// version. Bytewise ordering is stable across PostgreSQL locale configurations.
func (s *Store) Components(ctx context.Context) ([]ComponentRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT body,digest,installed_at FROM component_packages ORDER BY id COLLATE "C", version COLLATE "C"`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	records := make([]ComponentRecord, 0)
	for rows.Next() {
		record, err := scanComponent(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

// InstallComponent persists a previously validated package version. A matching
// digest is idempotent, while different content at the same ID/version is never
// substituted. The returned bool reports a new installation.
func (s *Store) InstallComponent(ctx context.Context, p model.ComponentPackage) (ComponentRecord, bool, error) {
	records, created, err := s.installComponents(ctx, []model.ComponentPackage{p})
	if err != nil {
		return ComponentRecord{}, false, err
	}
	return records[0], created, nil
}

// InstallComponentBundle installs an already validated portable closure in one
// transaction. A conflict anywhere leaves every previously absent version absent.
func (s *Store) InstallComponentBundle(ctx context.Context, bundle model.ComponentBundle) ([]ComponentRecord, bool, error) {
	packages := make([]model.ComponentPackage, len(bundle.Packages))
	for i, lock := range bundle.Packages {
		packages[i] = lock.Package
	}
	return s.installComponents(ctx, packages)
}

func (s *Store) installComponents(ctx context.Context, packages []model.ComponentPackage) ([]ComponentRecord, bool, error) {
	// Sort a copy, so neither caller-owned bundles nor manifest order are mutated.
	packages = append([]model.ComponentPackage(nil), packages...)
	sort.Slice(packages, func(i, j int) bool {
		if packages[i].ID != packages[j].ID {
			return packages[i].ID < packages[j].ID
		}
		return packages[i].Version < packages[j].Version
	})
	records := make([]ComponentRecord, len(packages))
	bodies := make([][]byte, len(packages))
	for i, p := range packages {
		digest, err := model.ComponentDigest(p)
		if err != nil {
			return nil, false, err
		}
		body, err := json.Marshal(p)
		if err != nil {
			return nil, false, err
		}
		records[i] = ComponentRecord{Package: p, Digest: digest}
		bodies[i] = body
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, false, err
	}
	defer tx.Rollback()
	// Serialize installations across processes, covering both immutability and
	// the global capacity check. The lock is automatically released on rollback.
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(1279477075, 2)`); err != nil {
		return nil, false, err
	}
	missing := make([]int, 0, len(packages))
	for i, p := range packages {
		existing, err := scanComponent(tx.QueryRowContext(ctx, `SELECT body,digest,installed_at FROM component_packages WHERE id=$1 AND version=$2`, p.ID, p.Version))
		if err == nil {
			if existing.Digest != records[i].Digest {
				return nil, false, fmt.Errorf("%w: %s@%s", ErrComponentConflict, p.ID, p.Version)
			}
			records[i] = existing
			continue
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, false, err
		}
		missing = append(missing, i)
	}
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM component_packages`).Scan(&count); err != nil {
		return nil, false, err
	}
	if count+len(missing) > MaxComponentVersions {
		return nil, false, ErrCatalogFull
	}
	// PostgreSQL stores microsecond precision. Use it in the response as well so
	// subsequent reads and identical reinstalls reproduce the same record.
	installed := time.Now().UTC().Truncate(time.Microsecond)
	for _, i := range missing {
		r := &records[i]
		if _, err := tx.ExecContext(ctx, `INSERT INTO component_packages(id,version,digest,body,installed_at) VALUES($1,$2,$3,$4,$5)`, r.Package.ID, r.Package.Version, r.Digest, string(bodies[i]), installed); err != nil {
			return nil, false, err
		}
		r.InstalledAt = installed.Format(time.RFC3339Nano)
	}
	if err := tx.Commit(); err != nil {
		return nil, false, err
	}
	return records, len(missing) > 0, nil
}

// ComponentBundle reconstructs the exact reachable closure from immutable
// installed versions. Evaluation itself never calls this method or resolves a
// reference against the catalog.
func (s *Store) ComponentBundle(ctx context.Context, root model.ComponentRef) (model.ComponentBundle, error) {
	records, err := s.Components(ctx)
	if err != nil {
		return model.ComponentBundle{}, err
	}
	key := func(id, version string) string { return id + "\x00" + version }
	byKey := make(map[string]ComponentRecord, len(records))
	for _, record := range records {
		byKey[key(record.Package.ID, record.Package.Version)] = record
	}
	bundle := model.ComponentBundle{SchemaVersion: 1, Root: root, Packages: []model.PackageLock{}}
	seen := make(map[string]bool)
	var visit func(model.ComponentRef) error
	visit = func(ref model.ComponentRef) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		k := key(ref.PackageID, ref.Version)
		record, ok := byKey[k]
		if !ok || record.Digest != ref.Digest {
			return ErrNotFound
		}
		if (record.Package.SchemaVersion != 2 && record.Package.SchemaVersion != 3) || record.Package.Behavior == nil {
			return ErrNotComposite
		}
		if seen[k] {
			return nil
		}
		seen[k] = true
		if len(seen) > 50 {
			return errors.New("stored component closure exceeds 50 definitions")
		}
		bundle.Packages = append(bundle.Packages, model.PackageLock{Package: record.Package, Digest: record.Digest})
		for _, node := range record.Package.Behavior.Nodes {
			if node.Component != nil {
				if err := visit(*node.Component); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := visit(root); err != nil {
		return model.ComponentBundle{}, err
	}
	sort.Slice(bundle.Packages, func(i, j int) bool {
		a, b := bundle.Packages[i].Package, bundle.Packages[j].Package
		if a.ID != b.ID {
			return a.ID < b.ID
		}
		return a.Version < b.Version
	})
	if issues := model.ValidateComponentBundle(bundle); len(issues) > 0 {
		return model.ComponentBundle{}, fmt.Errorf("stored component closure is invalid: %s: %s", issues[0].Path, issues[0].Message)
	}
	return bundle, nil
}
