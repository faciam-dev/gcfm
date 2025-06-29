package sdk

import (
	"context"

	"github.com/faciam-dev/gcfm/internal/customfield/registry/codec"
)

// Export returns registry metadata as YAML.
func (s *service) Export(ctx context.Context, cfg DBConfig) ([]byte, error) {
	metas, err := s.Scan(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return codec.EncodeYAML(metas)
}
