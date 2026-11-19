// Package api serves the HTTP REST API.
package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/auth"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/clientip"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/db/games"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/db/players"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/online"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/ratelimit"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/seek"
)

// Per-IP limits on endpoints that auth.Service doesn't already limit.
// Login and registration are limited inside auth.Service, so the text
// protocol shares their limits.
const (
	tokenCallsPerIP = 30 // refresh, logout and guest, per minute
	seekListsPerIP  = 60 // per minute
	historyPerIP    = 60 // games history and ratings, per minute
	eventsPerIP     = 30 // per minute
	maxTrackedIPs   = 100_000
)

const maxBodyBytes = 64 * 1024

type Server struct {
	auth    *auth.Service
	players *online.Registry
	seeks   *seek.Manager
	// serviceKey authorizes service-to-service calls. Empty disables them.
	serviceKey string
	clientIP   *clientip.Resolver
	log        *slog.Logger

	games         *games.Store
	playerStore   *players.Store
	events        *eventsFeed
	publicGamesDB string

	tokenCalls  *ratelimit.Keyed
	seekLists   *ratelimit.Keyed
	historyRead *ratelimit.Keyed
	eventReads  *ratelimit.Keyed
}

// PublicData is what the public read-only endpoints (games history,
// ratings, events) serve from.
type PublicData struct {
	Games   *games.Store
	Players *players.Store
	// EventsCSVURL is the events sheet published as CSV.
	EventsCSVURL string
	// PublicGamesDB is the downloadable games DB; empty disables its info
	// endpoint.
	PublicGamesDB string
}

// NewServer returns the API. clientIP finds the client's address behind a
// reverse proxy, for rate limits and logs.
func NewServer(authSvc *auth.Service, players *online.Registry, seeks *seek.Manager, serviceKey string, clientIP *clientip.Resolver, public PublicData, log *slog.Logger) *Server {
	return &Server{
		auth: authSvc, players: players, seeks: seeks, serviceKey: serviceKey, clientIP: clientIP, log: log,
		games: public.Games, playerStore: public.Players, events: newEventsFeed(public.EventsCSVURL, log), publicGamesDB: public.PublicGamesDB,
		tokenCalls:  ratelimit.New(tokenCallsPerIP, time.Minute, maxTrackedIPs),
		seekLists:   ratelimit.New(seekListsPerIP, time.Minute, maxTrackedIPs),
		historyRead: ratelimit.New(historyPerIP, time.Minute, maxTrackedIPs),
		eventReads:  ratelimit.New(eventsPerIP, time.Minute, maxTrackedIPs),
	}
}

// PruneLimits forgets IPs whose limits have reset.
func (s *Server) PruneLimits() {
	s.tokenCalls.Prune(s.log, "token calls per IP")
	s.seekLists.Prune(s.log, "seek lists per IP")
	s.historyRead.Prune(s.log, "history reads per IP")
	s.eventReads.Prune(s.log, "event reads per IP")
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/auth/login", s.login)
	mux.HandleFunc("POST /api/auth/register", s.register)
	mux.HandleFunc("POST /api/auth/refresh", s.limited(s.tokenCalls, s.refresh))
	mux.HandleFunc("POST /api/auth/guest", s.limited(s.tokenCalls, s.guest))
	mux.HandleFunc("POST /api/auth/logout", s.limited(s.tokenCalls, s.logout))
	mux.HandleFunc("GET /api/v1/seeks", s.limited(s.seekLists, s.listSeeks))
	mux.HandleFunc("PUT /api/v1/seeks", s.createSeek)

	// Public read-only endpoints, at the paths the NestJS API served them on
	// (api.playtak.com), so clients don't change.
	public := func(k *ratelimit.Keyed, h http.HandlerFunc) http.HandlerFunc { return publicCORS(s.limited(k, h)) }
	mux.HandleFunc("GET /v1/games-history", public(s.historyRead, s.listGames))
	mux.HandleFunc("GET /v1/games-history/{id}", public(s.historyRead, s.getGame))
	mux.HandleFunc("GET /v1/games-history/ptn/{id}", public(s.historyRead, s.getGamePTN))
	mux.HandleFunc("GET /v1/games-history/db", public(s.historyRead, s.publicGamesDBInfo))
	mux.HandleFunc("GET /v1/ratings", public(s.historyRead, s.listRatings))
	mux.HandleFunc("GET /v1/ratings/{name}", public(s.historyRead, s.getRating))
	mux.HandleFunc("GET /events", public(s.eventReads, s.listEvents))
	mux.HandleFunc("GET /health", health)
	return mux
}

type tokenResponse struct {
	Name                  string    `json:"name"`
	AccessToken           string    `json:"accessToken"`
	AccessTokenExpiresAt  time.Time `json:"accessTokenExpiresAt"`
	RefreshToken          string    `json:"refreshToken"`
	RefreshTokenExpiresAt time.Time `json:"refreshTokenExpiresAt"`
}

