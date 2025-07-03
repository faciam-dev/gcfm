//go:build integration
// +build integration

package events_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/faciam-dev/gcfm/internal/events"
)

func TestRedisSink(t *testing.T) {
	s := miniredis.RunT(t)
	opt := &redis.Options{Addr: s.Addr()}
	cli := redis.NewClient(opt)
	sink := &events.RedisSink{Client: cli, Channel: "cf"}
	sub := cli.Subscribe(context.Background(), "cf")
	defer sub.Close()
	if _, err := sub.Receive(context.Background()); err != nil {
		t.Fatalf("sub: %v", err)
	}
	evt := events.Event{Name: "n"}
	if err := sink.Emit(context.Background(), evt); err != nil {
		t.Fatalf("emit: %v", err)
	}
	select {
	case <-sub.Channel():
	case <-time.After(time.Second):
		t.Fatalf("timeout")
	}
}
