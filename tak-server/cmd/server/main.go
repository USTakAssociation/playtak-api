// Command server runs the PlayTak game server: the text protocol over telnet
// and WebSocket, and the REST API.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/api"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/auth"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/chat"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/config"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/db"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/db/games"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/db/players"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/email"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/game"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/moderation"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/player"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/seek"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/session"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/transport"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/transport/telnet"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/transport/websocket"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/wordfilter"
)

const (
	// guestIdleTimeout is how long an offline guest can still be rejoined
	// with its token.
	guestIdleTimeout = 6 * time.Hour
	cleanupInterval  = time.Hour
	shutdownTimeout  = 10 * time.Second
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: cfg.LogLevel}))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, cfg, log); err != nil {
		log.Error("server stopped", "err", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, cfg *config.Config, log *slog.Logger) error {
	playersDB, err := db.Open(ctx, cfg.PlayersDB)
	if err != nil {
		return err
	}
	defer playersDB.Close()
	if err := players.CheckSchema(ctx, playersDB); err != nil {
		return err
	}
	gamesDB, err := db.Open(ctx, cfg.GamesDB)
	if err != nil {
		return err
	}
	defer gamesDB.Close()

	var filter wordfilter.Filter
	loadCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	if n, err := filter.Load(loadCtx, cfg.BadWordsURL); err != nil {
		log.Warn("profanity filter disabled", "err", err)
	} else {
		log.Info("loaded profanity filter", "words", n)
	}
	cancel()

	var mailer auth.Mailer = email.Log{Logger: log}
	if cfg.SMTP.Host != "" {
		mailer = email.NewSMTP(cfg.SMTP.Host, cfg.SMTP.User, cfg.SMTP.Password, cfg.SMTP.From)
	} else {
		log.Warn("no SMTP server configured; emails will be logged instead of sent")
	}

	playerStore := players.NewStore(playersDB)
	authSvc := auth.NewService(playerStore, players.NewTokenStore(playersDB), mailer, &filter, log)
	registry := player.NewRegistry()
	seeks := seek.NewManager()

	gameStore := games.NewStore(gamesDB)
	results := games.NewResults(gameStore, cfg.PendingResultsFile, log)
	resultsDone := make(chan struct{})
	resultsCtx, stopResults := context.WithCancel(context.Background())
	go func() { results.Run(resultsCtx); close(resultsDone) }()
	defer func() { stopResults(); <-resultsDone }()

	gameManager := game.NewManager(game.Config{
		ReconnectTimeout:           cfg.ReconnectTimeout,
		TournamentReconnectTimeout: cfg.TournamentReconnectTimeout,
		CommandTimeout:             cfg.GameCommandTimeout,
		AllowMultipleSessions:      cfg.AllowMultipleSessions,
	}, gameStore, playerStore, results, seeks, registry, log)
	reloadWords := func(ctx context.Context) (int, error) { return filter.Load(ctx, cfg.BadWordsURL) }
	sessions := session.NewManager(ctx, session.Services{
		Auth:       authSvc,
		Players:    registry,
		Seeks:      seeks,
		Games:      gameManager,
		Chat:       chat.NewService(&filter, log),
		Moderation: moderation.NewService(playerStore, authSvc, gameManager, registry, mailer, reloadWords, log),
	}, session.Config{AllowMultipleSessions: cfg.AllowMultipleSessions}, log)
	if cfg.ServiceAPIKey == "" {
		log.Warn("TAK_SERVICE_API_KEY not set; service endpoints such as PUT /api/v1/seeks are disabled")
	}

	telnetLn, err := net.Listen("tcp", cfg.TelnetAddr)
	if err != nil {
		return err
	}
	wsSrv := &http.Server{
		Addr:              cfg.WSAddr,
		Handler:           &websocket.Handler{Acceptor: sessions, Log: log},
		ReadHeaderTimeout: 10 * time.Second,
	}
	apiSrv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           api.NewServer(authSvc, registry, seeks, cfg.ServiceAPIKey, log).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	errc := make(chan error, 3)
	go func() {
		log.Info("telnet listening", "addr", cfg.TelnetAddr)
		errc <- (&telnet.Server{Acceptor: sessions, Log: log}).Serve(ctx, telnetLn)
	}()
	go func() {
		log.Info("websocket listening", "addr", cfg.WSAddr)
		errc <- ignoreClosed(wsSrv.ListenAndServe())
	}()
	go func() {
		log.Info("http api listening", "addr", cfg.HTTPAddr)
		errc <- ignoreClosed(apiSrv.ListenAndServe())
	}()
	go cleanupLoop(ctx, authSvc, registry, log)

	select {
	case <-ctx.Done():
		log.Info("shutting down")
	case err = <-errc:
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	telnetLn.Close()
	sessions.CloseAll(transport.CloseGoingAway)
	wsSrv.Shutdown(shutdownCtx)
	apiSrv.Shutdown(shutdownCtx)
	return err
}

// cleanupLoop forgets idle guests and deletes expired API tokens hourly.
func cleanupLoop(ctx context.Context, authSvc *auth.Service, registry *player.Registry, log *slog.Logger) {
	t := time.NewTicker(cleanupInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		if n := registry.PruneGuests(guestIdleTimeout); n > 0 {
			log.Info("pruned idle guests", "count", n)
		}
		if n, err := authSvc.PurgeExpiredTokens(ctx); err != nil {
			log.Error("purge expired tokens", "err", err)
		} else if n > 0 {
			log.Info("purged expired tokens", "count", n)
		}
	}
}

func ignoreClosed(err error) error {
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