func newTokenResponse(name string, pair auth.TokenPair) tokenResponse {
	return tokenResponse{
		Name:                  name,
		AccessToken:           pair.AccessToken,
		AccessTokenExpiresAt:  pair.AccessExpiresAt,
		RefreshToken:          pair.RefreshToken,
		RefreshTokenExpiresAt: pair.RefreshExpiresAt,
	}
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decode(w, r, &body) {
		return
	}
	record, err := s.auth.Login(r.Context(), body.Username, body.Password, s.clientIP.IP(r))
	if errors.Is(err, auth.ErrAuthFailed) {
		writeError(w, http.StatusUnauthorized, "invalid_credentials")
		return
	}
	if rateLimited(w, err) {
		return
	}
	if err != nil {
		s.internalError(w, "login", err)
		return
	}
	pair, err := s.auth.IssueTokens(r.Context(), record.ID)
	if err != nil {
		s.internalError(w, "issue tokens", err)
		return
	}
	writeJSON(w, http.StatusOK, newTokenResponse(record.Name, pair))
}

func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		Email    string `json:"email"`
	}
	if !decode(w, r, &body) {
		return
	}
	record, err := s.auth.Register(r.Context(), body.Username, body.Email, s.clientIP.IP(r))
	switch {
	case rateLimited(w, err):
	case err == nil:
		writeJSON(w, http.StatusCreated, map[string]string{"name": record.Name})
	case errors.Is(err, auth.ErrInvalidUsername):
		writeError(w, http.StatusBadRequest, "invalid_username")
	case errors.Is(err, auth.ErrInvalidEmail):
		writeError(w, http.StatusBadRequest, "invalid_email")
	case errors.Is(err, auth.ErrGuestName):
		writeError(w, http.StatusBadRequest, "guest_name")
	case errors.Is(err, auth.ErrNameTaken):
		writeError(w, http.StatusConflict, "name_taken")
	default:
		s.internalError(w, "register", err)
	}
}

func (s *Server) refresh(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RefreshToken string `json:"refreshToken"`
	}
	if !decode(w, r, &body) {
		return
	}
	pair, err := s.auth.Refresh(r.Context(), body.RefreshToken)
	if errors.Is(err, auth.ErrAuthFailed) {
		writeError(w, http.StatusUnauthorized, "invalid_token")
		return
	}
	if err != nil {
		s.internalError(w, "refresh", err)
		return
	}
	record, err := s.auth.Authenticate(r.Context(), pair.AccessToken)
	if err != nil {
		s.internalError(w, "load refreshed player", err)
		return
	}
	writeJSON(w, http.StatusOK, newTokenResponse(record.Name, pair))
}

// guest reserves a guest identity. Guests have no account, so instead of API
// tokens they get a guest token to log in with over telnet or WebSocket.
func (s *Server) guest(w http.ResponseWriter, r *http.Request) {
	token := auth.NewGuestToken()
	p := s.players.Guest(token)
	writeJSON(w, http.StatusOK, map[string]string{"name": p.Name, "guestToken": token})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RefreshToken string `json:"refreshToken"`
	}
	if r.ContentLength != 0 && !decode(w, r, &body) {
		return
	}
	if err := s.auth.Logout(r.Context(), bearerToken(r), body.RefreshToken); err != nil {
		s.internalError(w, "logout", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// limited refuses requests from an IP that has used up k.
func (s *Server) limited(k *ratelimit.Keyed, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := s.clientIP.IP(r)
		if ok, wait := k.Allow(ratelimit.IPKey(ip)); !ok {
			s.log.Warn("rate limited", "path", r.URL.Path, "ip", ip, "retry_in", wait.Round(time.Second))
			tooManyRequests(w, wait)
			return
		}
		h(w, r)
	}
}

// rateLimited answers 429 if err is a rate limit, reporting whether it did.
func rateLimited(w http.ResponseWriter, err error) bool {
	var rl *auth.RateLimitError
	if !errors.As(err, &rl) {
		return false
	}
	tooManyRequests(w, rl.RetryAfter)
	return true
}

func tooManyRequests(w http.ResponseWriter, wait time.Duration) {
	w.Header().Set("Retry-After", strconv.Itoa(int((wait+time.Second-1)/time.Second)))
	writeError(w, http.StatusTooManyRequests, "rate_limited")
}

func bearerToken(r *http.Request) string {
	token, _ := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	return token
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes)).Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]string{"error": code})
}

func (s *Server) internalError(w http.ResponseWriter, op string, err error) {
	s.log.Error(op, "err", err)
	writeError(w, http.StatusInternalServerError, "internal_error")
}
