package config

import "os"

type Config struct {
	Port             string
	DataFile         string
	ReaderWebhookURL string
	WebhookSecret    string
	AllowOrigins     string
}

func Load() *Config {
	return &Config{
		Port:             getEnv("PORT", "4000"),
		DataFile:         getEnv("DATA_FILE", "../exam-blog-console/.data/blog-db.json"),
		ReaderWebhookURL: getEnv("READER_WEBHOOK_URL", "http://localhost:3002/api/revalidate"),
		WebhookSecret:    getEnv("REVALIDATE_SECRET", "nextblog-secret-key"),
		AllowOrigins:     getEnv("ALLOW_ORIGINS", "http://localhost:3001, http://localhost:3002"),
	}
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
