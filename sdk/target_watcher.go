package sdk

import (
	"context"
	"time"
)

// TargetProvider fetches target configurations from an external source.
type TargetProvider interface {
	Fetch(ctx context.Context) (cfgs map[string]TargetConfig, defaultKey string, version string, err error)
}

// TargetWatcher periodically applies configuration updates from a provider.
type TargetWatcher struct {
	svc      *service
	provider TargetProvider
	interval time.Duration
	lastVer  string
	cancel   context.CancelFunc
}

// StartTargetWatcher launches a goroutine that periodically fetches target updates.
func (s *service) StartTargetWatcher(ctx context.Context, p TargetProvider, interval time.Duration) (stop func()) {
	cctx, cancel := context.WithCancel(ctx)
	w := &TargetWatcher{svc: s, provider: p, interval: interval, cancel: cancel}
	go w.loop(cctx)
	return cancel
}

func (w *TargetWatcher) loop(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cfgs, def, ver, err := w.provider.Fetch(ctx)
			if err != nil {
				w.svc.logger.Warnf("target fetch error: %v", err)
				continue
			}
			if ver != "" && ver == w.lastVer {
				continue
			}
			if err := w.svc.targets.ReplaceAll(ctx, cfgs, w.svc.connector(), def); err != nil {
				w.svc.logger.Warnf("target replace error: %v", err)
				continue
			}
			w.lastVer = ver
		}
	}
}

func (s *service) connector() Connector {
	if s.cn != nil {
		return s.cn
	}
	return defaultConnector
}
