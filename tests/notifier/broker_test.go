package notifier_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/faciam-dev/gcfm/internal/customfield/notifier"
)

func TestRedisBrokerEmit(t *testing.T) {
	s := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	broker := &notifier.RedisBroker{Client: client, Channel: "cf"}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	sub := client.Subscribe(ctx, "cf")
	defer sub.Close()
	if _, err := sub.Receive(ctx); err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	if err := broker.Emit(ctx, notifier.DiffReport{Added: 1}); err != nil {
		t.Fatalf("emit: %v", err)
	}
	msg, err := sub.ReceiveMessage(ctx)
	if err != nil {
		t.Fatalf("receive: %v", err)
	}
	if msg.Payload == "" {
		t.Fatalf("no payload")
	}
}
