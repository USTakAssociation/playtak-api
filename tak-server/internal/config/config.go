// Package config loads server settings from environment variables. A .env
// file in the working directory, if present, is loaded first; variables
// already set in the environment take precedence over it.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	TelnetAddr string // TAK_TELNET_ADDR
	WSAddr     string // TAK_WS_ADDR
	HTTPAddr   string // TAK_HTTP_ADDR

	PlayersDB string // TAK_PLAYERS_DB
	GamesDB   string // TAK_GAMES_DB

	// ReconnectTimeout is how long a disconnected player's game is held open.
	ReconnectTimeout time.Duration // TAK_RECONNECT_TIMEOUT

	// EventSubscriberURL receives game update webhooks. Empty disables them.
	EventSubscriberURL string // TAK_EVENT_SUBSCRIBER_URL

	// BadWordsURL is the CSV word list for the profanity filter.
	BadWordsURL string // TAK_BADWORDS_URL

	// AllowMultipleSessions lets a player be logged in on several
	// connections at once. When false, a new login disconnects the old one.
	AllowMultipleSessions bool // TAK_ALLOW_MULTIPLE_SESSIONS

	LogLevel slog.Level // TAK_LOG_LEVEL: debug, info, warn or error

	// ServiceAPIKey authorizes service-to-service HTTP calls, such as the
	// tournament service creating seeks. Empty disables those endpoints.
	ServiceAPIKey string // TAK_SERVICE_API_KEY

	SMTP SMTP
}

type SMTP struct {
	Host     string // TAK_SMTP_HOST, "host" or "host:port" (default 465); empty disables email
	User     string // TAK_SMTP_USER
	Password string // TAK_SMTP_PASSWORD
	From     string // TAK_SMTP_FROM
}

const defaultBadWordsURL = "https://docs.google.com/spreadsheets/d/1eVhQjZe1wbZdPQ4a2IlZ-jJUsgZmVoyrzXAjlQpA7hk/export?format=csv"

// Load reads .env (if present) and then the environment.
func Load() (*Config, error) {
	if err := godotenv.Load(); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("load .env: %w", err)
	}
	return FromEnv(os.Getenv)
}

// FromEnv builds a Config using getenv to look up variables.
func FromEnv(getenv func(string) string) (*Config, error) {
	get := func(key, def string) string {
		if v := getenv(key); v != "" {
			return v
		}
		return def
	}

	reconnect, err := time.ParseDuration(get("TAK_RECONNECT_TIMEOUT", "2m"))
	if err != nil {
		return nil, fmt.Errorf("TAK_RECONNECT_TIMEOUT: %w", err)
	}

	multi, err := strconv.ParseBool(get("TAK_ALLOW_MULTIPLE_SESSIONS", "false"))
	if err != nil {
		return nil, fmt.Errorf("TAK_ALLOW_MULTIPLE_SESSIONS: %w", err)
	}

	var level slog.Level
	if err := level.UnmarshalText([]byte(get("TAK_LOG_LEVEL", "info"))); err != nil {
		return nil, fmt.Errorf("TAK_LOG_LEVEL: %w", err)
	}

	return &Config{
		TelnetAddr:            get("TAK_TELNET_ADDR", ":10000"),
		WSAddr:                get("TAK_WS_ADDR", ":9999"),
		HTTPAddr:              get("TAK_HTTP_ADDR", ":9998"),
		PlayersDB:             get("TAK_PLAYERS_DB", "/playtakdb/players.db"),
		GamesDB:               get("TAK_GAMES_DB", "/playtakdb/games.db"),
		ReconnectTimeout:      reconnect,
		EventSubscriberURL:    getenv("TAK_EVENT_SUBSCRIBER_URL"),
		BadWordsURL:           get("TAK_BADWORDS_URL", defaultBadWordsURL),
		AllowMultipleSessions: multi,
		LogLevel:              level,
		ServiceAPIKey:         getenv("TAK_SERVICE_API_KEY"),
		SMTP: SMTP{
			Host:     getenv("TAK_SMTP_HOST"),
			User:     getenv("TAK_SMTP_USER"),
			Password: getenv("TAK_SMTP_PASSWORD"),
			From:     getenv("TAK_SMTP_FROM"),
		},
	}, nil
}
