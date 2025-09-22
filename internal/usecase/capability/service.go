package capability

import (
	"context"
	"errors"
	"strings"

	"github.com/faciam-dev/gcfm/internal/domain/capability"
	"github.com/faciam-dev/gcfm/internal/monitordb"
)

// DatabaseReader exposes the subset of Repo behaviour needed by the usecase.
type DatabaseReader interface {
	Get(ctx context.Context, tenant string, id int64) (monitordb.Database, error)
}

// Service resolves driver capabilities for monitored databases.
type Service interface {
	Get(ctx context.Context, tenant string, dbID int64) (capability.Capabilities, error)
}

// ErrAdapterNotFound indicates that no adapter was registered for a driver.
var ErrAdapterNotFound = errors.New("capability adapter not found")

type service struct {
	repo     DatabaseReader
	adapters map[string]capability.Adapter
}

// New constructs the capability usecase with the provided repository and adapter registry.
func New(repo DatabaseReader, adapters map[string]capability.Adapter) Service {
	copyAdapters := make(map[string]capability.Adapter, len(adapters))
	for k, v := range adapters {
		if v != nil {
			copyAdapters[strings.ToLower(strings.TrimSpace(k))] = v
		}
	}
	return &service{repo: repo, adapters: copyAdapters}
}

func (s *service) Get(ctx context.Context, tenant string, dbID int64) (capability.Capabilities, error) {
	if s.repo == nil {
		return capability.Capabilities{}, errors.New("capability repo not configured")
	}
	db, err := s.repo.Get(ctx, tenant, dbID)
	if err != nil {
		return capability.Capabilities{}, err
	}
	drv := strings.ToLower(strings.TrimSpace(db.Driver))
	adapter, ok := s.adapters[drv]
	if !ok {
		switch drv {
		case "mongodb":
			adapter, ok = s.adapters["mongo"]
		case "mongo":
			adapter, ok = s.adapters["mongodb"]
		}
	}
	if !ok || adapter == nil {
		return capability.Capabilities{}, ErrAdapterNotFound
	}
	return adapter.Capabilities(ctx, dbID)
}
