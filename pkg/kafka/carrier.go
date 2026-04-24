package kafka

import "github.com/segmentio/kafka-go"

// headerCarrier adapts a slice of kafka headers to the otel propagation
// TextMapCarrier interface so trace context can ride along with each message.
type headerCarrier []kafka.Header

func (c *headerCarrier) Get(key string) string {
	for _, h := range *c {
		if h.Key == key {
			return string(h.Value)
		}
	}
	return ""
}

func (c *headerCarrier) Set(key, value string) {
	for i, h := range *c {
		if h.Key == key {
			(*c)[i].Value = []byte(value)
			return
		}
	}
	*c = append(*c, kafka.Header{Key: key, Value: []byte(value)})
}

func (c *headerCarrier) Keys() []string {
	keys := make([]string, 0, len(*c))
	for _, h := range *c {
		keys = append(keys, h.Key)
	}
	return keys
}
