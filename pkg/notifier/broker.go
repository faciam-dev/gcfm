package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/redis/go-redis/v9"
)

// Broker publishes diff reports to external systems.
// DiffReport summarizes registry changes.
type DiffReport struct {
	Added   int
	Deleted int
	Updated int
}

type Broker interface {
	Emit(ctx context.Context, diff DiffReport) error
}

type RedisBroker struct {
	Client  *redis.Client
	Channel string
}

func (b *RedisBroker) Emit(ctx context.Context, diff DiffReport) error {
	if b == nil || b.Client == nil {
		return nil
	}
	data, err := json.Marshal(diff)
	if err != nil {
		return err
	}
	return b.Client.Publish(ctx, b.Channel, data).Err()
}

type WebhookBroker struct {
	Endpoint string
	Secret   string
	Client   *http.Client
}

func (b *WebhookBroker) Emit(ctx context.Context, diff DiffReport) error {
	if b == nil || b.Endpoint == "" {
		return nil
	}
	data, err := json.Marshal(diff)
	if err != nil {
		return err
	}
	if b.Client == nil {
		b.Client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.Endpoint, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if b.Secret != "" {
		req.Header.Set("X-Webhook-Secret", b.Secret)
	}
	resp, err := b.Client.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("webhook: %s", resp.Status)
	}
	return nil
}
