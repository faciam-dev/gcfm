package events

import (
	"context"
	"encoding/json"

	"github.com/redis/go-redis/v9"
)

// RedisConfig configures RedisSink.
type RedisConfig struct {
	Enabled bool   `yaml:"enabled"`
	DSN     string `yaml:"dsn"`
	Channel string `yaml:"channel"`
}

// RedisSink publishes events via Redis Pub/Sub.
type RedisSink struct {
	Client  *redis.Client
	Channel string
}

// NewRedisSink returns a RedisSink based on config.
func NewRedisSink(c RedisConfig) (*RedisSink, error) {
	if !c.Enabled || c.DSN == "" {
		return nil, nil
	}
	opt, err := redis.ParseURL(c.DSN)
	if err != nil {
		return nil, err
	}
	return &RedisSink{Client: redis.NewClient(opt), Channel: c.Channel}, nil
}

func (s *RedisSink) Emit(ctx context.Context, e Event) error {
	if s == nil || s.Client == nil {
		return nil
	}
	data, err := json.Marshal(e)
	if err != nil {
		return err
	}
	return s.Client.Publish(ctx, s.Channel, data).Err()
}
