package email

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

func TestLogKeepsBodiesOutOfInfo(t *testing.T) {
	var info, debug bytes.Buffer
	Log{Logger: slog.New(slog.NewTextHandler(&info, nil))}.Send(context.Background(), "a@example.com", "Password reset", "token: s3cret")
	if strings.Contains(info.String(), "s3cret") || !strings.Contains(info.String(), "a@example.com") {
		t.Errorf("info log: %s", info.String())
	}
	Log{Logger: slog.New(slog.NewTextHandler(&debug, &slog.HandlerOptions{Level: slog.LevelDebug}))}.Send(context.Background(), "a@example.com", "Password reset", "token: s3cret")
	if !strings.Contains(debug.String(), "s3cret") {
		t.Errorf("debug log should include the body for local development: %s", debug.String())
	}
}
