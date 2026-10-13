package kafka

import (
	"context"
	"fmt"

	"go-api/internal/infrastructure/config"

	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/otel"
)

// Producer is an idempotent producer (acks=all). Records are keyed so that every
// message of a workflow lands on the same partition and stays ordered.
type Producer struct {
	client *kgo.Client
}

func NewProducer(cfg config.KafkaConfig) (*Producer, error) {
	cl, err := newClient(cfg,
		kgo.RequiredAcks(kgo.AllISRAcks()),
		kgo.ProducerBatchMaxBytes(MaxMessageBytes-64*1024),
		kgo.RecordPartitioner(kgo.StickyKeyPartitioner(nil)),
	)
	if err != nil {
		return nil, err
	}
	return &Producer{client: cl}, nil
}

func (p *Producer) Produce(ctx context.Context, topic string, key, value []byte, headers map[string]string) error {
	rec := &kgo.Record{Topic: topic, Key: key, Value: value}
	for k, v := range headers {
		rec.Headers = append(rec.Headers, kgo.RecordHeader{Key: k, Value: []byte(v)})
	}
	otel.GetTextMapPropagator().Inject(ctx, headerCarrier{headers: &rec.Headers})
	return p.produceRecord(ctx, rec)
}

func (p *Producer) produceRecord(ctx context.Context, rec *kgo.Record) error {
	if err := p.client.ProduceSync(ctx, rec).FirstErr(); err != nil {
		return fmt.Errorf("kafka: produce to %s: %w", rec.Topic, err)
	}
	return nil
}

func (p *Producer) Close() {
	p.client.Close()
}
