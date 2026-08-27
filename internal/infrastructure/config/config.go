package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	ServerAddr                string
	DatabaseURL               string
	CommonIDOrigin            string
	CommonIDAPIOrigin         string
	CommonIDClientID          string
	CommonIDRedirectURI       string
	CommonIDLogoutRedirectURI string
	CommonIDAPIKey            string
	AutoMigrate               bool
	SessionSecret             string
	SessionEncryptionSecret   string
}

func Load() Config {
	_ = godotenv.Load()

	return Config{
		ServerAddr:                normalizeAddr(getEnv("PORT", "8080")),
		DatabaseURL:               os.Getenv("DATABASE_URL"),
		CommonIDOrigin:            os.Getenv("COMMON_ID_ORIGIN"),
		CommonIDAPIOrigin:         os.Getenv("COMMON_ID_API_ORIGIN"),
		CommonIDClientID:          os.Getenv("COMMON_ID_CLIENT_ID"),
		CommonIDRedirectURI:       os.Getenv("COMMON_ID_REDIRECT_URI"),
		CommonIDLogoutRedirectURI: os.Getenv("COMMON_ID_LOGOUT_REDIRECT_URI"),
		CommonIDAPIKey:            os.Getenv("COMMON_ID_API_KEY"),
		AutoMigrate:               os.Getenv("AUTO_MIGRATE") == "true",
		SessionSecret:             os.Getenv("SESSION_SECRET"),
		SessionEncryptionSecret:   os.Getenv("SESSION_ENCRYPTION_SECRET"),
	}
}

func (c Config) Validate() error {
	if len(c.SessionSecret) < 32 {
		return fmt.Errorf("SESSION_SECRET must be at least 32 bytes")
	}
	if c.CommonIDOrigin == "" || c.CommonIDAPIOrigin == "" || c.CommonIDClientID == "" || c.CommonIDRedirectURI == "" || c.CommonIDLogoutRedirectURI == "" || c.CommonIDAPIKey == "" {
		return fmt.Errorf("COMMON_ID_ORIGIN, COMMON_ID_API_ORIGIN, COMMON_ID_CLIENT_ID, COMMON_ID_REDIRECT_URI, COMMON_ID_LOGOUT_REDIRECT_URI and COMMON_ID_API_KEY are required")
	}

	if c.SessionEncryptionSecret != "" && len(c.SessionEncryptionSecret) < 32 {
		return fmt.Errorf("SESSION_ENCRYPTION_SECRET must be at least 32 bytes when set")
	}

	return nil
}

func getEnv(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func normalizeAddr(port string) string {
	if port == "" {
		return ":8080"
	}
	if port[0] == ':' {
		return port
	}
	return ":" + port
}
