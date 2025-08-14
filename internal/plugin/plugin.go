package plugin

import "context"

// Plugin represents a loadable plugin or widget.
type Plugin struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// Repository defines methods to list available plugins.
type Repository interface {
	List(ctx context.Context) ([]Plugin, error)
}

// Usecase provides plugin-related use cases.
type Usecase struct {
	Repo Repository
}

// List returns the available plugins.
func (u Usecase) List(ctx context.Context) ([]Plugin, error) {
	if u.Repo == nil {
		return nil, nil
	}
	return u.Repo.List(ctx)
}
