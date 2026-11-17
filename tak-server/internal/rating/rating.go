// Package rating computes player ratings from finished games. It is a port of
// the NestJS rating job (api/src/modules/ratings/ratings.service.ts), itself
// based on Nohatcoder's Elo system (https://nohatcoder.dk/2015-03-22-1.html),
// and must produce the same numbers: the parity tests in this package check
// it against output captured from the Node job.
package rating

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"
)

const (
	initialRating       = 1000
	bonusRating         = 750
	bonusFactor         = 60
	participationLimit  = 10
	participationCutoff = 1500
	maxDrop             = 200
	ratingRetention     = 1000 * 60 * 60 * 24 * 240 // ms
	recentWindow        = 1000 * 60 * 60 * 6        // ms

	// firstNamedGame is when playtak switched from anonymous to named
	// accounts. Earlier games are never rated.
	firstNamedGame = 1461430800000
	// batchSize caps the games read in one run.
	batchSize = 50000
	// unratedChange marks a game's rating change columns as "not rated".
	unratedChange = -2000
)

// Player holds a player's rating state.
type Player struct {
	ID         int
	Name       string
	Rating     float64
	Boost      float64
	MaxRating  float64
	RatingAge  float64 // ms timestamp
	RatedGames int
	Unrated    bool
	IsBot      bool
	// Fatigue maps an opponent's ID (decimal string) to how much recent play
	// against them dampens rating changes.
	Fatigue map[string]float64

	changed bool
}

// Game is the subset of a games row the rating needs.
type Game struct {
	ID             int
	Date           int64 // ms timestamp
	White, Black   string
	Result         string
	Unrated        int
	Size           int
	TimerTime      int
	TimerInc       int
	Pieces         int
	Capstones      int
	NotationLength int
}

// GameUpdate is what gets written back to a games row.
type GameUpdate struct {
	ID                       int
	RatingWhite, RatingBlack int
	ChangeWhite, ChangeBlack int
}

var whiteScore = map[string]float64{
	"R-0": 1, "F-0": 1, "1-0": 1,
	"0-R": 0, "0-F": 0, "0-1": 0,
	"1/2-1/2": 0.5,
}

// rateGames applies games, in ID order, to the players in byName and returns
// the rows to write and the new cursor (the last game ID that never needs
// looking at again). It stops advancing the cursor at the first eligible
// game that may still be in progress, so the next run picks it up.
func rateGames(byName map[string]*Player, games []Game, cursor int, now int64) ([]GameUpdate, int) {
	recentLimit := now - recentWindow
	updating := true
	var updates []GameUpdate
	write := func(id int, white, black float64, changeWhite, changeBlack int) {
		updates = append(updates, GameUpdate{id, int(math.Floor(white)), int(math.Floor(black)), changeWhite, changeBlack})
	}

	for _, g := range games {
		white, black := byName[g.White], byName[g.Black]
		if white != nil && white.Unrated {
			white = nil
		}
		if black != nil && black.Unrated {
			black = nil
		}
		var whiteRating, blackRating, whiteAdjusted, blackAdjusted float64
		if white != nil {
			whiteRating = white.Rating
			whiteAdjusted = adjustedRating(white, g.Date)
		}
		if black != nil {
			blackRating = black.Rating
			blackAdjusted = adjustedRating(black, g.Date)
		}
		score, decided := whiteScore[g.Result]

		if white == nil || black == nil || white == black || g.Unrated != 0 || !eligible(g) {
			if updating {
				cursor = g.ID
			}
			write(g.ID, whiteAdjusted, blackAdjusted, unratedChange, unratedChange)
			continue
		}
		if !updating {
			continue
		}
		// A game with this little notation, no result and a recent start is
		// probably still being played: stop here and retry it next run.
		if g.NotationLength <= 6 && !decided && g.Date > recentLimit {
			updating = false
			continue
		}
		cursor = g.ID
		if g.NotationLength <= 6 || !decided {
			write(g.ID, whiteAdjusted, blackAdjusted, unratedChange, unratedChange)
			continue
		}

		sw := math.Pow(10, whiteRating/400)
		sb := math.Pow(10, blackRating/400)
		expected := sw / (sw + sb)
		fairness := expected * (1 - expected)
		blackID, whiteID := strconv.Itoa(black.ID), strconv.Itoa(white.ID)
		fatigueFactor := (1 - white.Fatigue[blackID]*0.4) * (1 - black.Fatigue[whiteID]*0.4)
		adjustPlayer(white, score-expected, fairness, fatigueFactor, g.Date)
		adjustPlayer(black, expected-score, fairness, fatigueFactor, g.Date)
		updateFatigue(white, blackID, fairness*fatigueFactor)
		updateFatigue(black, whiteID, fairness*fatigueFactor)
		write(g.ID, whiteAdjusted, blackAdjusted,
			jsRound((adjustedRating(white, g.Date)-whiteAdjusted)*10),
			jsRound((adjustedRating(black, g.Date)-blackAdjusted)*10))
	}
	return updates, cursor
}

