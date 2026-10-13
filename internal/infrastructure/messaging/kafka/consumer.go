package kafka

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"time"

	"go-api/internal/application/messaging"
	"go-api/internal/infrastructure/config"

	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sync/errgroup"
)

// Message is the transport-neutral view of a record handed to handlers.
type Message struct {
	Topic   string
	Key     []byte
	Value   []byte
	Headers map[string]string
	Attempt int
}

// Handler processes one message. Errors wrapped with messaging.NonRetryable go
// straight to the DLQ; any other error is retried.
type Handler func(ctx context.Context, msg Message) error

type ConsumerOptions struct {
	Group       string
	Topic       string
	Concurrency int
	// OrderByKey serializes records sharing a key (same workflow) while keeping
	// different keys parallel.
	OrderByKey bool
	// InPlaceRetries is the number of immediate retries before the retry topic.
	InPlaceRetries int
	// MaxAttempts is the number of deliveries (main + retry topic) before the DLQ.
	MaxAttempts int
	// RetryDelay is multiplied by the attempt number to compute not_before.
	RetryDelay time.Duration
}

func (o *ConsumerOptions) normalize() {
	if o.Concurrency <= 0 {
		o.Concurrency = 1
	}
	if o.InPlaceRetries < 0 {
		o.InPlaceRetries = 0
	}
	if o.MaxAttempts <= 0 {
		o.MaxAttempts = 5
	}
	if o.RetryDelay <= 0 {
		o.RetryDelay = 30 * time.Second
	}
}

// DefaultConsumerOptions follows the platform rules: 3 retries in place,
// then the retry topic, DLQ after 5 attempts.
func DefaultConsumerOptions(group, topic string, concurrency int, orderByKey bool) ConsumerOptions {
	return ConsumerOptions{
		Group:          group,
		Topic:          topic,
		Concurrency:    concurrency,
		OrderByKey:     orderByKey,
		InPlaceRetries: 3,
		MaxAttempts:    5,
		RetryDelay:     30 * time.Second,
	}
}

// Consumer consumes a topic and its `.retry` companion. Offsets are committed
// manually, up to the oldest record still being handled (at-least-once).
type Consumer struct {
	cfg      config.KafkaConfig
	opts     ConsumerOptions
	handler  Handler
	producer *Producer
	logger   *slog.Logger
	tracer   trace.Tracer
}

func NewConsumer(cfg config.KafkaConfig, opts ConsumerOptions, handler Handler, producer *Producer) *Consumer {
	opts.normalize()
	return &Consumer{
		cfg:      cfg,
		opts:     opts,
		handler:  handler,
		producer: producer,
		logger:   slog.Default().With("component", "kafka-consumer", "group", opts.Group, "topic", opts.Topic),
		tracer:   otel.Tracer("go-api/kafka"),
	}
}

// Run blocks until ctx is cancelled or a loop fails.
func (c *Consumer) Run(ctx context.Context) error {
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		return c.newLoop(c.opts.Topic, c.opts.Group, c.cfg.InstanceID, c.opts.Concurrency, c.opts.OrderByKey, false).run(ctx)
	})
	g.Go(func() error {
		return c.newLoop(RetryTopic(c.opts.Topic), c.opts.Group+".retry", c.cfg.InstanceID+"-retry", 1, true, true).run(ctx)
	})
	return g.Wait()
}

type loop struct {
	c           *Consumer
	topic       string
	group       string
	instanceID  string
	concurrency int
	ordered     bool
	// delayed loops wait for each record's not_before before handling it. They
	// run with one lane, so waiting effectively pauses the partition.
	delayed bool
}

func (c *Consumer) newLoop(topic, group, instanceID string, concurrency int, ordered, delayed bool) *loop {
	return &loop{
		c:           c,
		topic:       topic,
		group:       group,
		instanceID:  instanceID,
		concurrency: concurrency,
		ordered:     ordered,
		delayed:     delayed,
	}
}

