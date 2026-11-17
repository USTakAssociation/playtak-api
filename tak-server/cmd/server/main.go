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
	"github.com/USTakAssociation/playtak-api/tak-server/internal/clientip"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/config"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/db"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/db/games"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/db/players"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/email"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/game"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/moderation"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/online"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/rating"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/seek"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/session"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/transport"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/transport/telnet"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/transport/websocket"
)

const (
	// guestIdleTimeout is how long an offline guest can still be rejoined
	// with its token.
	guestIdleTimeout = 6 * time.Hour
	cleanupInterval  = time.Hour
	shutdownTimeout  = 10 * time.Second
)

func main() {
	settings, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: settings.LogLevel}))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	// "tak-server rate" runs the rating job once and exits; cron runs it.
	if len(os.Args) > 1 && os.Args[1] == "rate" {
		if err := rate(ctx, settings, log); err != nil {
			log.Error("rating job failed", "err", err)
			os.Exit(1)
		}
		return
	}
	if err := run(ctx, settings, log); err != nil {
		log.Error("server stopped", "err", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, settings *config.Config, log *slog.Logger) error {
	playersDB, err := db.Open(ctx, settings.PlayersDB)
	if err != nil {
		return err
	}
	defer playersDB.Close()
	if err := players.CheckSchema(ctx, playersDB); err != nil {
		return err
	}
	gamesDB, err := db.Open(ctx, settings.GamesDB)
	if err != nil {
		return err
	}
	defer gamesDB.Close()

	var mailer auth.Mailer = email.Log{Logger: log}
	if settings.SMTP.Host != "" {
		mailer = email.NewSMTP(settings.SMTP.Host, settings.SMTP.User, settings.SMTP.Password, settings.SMTP.From)
	} else {
		log.Warn("no SMTP server configured; emails will be logged instead of sent")
	}

	playerStore := players.NewStore(playersDB)
	authSvc := auth.NewService(playerStore, players.NewTokenStore(playersDB), mailer, log)
	registry := online.NewRegistry()
	seeks := seek.NewManager()

	gameStore := games.NewStore(gamesDB)
	results := games.NewResults(gameStore, settings.PendingResultsFile, log)
	resultsDone := make(chan struct{})
	resultsCtx, stopResults := context.WithCancel(context.Background())
	go func() { results.Run(resultsCtx); close(resultsDone) }()
	defer func() { stopResults(); <-resultsDone }()

	gameManager := game.NewManager(game.Config{
		ReconnectTimeout:           settings.ReconnectTimeout,
		TournamentReconnectTimeout: settings.TournamentReconnectTimeout,
		CommandTimeout:             settings.GameCommandTimeout,
		AllowMultipleSessions:      settings.AllowMultipleSessions,
	}, gameStore, playerStore, results, seeks, registry, log)
	sessions := session.NewManager(ctx, session.Services{
		Auth:       authSvc,
		Players:    registry,
		Seeks:      seeks,
		Games:      gameManager,
		Chat:       chat.NewService(log),
		Moderation: moderation.NewService(playerStore, authSvc, gameManager, registry, mailer, log),
	}, session.Config{AllowMultipleSessions: settings.AllowMultipleSessions}, log)
	if settings.ServiceAPIKey == "" {
		log.Warn("TAK_SERVICE_API_KEY not set; service endpoints such as PUT /api/v1/seeks are disabled")
	}

	clientIP, err := clientip.NewResolver(settings.TrustedProxies)
	if err != nil {
		return fmt.Errorf("TAK_TRUSTED_PROXIES: %w", err)
	}
	authSvc.EnableLimits(settings.LoginLimitByUsername)
	apiServer := api.NewServer(authSvc, registry, seeks, settings.ServiceAPIKey, clientIP, api.PublicData{
		Games:         gameStore,
		Players:       playerStore,
		EventsCSVURL:  settings.EventsCSVURL,
		PublicGamesDB: settings.PublicGamesDB,
	}, log)

	telnetLn, err := net.Listen("tcp", settings.TelnetAddr)
	if err != nil {
		return err
	}
	wsSrv := &http.Server{
		Addr:              settings.WSAddr,
		Handler:           &websocket.Handler{Acceptor: sessions, Log: log, ClientIP: clientIP},
		ReadHeaderTimeout: 10 * time.Second,
	}
	apiSrv := &http.Server{
		Addr:              settings.HTTPAddr,
		Handler:           apiServer.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	errc := make(chan error, 3)
	go func() {
		log.Info("telnet listening", "addr", settings.TelnetAddr)
		errc <- (&telnet.Server{Acceptor: sessions, Log: log}).Serve(ctx, telnetLn)
	}()
	go func() {
		log.Info("websocket listening", "addr", settings.WSAddr)
		errc <- ignoreClosed(wsSrv.ListenAndServe())
	}()
	go func() {
		log.Info("http api listening", "addr", settings.HTTPAddr)
		errc <- ignoreClosed(apiSrv.ListenAndServe())
	}()
	go cleanupLoop(ctx, authSvc, apiServer, registry, log)

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

// rate runs the rating job once over the games since the last run.
func rate(ctx context.Context, settings *config.Config, log *slog.Logger) error {
	gamesDB, err := db.Open(ctx, settings.GamesDB)
	if err != nil {
		return err
	}
	defer gamesDB.Close()
	conn, err := gamesDB.Conn(ctx) // ATTACH applies to one connection
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := rating.AttachPlayers(ctx, conn, settings.PlayersDB); err != nil {
		return err
	}
	if settings.RatingListFile == "" {
		log.Warn("TAK_RATING_LIST_FILE not set; the ratings page list will not be updated")
	}
	start := time.Now()
	result, err := rating.Run(ctx, conn, rating.Files{Cursor: settings.RatingCursorFile, RatingList: settings.RatingListFile}, start)
	if err != nil {
		return err
	}
	log.Info("ratings updated", "from_game", result.From, "to_game", result.To, "games_written", result.Games, "took", time.Since(start).Round(time.Millisecond))
	return nil
}

// cleanupLoop hourly forgets idle guests and rate-limit counters that have
// reset, and deletes expired API tokens.
func cleanupLoop(ctx context.Context, authSvc *auth.Service, apiServer *api.Server, registry *online.Registry, log *slog.Logger) {
	t := time.NewTicker(cleanupInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		authSvc.PruneLimits()
		apiServer.PruneLimits()
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
