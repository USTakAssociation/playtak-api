package api

import (
	"errors"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/db/games"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/db/players"
)

const (
	defaultPageSize = 50
	// maxGamesPageSize bounds a games history page; each row carries the
	// whole move list.
	maxGamesPageSize = 1000
)

// page is the paginated list shape the history and ratings endpoints return.
type page[T any] struct {
	Items      []T `json:"items"`
	Total      int `json:"total"`
	Page       int `json:"page"` // 1-based
	PerPage    int `json:"perPage"`
	TotalPages int `json:"totalPages"`
}

// pagination reads limit, page and skip as the NestJS API did: page counts
// from 0, and skip is used only when page is 0 or missing.
func pagination(q url.Values, maxLimit int) (limit, pageNum, offset int) {
	limit = positiveInt(q.Get("limit"), defaultPageSize)
	if maxLimit > 0 {
		limit = min(limit, maxLimit)
	}
	pageNum = positiveInt(q.Get("page"), 0)
	offset = limit * pageNum
	if offset == 0 {
		offset = positiveInt(q.Get("skip"), 0)
	}
	return limit, pageNum, offset
}

func positiveInt(s string, def int) int {
	if n, err := strconv.Atoi(s); err == nil && n > 0 {
		return n
	}
	return def
}

func newPage[T any](items []T, total, limit, pageNum int) page[T] {
	return page[T]{Items: items, Total: total, Page: pageNum + 1, PerPage: limit, TotalPages: (total + limit - 1) / limit}
}

// sortOrder reads sort and order, defaulting to defaultSort descending.
func sortOrder(q url.Values, defaultSort string, valid map[string]bool) (sort string, desc, ok bool) {
	sort = q.Get("sort")
	if sort == "" {
		sort = defaultSort
	}
	switch strings.ToUpper(q.Get("order")) {
	case "", "DESC":
		desc = true
	case "ASC":
	default:
		return "", false, false
	}
	return sort, desc, valid[sort]
}

// intFilter parses an id or date filter: "5", "5-10" (either order), "5,7,9",
// ">5" or "<5".
func intFilter(s string) (games.IntFilter, bool) {
	var f games.IntFilter
	parse := func(v string) (*int64, bool) {
		n, err := strconv.ParseInt(v, 10, 64)
		return &n, err == nil
	}
	switch {
	case s == "":
		return f, true
	case strings.HasPrefix(s, ">"):
		n, ok := parse(s[1:])
		if ok {
			*n++
		}
		f.Min = n
		return f, ok
	case strings.HasPrefix(s, "<"):
		n, ok := parse(s[1:])
		if ok {
			*n--
		}
		f.Max = n
		return f, ok
	case strings.Contains(s, ","):
		for v := range strings.SplitSeq(s, ",") {
			n, ok := parse(v)
			if !ok {
				return f, false
			}
			f.In = append(f.In, *n)
		}
		return f, true
	case strings.Contains(s, "-"):
		lo, hi, _ := strings.Cut(s, "-")
		a, okA := parse(lo)
		b, okB := parse(strings.TrimLeft(hi, "-")) // "5--10" was accepted too
		if *a > *b {
			a, b = b, a
		}
		f.Min, f.Max = a, b
		return f, okA && okB
	}
	n, ok := parse(s)
	f.Min, f.Max = n, n
	return f, ok
}

// gameEqualParams are query parameters that filter an integer column by
// equality; their names are the column names.
var gameEqualParams = []string{"size", "timertime", "timerinc", "komi", "extra_time_amount", "extra_time_trigger", "increment_scales"}