func (l *loop) run(ctx context.Context) error {
	logger := l.c.logger.With("loop_topic", l.topic)
	tracker := newOffsetTracker()

	cl, err := newClient(l.c.cfg,
		kgo.ConsumerGroup(l.group),
		kgo.ConsumeTopics(l.topic),
		kgo.DisableAutoCommit(),
		kgo.Balancers(kgo.CooperativeStickyBalancer()),
		kgo.InstanceID(l.instanceID),
		kgo.SessionTimeout(45*time.Second),
		// Equivalent of max.poll.interval.ms: above the 5 min workflow timeout.
		kgo.RebalanceTimeout(10*time.Minute),
		kgo.OnPartitionsRevoked(func(ctx context.Context, cl *kgo.Client, revoked map[string][]int32) {
			commitSync(ctx, cl, tracker, revoked, logger)
			tracker.drop(revoked)
		}),
		kgo.OnPartitionsLost(func(_ context.Context, _ *kgo.Client, lost map[string][]int32) {
			tracker.drop(lost)
		}),
	)
	if err != nil {
		return err
	}
	defer cl.Close()

	logger.Info("kafka consumer started", "concurrency", l.concurrency, "ordered", l.ordered)

	commitCtx, stopCommits := context.WithCancel(context.Background())
	commitsDone := make(chan struct{})
	go func() {
		defer close(commitsDone)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-commitCtx.Done():
				return
			case <-ticker.C:
				commitAsync(commitCtx, cl, tracker, logger)
			}
		}
	}()

	d := newDispatcher(l.concurrency, l.ordered)

	for {
		fetches := cl.PollRecords(ctx, 256)
		if fetches.IsClientClosed() || ctx.Err() != nil {
			break
		}
		fetches.EachError(func(topic string, partition int32, err error) {
			if !errors.Is(err, context.Canceled) {
				logger.Error("kafka fetch error", "fetch_topic", topic, "partition", partition, "error", err)
			}
		})
		fetches.EachPartition(func(p kgo.FetchTopicPartition) {
			if len(p.Records) == 0 {
				return
			}
			last := p.Records[len(p.Records)-1]
			consumerLag.WithLabelValues(l.group, p.Topic, strconv.Itoa(int(p.Partition))).
				Set(float64(max(p.HighWatermark-last.Offset-1, 0)))

			st := tracker.state(p.Topic, p.Partition)
			for _, rec := range p.Records {
				st.dispatched(rec.Offset)
				ok := d.submit(ctx, laneKey(rec), func() {
					if l.process(ctx, rec) {
						st.done(rec.Offset)
					}
				})
				if !ok {
					return
				}
			}
		})
	}

	d.close()
	stopCommits()
	<-commitsDone

	finalCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	commitSync(finalCtx, cl, tracker, nil, logger)
	logger.Info("kafka consumer stopped")
	return nil
}

func laneKey(rec *kgo.Record) []byte {
	if len(rec.Key) > 0 {
		return rec.Key
	}
	return []byte(strconv.Itoa(int(rec.Partition)))
}

// process handles one record and returns true when its offset may be committed:
// handled successfully, or safely forwarded to the retry topic or the DLQ.
func (l *loop) process(ctx context.Context, rec *kgo.Record) bool {
	if l.delayed {
		if wait := time.Until(notBeforeOf(rec)); wait > 0 {
			timer := time.NewTimer(wait)
			select {
			case <-timer.C:
			case <-ctx.Done():
				timer.Stop()
				return false
			}
		}
	}

	ctx = otel.GetTextMapPropagator().Extract(ctx, headerCarrier{headers: &rec.Headers})
	ctx, span := l.c.tracer.Start(ctx, "consume "+l.c.opts.Topic,
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(
			attribute.String("messaging.system", "kafka"),
			attribute.String("messaging.destination.name", rec.Topic),
			attribute.String("messaging.consumer.group.name", l.group),
			attribute.Int64("messaging.kafka.offset", rec.Offset),
		),
	)
	defer span.End()

	msg := Message{
		Topic:   rec.Topic,
		Key:     rec.Key,
		Value:   rec.Value,
		Headers: headersToMap(rec.Headers),
		Attempt: attemptOf(rec),
	}

	start := time.Now()
	err := l.c.handleWithRetries(ctx, msg)
	handlerDuration.WithLabelValues(l.group, l.c.opts.Topic).Observe(time.Since(start).Seconds())
	if err == nil {
		messagesProcessed.WithLabelValues(l.group, l.c.opts.Topic, "ok").Inc()
		return true
	}
	if ctx.Err() != nil {
		// Shutting down: leave the record uncommitted so it is redelivered.
		return false
	}

	span.RecordError(err)
	span.SetStatus(codes.Error, "handler failed")

	var nonRetryable *messaging.NonRetryableError
	switch {
	case errors.As(err, &nonRetryable):
		return l.c.toDLQ(ctx, rec, msg.Attempt, "non_retryable", err)
	case msg.Attempt >= l.c.opts.MaxAttempts:
		return l.c.toDLQ(ctx, rec, msg.Attempt, "max_attempts", err)
	default:
		return l.c.toRetry(ctx, rec, msg.Attempt, err)
	}
}

func (c *Consumer) handleWithRetries(ctx context.Context, msg Message) error {
	backoff := []time.Duration{200 * time.Millisecond, time.Second, 3 * time.Second}
	for i := 0; ; i++ {
		err := c.handler(ctx, msg)
		if err == nil {
			return nil
		}
		var nonRetryable *messaging.NonRetryableError
		if errors.As(err, &nonRetryable) || i >= c.opts.InPlaceRetries || ctx.Err() != nil {
			return err
		}
		c.logger.Warn("handler failed, retrying in place", "attempt", msg.Attempt, "retry", i+1, "error", err)
		if !sleep(ctx, backoff[min(i, len(backoff)-1)]) {
			return ctx.Err()
		}
	}
}

