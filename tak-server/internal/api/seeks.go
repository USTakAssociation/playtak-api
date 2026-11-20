package api

import (
	"crypto/subtle"
	"errors"
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

func newSeekDTO(openSeek seek.Seek) seekDTO {
	response := seekDTO{
		ID:                   openSeek.ID,
		Creator:              openSeek.Creator,
		TimeContingent:       openSeek.Time,
		TimeIncrement:        openSeek.Increment,
		ExtraTimeTriggerMove: openSeek.ExtraTimeTrigger,
		ExtraTimeAmount:      openSeek.ExtraTimeAmount,
		IncrementScales:      openSeek.IncrementScales,
		Opening:              openSeek.Opening.PTN(),
		Komi:                 float64(openSeek.Komi) / 2,
		BoardSize:            openSeek.Size,
		Capstones:            openSeek.Capstones,
		Pieces:               openSeek.Pieces,
		Unrated:              openSeek.Unrated,
		Tournament:           openSeek.Tournament,
		Color:                colorNames[openSeek.Color],
	}
	if openSeek.PNTID != 0 {
		response.PNTID = &openSeek.PNTID
	}
	if openSeek.Opponent != "" {
		response.Opponent = &openSeek.Opponent
	}
	return response
}

func (d seekDTO) settings() (protocol.GameSettings, bool) {
	settings := protocol.GameSettings{
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
		settings.Opening = protocol.OpeningDoubleBlackStack
	}
	switch d.Color {
	case "", "ANY":
	case "WHITE":
		settings.Color = protocol.White
	case "BLACK":
		settings.Color = protocol.Black
	default:
		return settings, false
	}
	return settings, true
}

func (s *Server) listSeeks(w http.ResponseWriter, r *http.Request) {
	seeks := s.seeks.List()
	out := make([]seekDTO, len(seeks))
	for i, openSeek := range seeks {
		out[i] = newSeekDTO(openSeek)
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
	var body seekDTO
	if !decode(w, r, &body) {
		return
	}
	settings, ok := body.settings()
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_color")
		return
	}

	player := s.players.Lookup(body.Creator)
	if player == nil || !player.Online() {
		writeError(w, http.StatusNotFound, "creator_not_online")
		return
	}
	// The seek belongs to one of the creator's sessions: the most recent one
	// that can display it.
	clients := player.Clients()
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
	if body.Opponent != nil {
		opponent = *body.Opponent
	}
	pntID := 0
	if body.PNTID != nil {
		pntID = *body.PNTID
	}
	created, err := s.seeks.Post(owner, seek.Request{
		Creator: player.Name, Bot: player.IsBot(), Opponent: opponent, Settings: settings, PNTID: pntID,
	})
	if errors.Is(err, seek.ErrIncompatible) {
		writeError(w, http.StatusUnprocessableEntity, "incompatible_protocol")
		return
	}
	if err != nil {
		s.internalError(w, "post seek", err)
		return
	}
	writeJSON(w, http.StatusOK, newSeekDTO(created))
}

func (s *Server) isService(r *http.Request) bool {
	token := bearerToken(r)
	return s.serviceKey != "" && token != "" &&
		subtle.ConstantTimeCompare([]byte(token), []byte(s.serviceKey)) == 1
}
