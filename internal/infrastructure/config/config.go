package config

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL           string
	ClerkWebhookSecret    string
	Port                  string
	Environment           string
	ClerkSecretKey        string
	ClerkFrontendAPI      string
	CORSAllowedOrigins    []string
	CORSAllowCredentials  bool
	CORSAllowMethods      []string
	CORSAllowHeaders      []string
	CORSMaxAge            int
	RateLimitMax          int
	OutboxPollInterval    time.Duration
	WorkerConcurrency     int
	WorkerResultsGroup    string
	WorkerEventsGroup     string
	CentrifugoURL         string
	CentrifugoAPIKey      string
	CentrifugoTokenSecret string
	CentrifugoPublicWSURL string
	SecretsEncryptionKey  string
	JobSealPublicKey      string
	MetricsAddr           string
	Kafka                 KafkaConfig
}

// Load reads the configuration shared by the api, worker and cli binaries.
func Load() *Config {
	loadDotEnv()

	return &Config{
		DatabaseURL:           getEnv("DATABASE_URL"),
		ClerkWebhookSecret:    getEnv("CLERK_WEBHOOK_SECRET"),
		Port:                  getEnv("PORT"),
		Environment:           getEnv("GO_ENV"),
		ClerkSecretKey:        getEnv("CLERK_SECRET_KEY"),
		ClerkFrontendAPI:      getEnv("CLERK_FRONTEND_API"),
		CORSAllowedOrigins:    strings.Split(getEnv("CORS_ALLOWED_ORIGINS"), ","),
		CORSAllowCredentials:  getEnvBool("CORS_ALLOW_CREDENTIALS"),
		CORSAllowMethods:      strings.Split(getEnv("CORS_ALLOW_METHODS"), ","),
		CORSAllowHeaders:      strings.Split(getEnv("CORS_ALLOW_HEADERS"), ","),
		CORSMaxAge:            getEnvInt("CORS_MAX_AGE"),
		RateLimitMax:          getEnvInt("RATE_LIMIT_MAX"),
		OutboxPollInterval:    getEnvDuration("OUTBOX_POLL_INTERVAL", time.Second),
		WorkerConcurrency:     getEnvIntOrDefault("WORKER_CONCURRENCY", 8),
		WorkerResultsGroup:    getEnvOrDefault("WORKER_RESULTS_GROUP", "worker"),
		WorkerEventsGroup:     getEnvOrDefault("WORKER_EVENTS_GROUP", "worker-events"),
		CentrifugoURL:         getEnv("CENTRIFUGO_URL"),
		CentrifugoAPIKey:      getEnv("CENTRIFUGO_API_KEY"),
		CentrifugoTokenSecret: getEnv("CENTRIFUGO_TOKEN_SECRET"),
		CentrifugoPublicWSURL: getEnvOrDefault("CENTRIFUGO_PUBLIC_WS_URL", ""),
		SecretsEncryptionKey:  getEnvOrDefault("SECRETS_ENCRYPTION_KEY", ""),
		JobSealPublicKey:      getEnvOrDefault("JOB_SEAL_PUBLIC_KEY", ""),
		MetricsAddr:           getEnvOrDefault("METRICS_ADDR", ":9100"),
		Kafka:                 loadKafka("worker"),
	}
}

func loadDotEnv() {
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file found, using environment variables")
	}
}

// IsProduction reports whether env is a production deployment (GO_ENV).
func IsProduction(env string) bool {
	switch strings.ToLower(strings.TrimSpace(env)) {
	case "production", "prod":
		return true
	default:
		return false
	}
}

func getEnv(key string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	log.Panicf("required environment variable %s is not set", key)
	return ""
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvBool(key string) bool {
	value := os.Getenv(key)
	if value == "" {
		return false
	}

	return value == "true"
}

func getEnvBoolOrDefault(key string, defaultValue bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value == "true"
}

func getEnvInt(key string) int {
	value := os.Getenv(key)
	if value == "" {
		log.Panicf("required environment variable %s is not set", key)
		return 0
	}

	parsedValue, err := strconv.Atoi(value)
	if err != nil {
		log.Panicf("invalid integer for %s: %q", key, value)
		return 0
	}

	return parsedValue
}

func getEnvIntOrDefault(key string, defaultValue int) int {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	parsedValue, err := strconv.Atoi(value)
	if err != nil {
		log.Panicf("invalid integer for %s: %q", key, value)
		return 0
	}
	return parsedValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		log.Panicf("invalid duration for %s: %q", key, value)
		return 0
	}
	return parsed
}

func hostname() string {
	name, err := os.Hostname()
	if err != nil || name == "" {
		return "local"
	}
	return name
}
