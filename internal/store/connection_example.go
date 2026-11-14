package store

import (
	"context"
	"fmt"
	"software-studio/internal/model"
	"software-studio/internal/system"
)

func (s *Store) InstallConnectionExample(ctx context.Context) (system.ConnectionExamplePack, error) {
	example := system.ConnectionExample()
	for _, p := range example.Packages {
		b, issues := model.PrepareComponentBundle(p, nil)
		if len(issues) > 0 {
			return example, fmt.Errorf("invalid connection fixture: %v", issues)
		}
		if issues = model.ValidateComponentBundle(b); len(issues) > 0 {
			return example, fmt.Errorf("invalid connection fixture closure: %v", issues)
		}
	}
	_, _, err := s.installComponents(ctx, example.Packages)
	return example, err
}
