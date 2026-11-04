package config

type WorkerNotifyConfig struct {
	Concurrency int
	Group       string
	Environment string
	MetricsAddr string
	DatabaseURL string
	SMTP        SMTPConfig
	Kafka       KafkaConfig
}

type SMTPConfig struct {
	Host     string
	Port     int
	From     string
	User     string
	Password string
}

// LoadWorkerNotify reads worker-notify configuration (Kafka + optional SMTP/DB).
func LoadWorkerNotify() *WorkerNotifyConfig {
	loadDotEnv()
	return &WorkerNotifyConfig{
		Concurrency: getEnvIntOrDefault("WORKER_NOTIFY_CONCURRENCY", 20),
		Group:       getEnvOrDefault("WORKER_NOTIFY_GROUP", "worker-notify"),
		Environment: getEnvOrDefault("GO_ENV", "development"),
		MetricsAddr: getEnvOrDefault("METRICS_ADDR", ":9100"),
		DatabaseURL: getEnvOrDefault("DATABASE_URL", ""),
		SMTP: SMTPConfig{
			Host:     getEnvOrDefault("SMTP_HOST", ""),
			Port:     getEnvIntOrDefault("SMTP_PORT", 1025),
			From:     getEnvOrDefault("SMTP_FROM", "noreply@uptime.local"),
			User:     getEnvOrDefault("SMTP_USER", ""),
			Password: getEnvOrDefault("SMTP_PASSWORD", ""),
		},
		Kafka: loadKafka("worker-notify"),
	}
}