func (c *Consumer) toRetry(ctx context.Context, rec *kgo.Record, attempt int, cause error) bool {
	notBefore := time.Now().UTC().Add(c.opts.RetryDelay * time.Duration(attempt))
	out := cloneRecord(rec, RetryTopic(c.opts.Topic))
	out.Headers = setHeader(out.Headers, headerAttempt, strconv.Itoa(attempt+1))
	out.Headers = setHeader(out.Headers, headerNotBefore, notBefore.Format(time.RFC3339Nano))
	out.Headers = setHeader(out.Headers, headerOriginalTopic, c.opts.Topic)
	out.Headers = setHeader(out.Headers, headerError, truncate(cause.Error(), 1024))

	c.logger.Warn("message sent to retry topic", "attempt", attempt, "not_before", notBefore, "error", cause)
	messagesProcessed.WithLabelValues(c.opts.Group, c.opts.Topic, "retry").Inc()
	return c.forward(ctx, out)
}

func (c *Consumer) toDLQ(ctx context.Context, rec *kgo.Record, attempt int, reason string, cause error) bool {
	out := cloneRecord(rec, DLQTopic(c.opts.Topic))
	out.Headers = setHeader(out.Headers, headerAttempt, strconv.Itoa(attempt))
	out.Headers = setHeader(out.Headers, headerDLQReason, reason)
	out.Headers = setHeader(out.Headers, headerOriginalTopic, c.opts.Topic)
	out.Headers = setHeader(out.Headers, headerOrigPartition, strconv.Itoa(int(rec.Partition)))
	out.Headers = setHeader(out.Headers, headerOrigOffset, strconv.FormatInt(rec.Offset, 10))
	out.Headers = setHeader(out.Headers, headerError, truncate(cause.Error(), 1024))

	// Internal alert: a DLQ message always needs a human.
	c.logger.Error("ALERT message moved to DLQ",
		"dlq_topic", out.Topic, "reason", reason, "attempt", attempt,
		"partition", rec.Partition, "offset", rec.Offset, "error", cause)
	dlqMessages.WithLabelValues(out.Topic, reason).Inc()
	messagesProcessed.WithLabelValues(c.opts.Group, c.opts.Topic, "dlq").Inc()
	return c.forward(ctx, out)
}

// forward publishes until it succeeds; the source offset is only committed afterwards.
func (c *Consumer) forward(ctx context.Context, rec *kgo.Record) bool {
	for attempt := 0; ; attempt++ {
		err := c.producer.produceRecord(ctx, rec)
		if err == nil {
			return true
		}
		c.logger.Error("failed to forward message", "target_topic", rec.Topic, "error", err)
		if !sleep(ctx, time.Duration(min(attempt+1, 10))*time.Second) {
			return false
		}
	}
}

func cloneRecord(rec *kgo.Record, topic string) *kgo.Record {
	headers := make([]kgo.RecordHeader, len(rec.Headers))
	copy(headers, rec.Headers)
	return &kgo.Record{Topic: topic, Key: rec.Key, Value: rec.Value, Headers: headers}
}

func commitAsync(ctx context.Context, cl *kgo.Client, tracker *offsetTracker, logger *slog.Logger) {
	offsets := tracker.uncommitted(nil)
	if len(offsets) == 0 {
		return
	}
	cl.CommitOffsets(ctx, offsets, func(_ *kgo.Client, _ *kmsg.OffsetCommitRequest, resp *kmsg.OffsetCommitResponse, err error) {
		onCommitted(tracker, offsets, resp, err, logger)
	})
}

func commitSync(ctx context.Context, cl *kgo.Client, tracker *offsetTracker, only map[string][]int32, logger *slog.Logger) {
	offsets := tracker.uncommitted(only)
	if len(offsets) == 0 {
		return
	}
	cl.CommitOffsetsSync(ctx, offsets, func(_ *kgo.Client, _ *kmsg.OffsetCommitRequest, resp *kmsg.OffsetCommitResponse, err error) {
		onCommitted(tracker, offsets, resp, err, logger)
	})
}

func onCommitted(
	tracker *offsetTracker,
	offsets map[string]map[int32]kgo.EpochOffset,
	resp *kmsg.OffsetCommitResponse,
	err error,
	logger *slog.Logger,
) {
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			logger.Warn("offset commit failed", "error", err)
		}
		return
	}
	committed := make(map[string]map[int32]kgo.EpochOffset)
	for _, t := range resp.Topics {
		for _, p := range t.Partitions {
			if p.ErrorCode != 0 {
				logger.Warn("offset commit rejected", "commit_topic", t.Topic, "partition", p.Partition, "code", p.ErrorCode)
				continue
			}
			if eo, ok := offsets[t.Topic][p.Partition]; ok {
				if committed[t.Topic] == nil {
					committed[t.Topic] = make(map[int32]kgo.EpochOffset)
				}
				committed[t.Topic][p.Partition] = eo
			}
		}
	}
	tracker.markCommitted(committed)
}

func sleep(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
