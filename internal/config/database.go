package config

import (
	"errors"
	"net/url"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

func DatabaseURL(lookup func(string) (string, bool)) (string, error) {
	value, _ := lookup("DATABASE_URL")
	invalid := errors.New("DATABASE_URL must be a valid PostgreSQL URL with host and database")
	u, err := url.Parse(value)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.Hostname() == "" || strings.Trim(u.Path, "/") == "" || u.Fragment != "" {
		return "", invalid
	}
	if _, err := pgxpool.ParseConfig(value); err != nil {
		return "", invalid
	}
	return value, nil
}
