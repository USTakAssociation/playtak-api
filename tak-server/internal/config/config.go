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
	"path/filepath"
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
	// TournamentReconnectTimeout is the same for tournament games.
	TournamentReconnectTimeout time.Duration // TAK_TOURNAMENT_RECONNECT_TIMEOUT
	// GameCommandTimeout bounds how long a command waits for its game.
	GameCommandTimeout time.Duration // TAK_GAME_COMMAND_TIMEOUT
	// PendingResultsFile holds finished games until they're written to the
	// games DB. Defaults to pending-game-results.jsonl beside the games DB.
	PendingResultsFile string // TAK_PENDING_RESULTS_FILE

	// EventSubscriberURL receives game update webhooks. Empty disables them.
	EventSubscriberURL string // TAK_EVENT_SUBSCRIBER_URL

	// AllowMultipleSessions lets a player be logged in on several
	// connections at once. When false, a new login disconnects the old one.
	AllowMultipleSessions bool // TAK_ALLOW_MULTIPLE_SESSIONS

	LogLevel slog.Level // TAK_LOG_LEVEL: debug, info, warn or error

	// TrustedProxies lists the reverse proxies (CIDRs, comma-separated) whose
	// X-Real-IP / X-Forwarded-For headers are believed. Requests from
	// anywhere else are judged by their connection address.
	TrustedProxies string // TAK_TRUSTED_PROXIES

	// LoginLimitByUsername counts failed logins per username as well as per
	// IP. TAK_LOGIN_LIMIT: "username" (default) or "ip" for per-IP only.
	LoginLimitByUsername bool

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

	var durationErr error
	duration := func(key, def string) time.Duration {
		d, err := time.ParseDuration(get(key, def))
		if err != nil && durationErr == nil {
			durationErr = fmt.Errorf("%s: %w", key, err)
		}
		return d
	}
	reconnect := duration("TAK_RECONNECT_TIMEOUT", "2m")
	tournamentReconnect := duration("TAK_TOURNAMENT_RECONNECT_TIMEOUT", "15m")
	commandTimeout := duration("TAK_GAME_COMMAND_TIMEOUT", "10s")
	if durationErr != nil {
		return nil, durationErr
	}
	gamesDB := get("TAK_GAMES_DB", "/playtakdb/games.db")

	multi, err := strconv.ParseBool(get("TAK_ALLOW_MULTIPLE_SESSIONS", "false"))
	if err != nil {
		return nil, fmt.Errorf("TAK_ALLOW_MULTIPLE_SESSIONS: %w", err)
	}

	var byUsername bool
	switch mode := get("TAK_LOGIN_LIMIT", "username"); mode {
	case "username":
		byUsername = true
	case "ip":
	default:
		return nil, fmt.Errorf("TAK_LOGIN_LIMIT: %q is not \"username\" or \"ip\"", mode)
	}

	var level slog.Level
	if err := level.UnmarshalText([]byte(get("TAK_LOG_LEVEL", "info"))); err != nil {
		return nil, fmt.Errorf("TAK_LOG_LEVEL: %w", err)
	}

	return &Config{
		TelnetAddr:                 get("TAK_TELNET_ADDR", ":10000"),
		WSAddr:                     get("TAK_WS_ADDR", ":9999"),
		HTTPAddr:                   get("TAK_HTTP_ADDR", ":9998"),
		PlayersDB:                  get("TAK_PLAYERS_DB", "/playtakdb/players.db"),
		GamesDB:                    gamesDB,
		ReconnectTimeout:           reconnect,
		TournamentReconnectTimeout: tournamentReconnect,
		GameCommandTimeout:         commandTimeout,
		PendingResultsFile:         get("TAK_PENDING_RESULTS_FILE", filepath.Join(filepath.Dir(gamesDB), "pending-game-results.jsonl")),
		EventSubscriberURL:         getenv("TAK_EVENT_SUBSCRIBER_URL"),
		AllowMultipleSessions:      multi,
		LogLevel:                   level,
		ServiceAPIKey:              getenv("TAK_SERVICE_API_KEY"),
		TrustedProxies:             get("TAK_TRUSTED_PROXIES", "127.0.0.1/32,::1/128"),
		LoginLimitByUsername:       byUsername,
		SMTP: SMTP{
			Host:     getenv("TAK_SMTP_HOST"),
			User:     getenv("TAK_SMTP_USER"),
			Password: getenv("TAK_SMTP_PASSWORD"),
			From:     getenv("TAK_SMTP_FROM"),
		},
	}, nil
}
