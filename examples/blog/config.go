// Package blog wires the application together: configuration from the environment,
// the database, and the services. Both commands in cmd build on it.
package blog

import (
	"errors"
	"log/slog"
	"os"
	"strconv"
)

// Config is everything that changes between environments. Nothing else reads the
// environment, so this file is the full list of what a deployment must set.
type Config struct {
	// DatabaseURL is the PostgreSQL connection string. Required.
	DatabaseURL string
	// AdminToken guards the endpoints that write. Required.
	AdminToken string
	// Port is what the server listens on.
	Port string
	// AutoMigrate applies migrations when the server starts. The compose file
	// leaves it off and uses the migrate command, which keeps a schema change
	// separate from a code rollout. It suits small deployments.
	AutoMigrate bool
	// LogLevel is debug, info, warn, or error.
	LogLevel slog.Level
}

// LoadConfig reads the environment. It reports every problem at once, so a broken
// deployment takes one attempt to fix.
func LoadConfig() (Config, error) {
	c := Config{
		DatabaseURL: os.Getenv("DATABASE_URL"),
		AdminToken:  os.Getenv("ADMIN_TOKEN"),
		Port:        os.Getenv("PORT"),
	}

	var problems []error

	if c.DatabaseURL == "" {
		problems = append(problems, errors.New("DATABASE_URL is not set"))
	}

	if c.AdminToken == "" {
		problems = append(problems, errors.New("ADMIN_TOKEN is not set"))
	}

	if c.Port == "" {
		c.Port = "8080"
	}

	if text := os.Getenv("AUTO_MIGRATE"); text != "" {
		on, err := strconv.ParseBool(text)
		if err != nil {
			problems = append(problems, errors.New("AUTO_MIGRATE must be true or false"))
		}

		c.AutoMigrate = on
	}

	if text := os.Getenv("LOG_LEVEL"); text != "" {
		if err := c.LogLevel.UnmarshalText([]byte(text)); err != nil {
			problems = append(problems, errors.New("LOG_LEVEL must be debug, info, warn, or error"))
		}
	}

	return c, errors.Join(problems...)
}
