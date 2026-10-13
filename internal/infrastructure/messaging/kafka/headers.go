package kafka

import (
	"strconv"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/otel/propagation"
)

const (
	headerAttempt       = "attempt"
	headerNotBefore     = "not_before"
	headerOriginalTopic = "original_topic"
	headerError         = "error"
	headerDLQReason     = "dlq_reason"
	headerOrigPartition = "original_partition"
	headerOrigOffset    = "original_offset"
)

func headerValue(headers []kgo.RecordHeader, key string) string {
	for i := len(headers) - 1; i >= 0; i-- {
		if headers[i].Key == key {
			return string(headers[i].Value)
		}
	}
	return ""
}

func setHeader(headers []kgo.RecordHeader, key, value string) []kgo.RecordHeader {
	for i := range headers {
		if headers[i].Key == key {
			headers[i].Value = []byte(value)
			return headers
		}
	}
	return append(headers, kgo.RecordHeader{Key: key, Value: []byte(value)})
}

func attemptOf(rec *kgo.Record) int {
	n, err := strconv.Atoi(headerValue(rec.Headers, headerAttempt))
	if err != nil || n < 1 {
		return 1
	}
	return n
}

func notBeforeOf(rec *kgo.Record) time.Time {
	t, err := time.Parse(time.RFC3339Nano, headerValue(rec.Headers, headerNotBefore))
	if err != nil {
		return time.Time{}
	}
	return t
}

func headersToMap(headers []kgo.RecordHeader) map[string]string {
	out := make(map[string]string, len(headers))
	for _, h := range headers {
		out[h.Key] = string(h.Value)
	}
	return out
}

// headerCarrier lets OpenTelemetry propagate trace context through Kafka headers.
type headerCarrier struct {
	headers *[]kgo.RecordHeader
}

var _ propagation.TextMapCarrier = headerCarrier{}

func (c headerCarrier) Get(key string) string { return headerValue(*c.headers, key) }

func (c headerCarrier) Set(key, value string) { *c.headers = setHeader(*c.headers, key, value) }

func (c headerCarrier) Keys() []string {
	keys := make([]string, 0, len(*c.headers))
	for _, h := range *c.headers {
		keys = append(keys, h.Key)
	}
	return keys
}
