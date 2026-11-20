package email

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

func TestLogKeepsBodiesAndAddressesOutOfInfo(t *testing.T) {
	var info, debug bytes.Buffer
	Log{Logger: slog.New(slog.NewTextHandler(&info, nil))}.Send(context.Background(), "a@example.com", "Password reset", "token: s3cret")
	if strings.Contains(info.String(), "s3cret") || strings.Contains(info.String(), "a@example.com") ||
		!strings.Contains(info.String(), "Password reset") {
		t.Errorf("info log: %s", info.String())
	}
	Log{Logger: slog.New(slog.NewTextHandler(&debug, &slog.HandlerOptions{Level: slog.LevelDebug}))}.Send(context.Background(), "a@example.com", "Password reset", "token: s3cret")
	if !strings.Contains(debug.String(), "s3cret") || !strings.Contains(debug.String(), "a@example.com") {
		t.Errorf("debug log should include the body and address for local development: %s", debug.String())
	}
}
