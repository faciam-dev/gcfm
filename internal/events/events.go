package events

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// Default is the global dispatcher used by Emit.
var Default *Dispatcher

// Event represents a notification payload.
type Event struct {
	Name string    `json:"name"`
	Time time.Time `json:"time"`
	Data any       `json:"data"`
	ID   string    `json:"id"`
}

// Sink publishes events.
type Sink interface {
	Emit(ctx context.Context, e Event) error
}

// DLQ stores failed events.
type DLQ interface {
	Store(ctx context.Context, e Event, attempts int, lastErr string) error
}

// Dispatcher broadcasts events to multiple sinks with retries.
type Dispatcher struct {
	sinks        []Sink
	maxAttempts  int
	initialDelay time.Duration
	dlq          DLQ
}

// Config provides dispatcher settings.
type Config struct {
	Sinks struct {
		Webhook WebhookConfig `yaml:"webhook"`
		Redis   RedisConfig   `yaml:"redis"`
		Kafka   KafkaConfig   `yaml:"kafka"`
	} `yaml:"sinks"`
	Retry RetryConfig `yaml:"retry"`
}

type RetryConfig struct {
	MaxAttempts  int           `yaml:"max_attempts"`
	InitialDelay time.Duration `yaml:"initial_delay"`
}

// NewDispatcher creates a dispatcher from sinks and retry config.
func NewDispatcher(cfg Config, dlq DLQ, sinks ...Sink) *Dispatcher {
	d := &Dispatcher{maxAttempts: 3, initialDelay: time.Second}
	if cfg.Retry.MaxAttempts > 0 {
		d.maxAttempts = cfg.Retry.MaxAttempts
	}
	if cfg.Retry.InitialDelay > 0 {
		d.initialDelay = cfg.Retry.InitialDelay
	}
	d.sinks = append(d.sinks, sinks...)
	d.dlq = dlq
	return d
}

// Emit sends an event using the global dispatcher if set.
func Emit(ctx context.Context, e Event) {
	if Default != nil {
		Default.Dispatch(ctx, e)
	}
}

// Dispatch sends the event to all sinks asynchronously.
func (d *Dispatcher) Dispatch(ctx context.Context, e Event) {
	for _, s := range d.sinks {
		sink := s
		go d.retrySend(ctx, sink, e)
	}
}

func (d *Dispatcher) retrySend(ctx context.Context, s Sink, e Event) {
	delay := d.initialDelay
	var err error
	for i := 1; i <= d.maxAttempts; i++ {
		if err = s.Emit(ctx, e); err == nil {
			return
		}
		time.Sleep(delay)
		delay *= 2
	}
	if d.dlq != nil {
		_ = d.dlq.Store(ctx, e, d.maxAttempts, err.Error())
	}
}

// SQLDLQ stores failed events in the database.
type SQLDLQ struct {
	DB          *sql.DB
	Driver      string
	TablePrefix string
}

// Store inserts the failed event.
func (q *SQLDLQ) Store(ctx context.Context, e Event, attempts int, lastErr string) error {
	if q == nil || q.DB == nil {
		return nil
	}
	data, err := json.Marshal(e)
	if err != nil {
		return err
	}
	tbl := q.TablePrefix + "events_failed"
	var stmt string
	if q.Driver == "postgres" {
		stmt = fmt.Sprintf("INSERT INTO %s(name, payload, attempts, last_error) VALUES ($1, $2, $3, $4)", tbl)
	} else {
		stmt = fmt.Sprintf("INSERT INTO %s(name, payload, attempts, last_error) VALUES (?, ?, ?, ?)", tbl)
	}
	_, err = q.DB.ExecContext(ctx, stmt, e.Name, string(data), attempts, lastErr)
	return err
}
