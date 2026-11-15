package store

import (
	"context"
	"software-studio/internal/assembly"
	"software-studio/internal/system"
)

func (s *Store) CheckDefinition(ctx context.Context, d system.ComponentDefinition) (system.DefinitionCompatibility, error) {
	c, err := s.SystemCatalog(ctx)
	if err != nil {
		return system.DefinitionCompatibility{}, err
	}
	return s.checkDefinition(ctx, d, c)
}
func (s *Store) checkDefinition(ctx context.Context, d system.ComponentDefinition, c []system.CatalogEntry) (system.DefinitionCompatibility, error) {
	return assembly.CheckDefinition(ctx, assemblySource{s}, d, c)
}
