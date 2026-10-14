package config

// StorageConfig is the S3-compatible object store used for claim-check bodies.
type StorageConfig struct {
	Endpoint  string
	Region    string
	AccessKey string
	SecretKey string
	Bucket    string
}

func loadStorage(preferInternal bool) StorageConfig {
	endpoint := getEnvOrDefault("STORAGE_ENDPOINT", "")
	if preferInternal {
		if internal := getEnvOrDefault("STORAGE_INTERNAL_ENDPOINT", ""); internal != "" {
			endpoint = internal
		}
	}
	return StorageConfig{
		Endpoint:  endpoint,
		Region:    getEnvOrDefault("STORAGE_REGION", "us-east-1"),
		AccessKey: getEnvOrDefault("STORAGE_ACCESS_KEY", ""),
		SecretKey: getEnvOrDefault("STORAGE_SECRET_KEY", ""),
		Bucket:    getEnvOrDefault("STORAGE_BUCKET", "bodies"),
	}
}
