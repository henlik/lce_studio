package store

import (
	"context"
	"software-studio/internal/assembly"
	"software-studio/internal/system"
)

func (s *Store) resolveSystemCompatibility(ctx context.Context, d system.SystemDesign, c []system.CatalogEntry) (system.Resolution, error) {
	return assembly.Resolve(ctx, assemblySource{s}, d, c)
}
