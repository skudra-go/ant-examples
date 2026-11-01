// Package ledger wires the application together: configuration from the
// environment, the database, and the services.
package ledger

import (
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"os"
)

// Config is everything that changes between environments.
type Config struct {
	// DatabaseURL is the PostgreSQL connection string. Required.
	DatabaseURL string
	// APIToken guards every route. Required.
	APIToken string
	// EncryptionKey protects IBANs: 32 bytes, in base64. Required.
	EncryptionKey []byte
	// Port is what the server listens on.
	Port string
	// LogLevel is debug, info, warn, or error.
	LogLevel slog.Level
}

// LoadConfig reads the environment, and reports every problem at once.
func LoadConfig() (Config, error) {
	c := Config{
		DatabaseURL: os.Getenv("DATABASE_URL"),
		APIToken:    os.Getenv("API_TOKEN"),
		Port:        os.Getenv("PORT"),
	}

	var problems []error

	if c.DatabaseURL == "" {
		problems = append(problems, errors.New("DATABASE_URL is not set"))
	}

	if c.APIToken == "" {
		problems = append(problems, errors.New("API_TOKEN is not set"))
	}

	key, err := base64.StdEncoding.DecodeString(os.Getenv("ENCRYPTION_KEY"))
	if err != nil || len(key) != 32 {
		problems = append(problems, fmt.Errorf("ENCRYPTION_KEY must be 32 bytes in base64"))
	}

	c.EncryptionKey = key

	if c.Port == "" {
		c.Port = "8080"
	}

	if text := os.Getenv("LOG_LEVEL"); text != "" {
		if err := c.LogLevel.UnmarshalText([]byte(text)); err != nil {
			problems = append(problems, errors.New("LOG_LEVEL must be debug, info, warn, or error"))
		}
	}

	return c, errors.Join(problems...)
}
