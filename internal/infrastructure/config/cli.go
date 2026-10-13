package config

// CLIConfig only holds what the administration commands need, so they run
// without the HTTP and broker settings.
type CLIConfig struct {
	DatabaseURL          string
	SecretsEncryptionKey string
}

func LoadCLI() *CLIConfig {
	loadDotEnv()
	return &CLIConfig{
		DatabaseURL:          getEnv("DATABASE_URL"),
		SecretsEncryptionKey: getEnvOrDefault("SECRETS_ENCRYPTION_KEY", ""),
	}
}
