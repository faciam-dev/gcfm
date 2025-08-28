package events

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// WebhookConfig configures WebhookSink.
type WebhookConfig struct {
	Enabled  bool          `yaml:"enabled"`
	Endpoint string        `yaml:"endpoint"`
	Secret   string        `yaml:"secret"`
	Timeout  time.Duration `yaml:"timeout"`
}

// WebhookSink posts events to an HTTP endpoint.
type WebhookSink struct {
	Endpoint string
	Secret   string
	Client   *http.Client
}

// NewWebhookSink creates a WebhookSink from config.
func NewWebhookSink(c WebhookConfig) *WebhookSink {
	if !c.Enabled || c.Endpoint == "" {
		return nil
	}
	cli := &http.Client{Timeout: c.Timeout}
	if c.Timeout == 0 {
		cli.Timeout = 5 * time.Second
	}
	return &WebhookSink{Endpoint: c.Endpoint, Secret: c.Secret, Client: cli}
}

func (s *WebhookSink) Emit(ctx context.Context, e Event) error {
	if s == nil {
		return nil
	}
	data, err := json.Marshal(e)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.Endpoint, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if s.Secret != "" {
		h := hmac.New(sha256.New, []byte(s.Secret))
		h.Write(data)
		sig := hex.EncodeToString(h.Sum(nil))
		req.Header.Set("X-CF-Signature", "sha256="+sig)
	}
	resp, err := s.Client.Do(req)
	if err != nil {
		return err
	}
	if err := resp.Body.Close(); err != nil {
		return err
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("webhook: %s", resp.Status)
	}
	return nil
}
