// Package audit wires the application together: configuration from the
// environment, the database, and the services.
package audit

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
)

// Config is everything that changes between environments.
type Config struct {
	// DatabaseURL is the PostgreSQL connection string. Required.
	DatabaseURL string
	// IngestToken authorizes writers, and AdminToken the routes that manage or
	// erase data. Both are required, because they guard different people.
	IngestToken string
	AdminToken  string
	// Port is what the server listens on.
	Port string
	// RetentionMonths is how many whole months of events to keep, besides the
	// current one. Older months are dropped.
	RetentionMonths int
	// PartitionsAhead is how many future months to keep ready.
	PartitionsAhead int
	// CleanupCron says when to create and drop partitions, in standard cron
	// syntax. Empty turns the schedule off, leaving POST /admin/cleanup.
	CleanupCron string
	// LogLevel is debug, info, warn, or error.
	LogLevel slog.Level
}

// LoadConfig reads the environment, and reports every problem at once.
func LoadConfig() (Config, error) {
	c := Config{
		DatabaseURL: os.Getenv("DATABASE_URL"),
		IngestToken: os.Getenv("INGEST_TOKEN"),
		AdminToken:  os.Getenv("ADMIN_TOKEN"),
		Port:        os.Getenv("PORT"),
		CleanupCron: os.Getenv("CLEANUP_CRON"),
	}

	var problems []error

	for name, value := range map[string]string{"DATABASE_URL": c.DatabaseURL, "INGEST_TOKEN": c.IngestToken, "ADMIN_TOKEN": c.AdminToken} {
		if value == "" {
			problems = append(problems, fmt.Errorf("%s is not set", name))
		}
	}

	if c.Port == "" {
		c.Port = "8080"
	}

	number := func(name string, fallback int, into *int) {
		*into = fallback

		if text := os.Getenv(name); text != "" {
			n, err := strconv.Atoi(text)
			if err != nil || n < 0 {
				problems = append(problems, fmt.Errorf("%s must be a number, zero or more", name))
			}

			*into = n
		}
	}
	number("RETENTION_MONTHS", 12, &c.RetentionMonths)
	number("PARTITIONS_AHEAD", 3, &c.PartitionsAhead)

	if text := os.Getenv("LOG_LEVEL"); text != "" {
		if err := c.LogLevel.UnmarshalText([]byte(text)); err != nil {
			problems = append(problems, errors.New("LOG_LEVEL must be debug, info, warn, or error"))
		}
	}

	return c, errors.Join(problems...)
}
