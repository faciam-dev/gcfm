package events

import (
	"context"
	"encoding/json"

	"github.com/IBM/sarama"
)

// KafkaConfig configures KafkaSink.
type KafkaConfig struct {
	Enabled bool     `yaml:"enabled"`
	Brokers []string `yaml:"brokers"`
	Topic   string   `yaml:"topic"`
}

// KafkaSink publishes events to Kafka.
type KafkaSink struct {
	Producer sarama.AsyncProducer
	Topic    string
}

// NewKafkaSink creates a KafkaSink from config.
func NewKafkaSink(c KafkaConfig) (*KafkaSink, error) {
	if !c.Enabled || len(c.Brokers) == 0 {
		return nil, nil
	}
	cfg := sarama.NewConfig()
	prod, err := sarama.NewAsyncProducer(c.Brokers, cfg)
	if err != nil {
		return nil, err
	}
	return &KafkaSink{Producer: prod, Topic: c.Topic}, nil
}

func (s *KafkaSink) Emit(ctx context.Context, e Event) error {
	if s == nil || s.Producer == nil {
		return nil
	}
	data, err := json.Marshal(e)
	if err != nil {
		return err
	}
	msg := &sarama.ProducerMessage{Topic: s.Topic, Value: sarama.ByteEncoder(data)}
	select {
	case s.Producer.Input() <- msg:
		return nil
	case err := <-s.Producer.Errors():
		return err.Err
	case <-ctx.Done():
		return ctx.Err()
	}
}
