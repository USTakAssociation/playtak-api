package api

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"
)

// eventsCacheFor is how long the events sheet is cached before it's fetched
// again.
const eventsCacheFor = 30 * time.Minute

// eventsRetryAfter is how long a failed fetch is remembered before trying
// again, so requests don't each wait out the timeout while the sheet is down.
const eventsRetryAfter = time.Minute

type event struct {
	Name         string  `json:"name"`
	Event        string  `json:"event"`
	Category     string  `json:"category"`
	StartDate    *string `json:"start_date"`
	EndDate      *string `json:"end_date"`
	Details      *string `json:"details"`
	Registration *string `json:"registration"`
	Standings    *string `json:"standings"`
}

type eventList struct {
	Data       []event  `json:"data"`
	Categories []string `json:"categories"`
}

// eventsFeed serves the USTA events Google Sheet, published as CSV, cached
// for eventsCacheFor.
type eventsFeed struct {
	url    string
	client *http.Client
	log    *slog.Logger

	mu       sync.Mutex
	cached   *eventList
	fetched  time.Time
	failedAt time.Time // the last failed fetch, zero after a success
	failErr  error
}

func newEventsFeed(url string, log *slog.Logger) *eventsFeed {
	return &eventsFeed{url: url, client: &http.Client{Timeout: 10 * time.Second}, log: log}
}

// get returns the events, fetching them if the cache has expired. If the
// fetch fails it falls back to the expired copy, if there is one, and
// doesn't try again for eventsRetryAfter.
func (f *eventsFeed) get(ctx context.Context) (*eventList, error) {
	f.mu.Lock() // also makes concurrent requests share one fetch
	defer f.mu.Unlock()
	if f.cached != nil && time.Since(f.fetched) < eventsCacheFor {
		return f.cached, nil
	}
	if !f.failedAt.IsZero() && time.Since(f.failedAt) < eventsRetryAfter {
		return f.fallback(f.failErr)
	}
	// Not the request's context: one client hanging up mustn't abort the
	// fetch the others are waiting on. The client's timeout still bounds it.
	list, err := f.fetch(context.WithoutCancel(ctx))
	if err != nil {
		f.failedAt, f.failErr = time.Now(), err
		f.log.Warn("events fetch failed", "err", err, "have_cached_copy", f.cached != nil)
		return f.fallback(err)
	}
	f.cached, f.fetched, f.failedAt, f.failErr = list, time.Now(), time.Time{}, nil
	return list, nil
}

// fallback serves the expired copy if there is one, or err.
func (f *eventsFeed) fallback(err error) (*eventList, error) {
	if f.cached != nil {
		return f.cached, nil
	}
	return nil, err
}

func (f *eventsFeed) fetch(ctx context.Context) (*eventList, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch events: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch events: %s", resp.Status)
	}
	return parseEvents(io.LimitReader(resp.Body, 1<<20))
}

// parseEvents reads the sheet's "Event List" tab: a header row, then name,
// type, category, start date, end date, details, registration and standings
// columns.
func parseEvents(r io.Reader) (*eventList, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1
	records, err := cr.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parse events: %w", err)
	}
	list := &eventList{Data: []event{}, Categories: []string{"All"}}
	for i, rec := range records {
		if i == 0 || strings.Join(rec, "") == "" { // header, blank rows
			continue
		}
		rec = append(rec, make([]string, max(0, 8-len(rec)))...)
		list.Data = append(list.Data, event{
			Name:         rec[0],
			Event:        rec[1],
			Category:     rec[2],
			StartDate:    nonEmpty(rec[3]),
			EndDate:      nonEmpty(rec[4]),
			Details:      link(rec[5]),
			Registration: link(rec[6]),
			Standings:    link(rec[7]),
		})
		if rec[2] != "" && !slices.Contains(list.Categories, rec[2]) {
			list.Categories = append(list.Categories, rec[2])
		}
	}
	return list, nil
}

func nonEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func link(s string) *string {
	if !strings.HasPrefix(s, "http") {
		return nil
	}
	return &s
}

func (s *Server) listEvents(w http.ResponseWriter, r *http.Request) {
	list, err := s.events.get(r.Context())
	if err != nil {
		s.log.Error("events", "err", err)
		writeError(w, http.StatusBadGateway, "events_unavailable")
		return
	}
	w.Header().Set("Cache-Control", "max-age=300")
	writeJSON(w, http.StatusOK, list)
}
