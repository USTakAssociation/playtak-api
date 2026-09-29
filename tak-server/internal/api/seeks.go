package api

import (
	"crypto/subtle"
	"math"
	"net/http"
	"slices"
	"strings"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/protocol"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/seek"
)

// seekDTO is the JSON form of a seek. Field names match the Java server's so
// existing clients (the tournament service) keep working.
type seekDTO struct {
	ID                   int     `json:"id"`
	PNTID                *int    `json:"pntId"`
	Creator              string  `json:"creator"`
	Opponent             *string `json:"opponent"`
	TimeContingent       int     `json:"timeContingent"`
	TimeIncrement        int     `json:"timeIncrement"`
	ExtraTimeTriggerMove int     `json:"extraTimeTriggerMove"`
	ExtraTimeAmount      int     `json:"extraTimeAmount"`
	IncrementScales      bool    `json:"incrementScales"`
	Opening              string  `json:"opening"`
	Komi                 float64 `json:"komi"` // in flats
	BoardSize            int     `json:"boardSize"`
	Capstones            int     `json:"capstones"`
	Pieces               int     `json:"pieces"`
	Unrated              bool    `json:"unrated"`
	Tournament           bool    `json:"tournament"`
	Color                string  `json:"color"` // WHITE, BLACK or ANY
}

var colorNames = map[protocol.Color]string{protocol.AnyColor: "ANY", protocol.White: "WHITE", protocol.Black: "BLACK"}

func newSeekDTO(s seek.Seek) seekDTO {
	d := seekDTO{
		ID:                   s.ID,
		Creator:              s.Creator,
		TimeContingent:       s.Time,
		TimeIncrement:        s.Increment,
		ExtraTimeTriggerMove: s.ExtraTimeTrigger,
		ExtraTimeAmount:      s.ExtraTimeAmount,
		IncrementScales:      s.IncrementScales,
		Opening:              s.Opening.PTN(),
		Komi:                 float64(s.Komi) / 2,
		BoardSize:            s.Size,
		Capstones:            s.Capstones,
		Pieces:               s.Pieces,
		Unrated:              s.Unrated,
		Tournament:           s.Tournament,
		Color:                colorNames[s.Color],
	}
	if s.PNTID != 0 {
		d.PNTID = &s.PNTID
	}
	if s.Opponent != "" {
		d.Opponent = &s.Opponent
	}
	return d
}

func (d seekDTO) settings() (protocol.GameSettings, bool) {
	g := protocol.GameSettings{
		Size:             d.BoardSize,
		Time:             d.TimeContingent,
		Increment:        d.TimeIncrement,
		IncrementScales:  d.IncrementScales,
		Komi:             int(math.Round(d.Komi * 2)),
		Pieces:           d.Pieces,
		Capstones:        d.Capstones,
		Unrated:          d.Unrated,
		Tournament:       d.Tournament,
		ExtraTimeTrigger: d.ExtraTimeTriggerMove,
		ExtraTimeAmount:  d.ExtraTimeAmount,
	}
	if strings.EqualFold(d.Opening, protocol.OpeningDoubleBlackStack.PTN()) {
		g.Opening = protocol.OpeningDoubleBlackStack
	}
	switch d.Color {
	case "", "ANY":
	case "WHITE":
		g.Color = protocol.White
	case "BLACK":
		g.Color = protocol.Black
	default:
		return g, false
	}
	return g, true
}

func (s *Server) listSeeks(w http.ResponseWriter, r *http.Request) {
	seeks := s.seeks.List()
	out := make([]seekDTO, len(seeks))
	for i, sk := range seeks {
		out[i] = newSeekDTO(sk)
	}
	writeJSON(w, http.StatusOK, out)
}

// createSeek posts a seek on behalf of an online player, for services such as
// tournament scheduling. It requires the service API key.
func (s *Server) createSeek(w http.ResponseWriter, r *http.Request) {
	if !s.isService(r) {
		writeError(w, http.StatusUnauthorized, "invalid_service_key")
		return
	}
	var req seekDTO
	if !decode(w, r, &req) {
		return
	}
	settings, ok := req.settings()
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_color")
		return
	}

	p := s.players.Lookup(req.Creator)
	if p == nil || !p.Online() {
		writeError(w, http.StatusNotFound, "creator_not_online")
		return
	}
	// The seek belongs to one of the creator's sessions: the most recent one
	// that can display it.
	clients := p.Clients()
	slices.Reverse(clients)
	var owner seek.Listener
	for _, c := range clients {
		if c.ProtocolVersion() >= settings.RequiredProtocolVersion() {
			owner = c
			break
		}
	}
	if owner == nil {
		writeError(w, http.StatusUnprocessableEntity, "incompatible_protocol")
		return
	}

	opponent := ""
	if req.Opponent != nil {
		opponent = *req.Opponent
	}
	pntID := 0
	if req.PNTID != nil {
		pntID = *req.PNTID
	}
	created, err := s.seeks.Post(owner, seek.Request{
		Creator: p.Name, Bot: p.IsBot(), Opponent: opponent, Settings: settings, PNTID: pntID,
	})
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "incompatible_protocol")
		return
	}
	writeJSON(w, http.StatusOK, newSeekDTO(created))
}

func (s *Server) isService(r *http.Request) bool {
	token := bearerToken(r)
	return s.serviceKey != "" && token != "" &&
		subtle.ConstantTimeCompare([]byte(token), []byte(s.serviceKey)) == 1
}
