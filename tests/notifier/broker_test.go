package notifier_test

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/faciam-dev/gcfm/internal/customfield/notifier"
)

func TestRedisBrokerEmit(t *testing.T) {
	s := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	broker := &notifier.RedisBroker{Client: client, Channel: "cf"}
	sub := client.Subscribe(context.Background(), "cf")
	defer sub.Close()
	if err := broker.Emit(context.Background(), notifier.DiffReport{Added: 1}); err != nil {
		t.Fatalf("emit: %v", err)
	}
	msg := <-sub.Channel()
	if msg.Payload == "" {
		t.Fatalf("no payload")
	}
}
