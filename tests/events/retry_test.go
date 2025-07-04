package events_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/faciam-dev/gcfm/internal/events"
)

type failSink struct {
	count int
	wg    *sync.WaitGroup
}

func (f *failSink) Emit(ctx context.Context, e events.Event) error {
	f.count++
	if f.wg != nil {
		f.wg.Done()
	}
	return errors.New("fail")
}

func TestRetry(t *testing.T) {
	wg := &sync.WaitGroup{}
	wg.Add(2)
	s := &failSink{wg: wg}
	d := events.NewDispatcher(events.Config{Retry: events.RetryConfig{MaxAttempts: 2, InitialDelay: time.Millisecond}}, nil, s)
	d.Dispatch(context.Background(), events.Event{Name: "x"})
	wg.Wait()
	if s.count != 2 {
		t.Fatalf("attempts=%d", s.count)
	}
}
