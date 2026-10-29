package config

type WorkerNotifyConfig struct {
	Concurrency int
	Group       string
	Environment string
	MetricsAddr string
	Kafka       KafkaConfig
}

// LoadWorkerNotify reads worker-notify configuration (Kafka only, no Postgres).
func LoadWorkerNotify() *WorkerNotifyConfig {
	loadDotEnv()
	return &WorkerNotifyConfig{
		Concurrency: getEnvIntOrDefault("WORKER_NOTIFY_CONCURRENCY", 20),
		Group:       getEnvOrDefault("WORKER_NOTIFY_GROUP", "worker-notify"),
		Environment: getEnvOrDefault("GO_ENV", "development"),
		MetricsAddr: getEnvOrDefault("METRICS_ADDR", ":9100"),
		Kafka:       loadKafka("worker-notify"),
	}
}