func (s *Server) listGames(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, pageNum, offset := pagination(q, maxGamesPageSize)
	sort, desc, ok := sortOrder(q, "id", games.SortColumns)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_sort")
		return
	}
	search := games.Search{
		White:  q.Get("player_white"),
		Black:  q.Get("player_black"),
		Result: q.Get("game_result"),
		Mirror: q.Get("mirror") == "true",
		Equal:  map[string]int64{},
		Sort:   sort, Desc: desc, Limit: limit, Offset: offset,
	}
	var okID, okDate bool
	search.ID, okID = intFilter(q.Get("id"))
	search.Date, okDate = intFilter(q.Get("date"))
	if !okID || !okDate {
		writeError(w, http.StatusBadRequest, "invalid_filter")
		return
	}
	for _, name := range gameEqualParams {
		if v := q.Get(name); v != "" {
			n, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid_filter")
				return
			}
			search.Equal[name] = n
		}
	}
	switch strings.ToLower(q.Get("type")) {
	case "":
	case "normal":
		search.Equal["tournament"], search.Equal["unrated"] = 0, 0
	case "tournament":
		search.Equal["tournament"] = 1
	case "unrated":
		search.Equal["unrated"] = 1
	default:
		writeError(w, http.StatusBadRequest, "invalid_filter")
		return
	}

	items, total, err := s.games.History(r.Context(), search)
	if err != nil {
		s.internalError(w, "games history", err)
		return
	}
	writeJSON(w, http.StatusOK, newPage(items, total, limit, pageNum))
}

// gameByID loads the game named by the {id} path value, answering 400 or 404
// itself when it can't.
func (s *Server) gameByID(w http.ResponseWriter, r *http.Request) (games.Game, bool) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id")
		return games.Game{}, false
	}
	g, err := s.games.ByID(r.Context(), id)
	if errors.Is(err, games.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found")
		return games.Game{}, false
	}
	if err != nil {
		s.internalError(w, "get game", err)
		return games.Game{}, false
	}
	return g, true
}

func (s *Server) getGame(w http.ResponseWriter, r *http.Request) {
	if g, ok := s.gameByID(w, r); ok {
		writeJSON(w, http.StatusOK, g)
	}
}

func (s *Server) getGamePTN(w http.ResponseWriter, r *http.Request) {
	if g, ok := s.gameByID(w, r); ok {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write([]byte(gamePTN(g)))
	}
}

// publicGamesDBInfo reports the size and age of the downloadable games DB.
func (s *Server) publicGamesDBInfo(w http.ResponseWriter, r *http.Request) {
	if s.publicGamesDB == "" {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	info, err := os.Stat(s.publicGamesDB)
	if err != nil {
		s.internalError(w, "stat public games db", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"size":    info.Size(),
		"mtime":   info.ModTime().UTC().Format(time.RFC3339Nano),
		"mtimeMs": info.ModTime().UnixMilli(),
	})
}

func (s *Server) listRatings(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, pageNum, offset := pagination(q, 0)
	sort, desc, ok := sortOrder(q, "participation_rating", players.RatingSortColumns)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_sort")
		return
	}
	search := players.RatingSearch{Name: q.Get("name"), Sort: sort, Desc: desc, Limit: limit, Offset: offset}
	if v := q.Get("id"); v != "" {
		id, err := strconv.Atoi(v)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_filter")
			return
		}
		search.ID = id
	}
	items, total, err := s.playerStore.Ratings(r.Context(), search)
	if err != nil {
		s.internalError(w, "ratings", err)
		return
	}
	writeJSON(w, http.StatusOK, newPage(items, total, limit, pageNum))
}

func (s *Server) getRating(w http.ResponseWriter, r *http.Request) {
	rating, err := s.playerStore.RatingByName(r.Context(), r.PathValue("name"))
	if errors.Is(err, players.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		s.internalError(w, "get rating", err)
		return
	}
	writeJSON(w, http.StatusOK, rating)
}

func health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "timestamp": time.Now().UTC().Format(time.RFC3339Nano)})
}

// publicCORS lets any site read these public, read-only endpoints (the
// games history site is served from another origin).
func publicCORS(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		h(w, r)
	}
}