func adjustPlayer(p *Player, amount, fairness, fatigueFactor float64, date int64) {
	bonus := math.Min(math.Max(0, fatigueFactor*amount*math.Max(p.Boost, 1)*bonusFactor/bonusRating), p.Boost)
	p.Boost -= bonus
	k := 10 +
		15*math.Pow(0.5, float64(p.RatedGames)/200) +
		15*math.Pow(0.5, (p.MaxRating-initialRating)/300)
	p.Rating += fatigueFactor*amount*k + bonus
	d := float64(date)
	if p.RatingAge == 0 {
		p.RatingAge = d - ratingRetention
	}
	participation := 20 * math.Pow(0.5, (d-p.RatingAge)/ratingRetention)
	participation += fairness * fatigueFactor
	participation = math.Min(20, participation)
	p.RatingAge = math.Log2(participation/20)*ratingRetention + d
	p.RatedGames++
	p.MaxRating = math.Max(p.MaxRating, p.Rating)
	p.changed = true
}

func updateFatigue(p *Player, opponentID string, gameFactor float64) {
	multiplier := 1 - gameFactor*0.4
	for id := range p.Fatigue {
		p.Fatigue[id] *= multiplier
		if id != opponentID && p.Fatigue[id] < 0.01 {
			delete(p.Fatigue, id)
		}
	}
	p.Fatigue[opponentID] += gameFactor
}

// adjustedRating is the rating shown for p at time date: ratings of 1500 or
// more drop by up to 200 when the player hasn't played much recently.
func adjustedRating(p *Player, date int64) float64 {
	if p.Rating < participationCutoff {
		return p.Rating
	}
	participation := 20 * math.Pow(0.5, (float64(date)-p.RatingAge)/ratingRetention)
	if p.Rating < participationCutoff+maxDrop {
		return math.Min(p.Rating, participationCutoff+maxDrop*participation/participationLimit)
	}
	return math.Min(p.Rating, p.Rating-maxDrop*(1-participation/participationLimit))
}

// sizeLimits are, per board size: minimum time (s), min and max flats, min
// and max capstones for a rated game. Sizes without an entry use size 5's.
var sizeLimits = map[int][5]int{
	5: {180, 20, 32, 1, 1},
	6: {240, 25, 40, 1, 2},
	7: {300, 30, 48, 1, 2},
	8: {360, 40, 64, 1, 3},
}

func eligible(g Game) bool {
	if g.Size < 5 {
		return false
	}
	limits, ok := sizeLimits[g.Size]
	if !ok {
		limits = sizeLimits[5]
	}
	if g.Pieces != -1 && (g.Pieces < limits[1] || g.Pieces > limits[2] || g.Capstones < limits[3] || g.Capstones > limits[4]) {
		return false
	}
	if g.TimerTime > 0 && (limits[0]*3 > g.TimerTime*3+g.TimerInc*limits[0] || g.TimerTime < 60) {
		return false
	}
	return true
}

// jsRound is JavaScript's Math.round: halves round up, toward +Inf. Go's
// math.Round rounds them away from zero, so -2.5 would give -3, not -2.
func jsRound(x float64) int {
	r := math.Floor(x)
	if x-r >= 0.5 {
		r++
	}
	return int(r)
}

// parseFatigue reads a fatigue column. The Node job wrote it JSON-encoded
// twice ("{\"85\":0.2}"), so it strips backslashes and one pair of outer
// quotes before parsing, as Node's parseFatigue did. Unreadable values give
// an empty map; Node crashed on them mid-run instead.
func parseFatigue(raw string) map[string]float64 {
	s := strings.ReplaceAll(raw, `\`, "")
	s = strings.TrimPrefix(s, `"`)
	s = strings.TrimSuffix(s, `"`)
	fatigue := map[string]float64{}
	if json.Unmarshal([]byte(s), &fatigue) != nil || fatigue == nil {
		return map[string]float64{}
	}
	return fatigue
}
