package widgets

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"

	widgetsrepo "github.com/faciam-dev/gcfm/internal/repository/widgets"
	"github.com/faciam-dev/gcfm/internal/util"
)

// Repo defines database operations required by the subscriber.
type Repo interface {
	GetByID(ctx context.Context, id string) (widgetsrepo.Row, error)
	List(ctx context.Context, f widgetsrepo.Filter) ([]widgetsrepo.Row, int, error)
}

// RedisSubscriber consumes widget events from Redis and updates the registry.
type RedisSubscriber struct {
	RDB          *redis.Client
	Channel      string
	Repo         Repo
	Reg          Registry
	Logger       *slog.Logger
	BackoffMS    int
	BackoffMaxMS int
}

// Event represents a widget event message.
type message struct {
	Type string    `json:"type"`
	ID   string    `json:"id,omitempty"`
	TS   time.Time `json:"ts"`
}

// Start begins consuming events in a background goroutine.
func (s *RedisSubscriber) Start(ctx context.Context) (stop func()) {
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		defer cancel()
		backoff := time.Duration(s.BackoffMS) * time.Millisecond
		max := time.Duration(s.BackoffMaxMS) * time.Millisecond
		for {
			if err := s.loop(ctx); err != nil && s.Logger != nil {
				s.Logger.Warn("redis subscribe loop error", "err", err)
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
				backoff *= 2
				if backoff > max {
					backoff = max
				}
			}
		}
	}()
	return func() { cancel() }
}

func (s *RedisSubscriber) loop(ctx context.Context) error {
	sub := s.RDB.Subscribe(ctx, s.Channel)
	ch := sub.Channel()
	if _, err := s.RDB.Ping(ctx).Result(); err != nil {
		return err
	}
	if s.Logger != nil {
		s.Logger.Info("subscribed", "channel", s.Channel)
	}
	for {
		select {
		case <-ctx.Done():
			_ = sub.Close()
			return nil
		case msg, ok := <-ch:
			if !ok {
				_ = sub.Close()
				return context.Canceled
			}
			var ev message
			if err := json.Unmarshal([]byte(msg.Payload), &ev); err != nil {
				if s.Logger != nil {
					s.Logger.Warn("invalid payload", "payload", msg.Payload, "err", err)
				}
				continue
			}
			switch ev.Type {
			case "upsert":
				row, err := s.Repo.GetByID(ctx, ev.ID)
				if err != nil {
					_ = s.Reg.Remove(ctx, ev.ID)
					continue
				}
				_ = s.Reg.Upsert(ctx, toWidget(row))
			case "remove":
				_ = s.Reg.Remove(ctx, ev.ID)
			case "reload":
				rows, _, err := s.Repo.List(ctx, widgetsrepo.Filter{})
				if err != nil {
					if s.Logger != nil {
						s.Logger.Warn("reload list failed", "err", err)
					}
					continue
				}
				ups := make([]Widget, 0, len(rows))
				for _, r := range rows {
					ups = append(ups, toWidget(r))
				}
				_, _, _ = s.Reg.ApplyDiff(ctx, ups, nil)
			default:
				if s.Logger != nil {
					s.Logger.Warn("unknown event type", "type", ev.Type)
				}
			}
		}
	}
}

func toWidget(r widgetsrepo.Row) Widget {
	return Widget{
		ID:           r.ID,
		Name:         r.Name,
		Version:      r.Version,
		Type:         r.Type,
		Scopes:       r.Scopes,
		Enabled:      r.Enabled,
		Description:  util.Deref(r.Description),
		Capabilities: r.Capabilities,
		Homepage:     util.Deref(r.Homepage),
		Meta:         r.Meta,
		Tenants:      r.Tenants,
		UpdatedAt:    r.UpdatedAt,
	}
}
