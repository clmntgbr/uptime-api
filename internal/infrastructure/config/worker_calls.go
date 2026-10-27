package config

type WorkerCallsConfig struct {
	JobSealPublicKey  string
	JobSealPrivateKey string
	Concurrency       int
	Group             string
	UserAgent         string
	AllowPrivateIPs   bool
	MinTLSVersion     string
	Environment       string
	MetricsAddr       string
	Kafka             KafkaConfig
	Storage           StorageConfig
}

// LoadWorkerCalls reads the worker-calls configuration. It deliberately has no
// database settings: worker-calls never talks to Postgres.
func LoadWorkerCalls() *WorkerCallsConfig {
	loadDotEnv()

	return &WorkerCallsConfig{
		JobSealPublicKey:  getEnv("JOB_SEAL_PUBLIC_KEY"),
		JobSealPrivateKey: getEnv("JOB_SEAL_PRIVATE_KEY"),
		Concurrency:       getEnvIntOrDefault("WORKER_CALLS_CONCURRENCY", 50),
		Group:             getEnvOrDefault("WORKER_CALLS_GROUP", "worker-calls"),
		UserAgent:         getEnvOrDefault("HTTP_USER_AGENT", "UptimeMonitor/1.0 (+https://uptime.example.com/bot)"),
		AllowPrivateIPs:   getEnvBoolOrDefault("SSRF_ALLOW_PRIVATE", false),
		MinTLSVersion:     getEnvOrDefault("HTTP_MIN_TLS_VERSION", "1.2"),
		Environment:       getEnvOrDefault("GO_ENV", "development"),
		MetricsAddr:       getEnvOrDefault("METRICS_ADDR", ":9100"),
		Kafka:             loadKafka("worker-calls"),
		Storage:           loadStorage(true),
	}
}
