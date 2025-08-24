package widgets

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
)

// Event represents a widget change event.
type Event struct {
	Type string    `json:"type"`
	ID   string    `json:"id,omitempty"`
	TS   time.Time `json:"ts"`
}

// RedisNotifier publishes widget events to a Redis channel.
type RedisNotifier struct {
	RDB     *redis.Client
	Channel string
}

// NewRedisNotifier constructs a RedisNotifier.
func NewRedisNotifier(rdb *redis.Client, channel string) *RedisNotifier {
	return &RedisNotifier{RDB: rdb, Channel: channel}
}

// NotifyWidgetChanged publishes an upsert event.
func (n *RedisNotifier) NotifyWidgetChanged(ctx context.Context, id string) error {
	if n == nil || n.RDB == nil {
		return nil
	}
	ev := Event{Type: "upsert", ID: id, TS: time.Now().UTC()}
	b, _ := json.Marshal(ev)
	return n.RDB.Publish(ctx, n.Channel, b).Err()
}

// NotifyWidgetRemoved publishes a remove event.
func (n *RedisNotifier) NotifyWidgetRemoved(ctx context.Context, id string) error {
	if n == nil || n.RDB == nil {
		return nil
	}
	ev := Event{Type: "remove", ID: id, TS: time.Now().UTC()}
	b, _ := json.Marshal(ev)
	return n.RDB.Publish(ctx, n.Channel, b).Err()
}

// NotifyReload publishes a reload event.
func (n *RedisNotifier) NotifyReload(ctx context.Context) error {
	if n == nil || n.RDB == nil {
		return nil
	}
	ev := Event{Type: "reload", TS: time.Now().UTC()}
	b, _ := json.Marshal(ev)
	return n.RDB.Publish(ctx, n.Channel, b).Err()
}
