package config

import (
	"log/slog"
	"testing"
	"time"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestFromEnvDefaults(t *testing.T) {
	cfg, err := FromEnv(env(nil))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.TelnetAddr != ":10000" || cfg.WSAddr != ":9999" || cfg.HTTPAddr != ":9998" {
		t.Errorf("addrs = %q %q %q", cfg.TelnetAddr, cfg.WSAddr, cfg.HTTPAddr)
	}
	if cfg.ReconnectTimeout != 2*time.Minute {
		t.Errorf("ReconnectTimeout = %v", cfg.ReconnectTimeout)
	}
	if cfg.SMTP.Host != "" {
		t.Errorf("SMTP should be disabled by default, got host %q", cfg.SMTP.Host)
	}
	if cfg.AllowMultipleSessions {
		t.Error("multiple sessions should be off by default")
	}
	if cfg.TournamentReconnectTimeout != 15*time.Minute || cfg.GameCommandTimeout != 10*time.Second {
		t.Errorf("timeouts %v %v", cfg.TournamentReconnectTimeout, cfg.GameCommandTimeout)
	}
	if cfg.PendingResultsFile != "/playtakdb/pending-game-results.jsonl" {
		t.Errorf("PendingResultsFile = %q", cfg.PendingResultsFile)
	}
	if cfg.LogLevel != slog.LevelInfo {
		t.Errorf("LogLevel = %v", cfg.LogLevel)
	}
	if cfg.TrustedProxies != "127.0.0.1/32,::1/128" || !cfg.LoginLimitByUsername {
		t.Errorf("TrustedProxies = %q, LoginLimitByUsername = %v", cfg.TrustedProxies, cfg.LoginLimitByUsername)
	}
	if cfg.MaxConnectionsPerIP != 100 || cfg.ConnectionLimitExempt != "" {
		t.Errorf("MaxConnectionsPerIP = %d, ConnectionLimitExempt = %q", cfg.MaxConnectionsPerIP, cfg.ConnectionLimitExempt)
	}
}

func TestFromEnvOverrides(t *testing.T) {
	cfg, err := FromEnv(env(map[string]string{
		"TAK_TELNET_ADDR":             ":10002",
		"TAK_PLAYERS_DB":              "./playtakdb/players.db",
		"TAK_RECONNECT_TIMEOUT":       "30s",
		"TAK_SMTP_HOST":               "smtp.example.com",
		"TAK_ALLOW_MULTIPLE_SESSIONS": "true",
		"TAK_LOG_LEVEL":               "debug",
		"TAK_TRUSTED_PROXIES":         "10.0.0.0/8",
		"TAK_LOGIN_LIMIT":             "ip",
		"TAK_MAX_CONNECTIONS_PER_IP":  "0",
		"TAK_CONNECTION_LIMIT_EXEMPT": "10.0.0.5",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.TelnetAddr != ":10002" || cfg.PlayersDB != "./playtakdb/players.db" {
		t.Errorf("got %q %q", cfg.TelnetAddr, cfg.PlayersDB)
	}
	if cfg.ReconnectTimeout != 30*time.Second {
		t.Errorf("ReconnectTimeout = %v", cfg.ReconnectTimeout)
	}
	if cfg.SMTP.Host != "smtp.example.com" {
		t.Errorf("SMTP.Host = %q", cfg.SMTP.Host)
	}
	if !cfg.AllowMultipleSessions || cfg.LogLevel != slog.LevelDebug {
		t.Errorf("AllowMultipleSessions = %v, LogLevel = %v", cfg.AllowMultipleSessions, cfg.LogLevel)
	}
	if cfg.TrustedProxies != "10.0.0.0/8" || cfg.LoginLimitByUsername {
		t.Errorf("TrustedProxies = %q, LoginLimitByUsername = %v", cfg.TrustedProxies, cfg.LoginLimitByUsername)
	}
	if cfg.MaxConnectionsPerIP != 0 || cfg.ConnectionLimitExempt != "10.0.0.5" {
		t.Errorf("MaxConnectionsPerIP = %d, ConnectionLimitExempt = %q", cfg.MaxConnectionsPerIP, cfg.ConnectionLimitExempt)
	}
}

func TestFromEnvInvalidValues(t *testing.T) {
	for k, v := range map[string]string{
		"TAK_RECONNECT_TIMEOUT":       "120", // no unit
		"TAK_ALLOW_MULTIPLE_SESSIONS": "maybe",
		"TAK_LOG_LEVEL":               "loud",
		"TAK_GAME_COMMAND_TIMEOUT":    "soon",
		"TAK_LOGIN_LIMIT":             "account",
		"TAK_MAX_CONNECTIONS_PER_IP":  "-1",
	} {
		if _, err := FromEnv(env(map[string]string{k: v})); err == nil {
			t.Errorf("%s=%q: expected error", k, v)
		}
	}
}
