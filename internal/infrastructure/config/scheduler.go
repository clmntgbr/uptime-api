package config

import "time"

type SchedulerConfig struct {
	DatabaseURL          string
	SecretsEncryptionKey string
	JobSealPublicKey     string
	RetentionDays        int
	DispatchBatchSize    int
	LeaseMargin          time.Duration
	MaxJitter            time.Duration
	HeartbeatURL         string
	MetricsAddr          string
	Kafka                KafkaConfig
}

// LoadScheduler reads the scheduler configuration. The scheduler only holds the
// public half of the job sealing key: it can seal secrets but never open them.
func LoadScheduler() *SchedulerConfig {
	loadDotEnv()

	return &SchedulerConfig{
		DatabaseURL:          getEnv("DATABASE_URL"),
		SecretsEncryptionKey: getEnv("SECRETS_ENCRYPTION_KEY"),
		JobSealPublicKey:     getEnv("JOB_SEAL_PUBLIC_KEY"),
		RetentionDays:        getEnvIntOrDefault("RESULTS_RETENTION_DAYS", 30),
		DispatchBatchSize:    getEnvIntOrDefault("SCHEDULER_BATCH_SIZE", 500),
		LeaseMargin:          getEnvDuration("SCHEDULER_LEASE_MARGIN", time.Minute),
		MaxJitter:            getEnvDuration("SCHEDULER_MAX_JITTER", 5*time.Second),
		HeartbeatURL:         getEnvOrDefault("HEARTBEAT_URL", ""),
		MetricsAddr:          getEnvOrDefault("METRICS_ADDR", ":9100"),
		Kafka:                loadKafka("scheduler"),
	}
}
