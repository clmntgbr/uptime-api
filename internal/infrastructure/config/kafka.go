package config

import "strings"

type KafkaConfig struct {
	Brokers       []string
	ClientID      string
	InstanceID    string
	SASLMechanism string
	Username      string
	Password      string
	TLS           bool
	TLSCAFile     string
}

func loadKafka(defaultClientID string) KafkaConfig {
	clientID := getEnvOrDefault("KAFKA_CLIENT_ID", defaultClientID)
	return KafkaConfig{
		Brokers:       splitNonEmpty(getEnvOrDefault("KAFKA_BROKERS", "kafka:9092")),
		ClientID:      clientID,
		InstanceID:    getEnvOrDefault("KAFKA_INSTANCE_ID", clientID+"-"+hostname()),
		SASLMechanism: getEnvOrDefault("KAFKA_SASL_MECHANISM", ""),
		Username:      getEnvOrDefault("KAFKA_USERNAME", ""),
		Password:      getEnvOrDefault("KAFKA_PASSWORD", ""),
		TLS:           getEnvBool("KAFKA_TLS"),
		TLSCAFile:     getEnvOrDefault("KAFKA_TLS_CA_FILE", ""),
	}
}

func splitNonEmpty(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
