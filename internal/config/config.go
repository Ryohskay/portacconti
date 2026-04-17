package config

import (
	"fmt"
	"os"
)

type Config struct {
	Port              string
	DatabaseURL       string
	JWTSecret         string
	EncryptionKey     string // 32-byte hex string
	StripeSecretKey   string
	StripeWebhookSecret string
	SMTPHost          string
	SMTPPort          string
	SMTPUser          string
	SMTPPassword      string
	EmailFrom         string
	BaseURL           string
}

func Load() (*Config, error) {
	cfg := &Config{
		Port:              getEnv("PORT", "8080"),
		DatabaseURL:       os.Getenv("DATABASE_URL"),
		JWTSecret:         os.Getenv("JWT_SECRET"),
		EncryptionKey:     os.Getenv("ENCRYPTION_KEY"),
		StripeSecretKey:   os.Getenv("STRIPE_SECRET_KEY"),
		StripeWebhookSecret: os.Getenv("STRIPE_WEBHOOK_SECRET"),
		SMTPHost:          getEnv("SMTP_HOST", "localhost"),
		SMTPPort:          getEnv("SMTP_PORT", "587"),
		SMTPUser:          os.Getenv("SMTP_USER"),
		SMTPPassword:      os.Getenv("SMTP_PASSWORD"),
		EmailFrom:         getEnv("EMAIL_FROM", "no-reply@clinic.example.com"),
		BaseURL:           getEnv("BASE_URL", "http://localhost:3000"),
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}
	if cfg.EncryptionKey == "" {
		return nil, fmt.Errorf("ENCRYPTION_KEY is required")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
