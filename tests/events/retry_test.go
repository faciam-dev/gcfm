package events_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/faciam-dev/gcfm/internal/events"
)

type failSink struct{ count int }

func (f *failSink) Emit(ctx context.Context, e events.Event) error {
	f.count++
	return errors.New("fail")
}

func TestRetry(t *testing.T) {
	s := &failSink{}
	d := events.NewDispatcher(events.Config{Retry: events.RetryConfig{MaxAttempts: 2, InitialDelay: time.Millisecond}}, nil, s)
	d.Dispatch(context.Background(), events.Event{Name: "x"})
	time.Sleep(5 * time.Millisecond)
	if s.count != 2 {
		t.Fatalf("attempts=%d", s.count)
	}
}
