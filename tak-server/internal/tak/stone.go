// Package tak implements the rules of Tak: the board, placing and moving
// stones, and deciding results from the position. It has no clocks, players
// or I/O; package game builds live games on top of it.
package tak

import (
	"fmt"
	"strings"
)

type Color int8

const (
	White Color = iota
	Black
)

func (c Color) Opponent() Color { return 1 - c }

func (c Color) String() string {
	if c == White {
		return "white"
	}
	return "black"
}

type Kind int8

const (
	Flat Kind = iota
	Wall
	Capstone
)

// Stone is one piece on the board.
type Stone struct {
	Color Color
	Kind  Kind
}

// Square is a board coordinate. File 0 is column A and Rank 0 is row 1.
type Square struct {
	File, Rank int
}

// ParseSquare parses a coordinate such as "A1" or "c5" (case-insensitive).
// It doesn't check the coordinate is on a particular board.
func ParseSquare(s string) (Square, bool) {
	if len(s) != 2 {
		return Square{}, false
	}
	file := strings.ToUpper(s)[0]
	rank := s[1]
	if file < 'A' || file > 'Z' || rank < '0' || rank > '9' {
		return Square{}, false
	}
	return Square{File: int(file - 'A'), Rank: int(rank-'0') - 1}, true
}

// String formats the square as the protocol does, e.g. "A1".
func (s Square) String() string {
	return fmt.Sprintf("%c%d", 'A'+rune(s.File), s.Rank+1)
}

// letter is the stone's character in the protocol's square display: F, W, C
// for white and f, w, c for black.
func (s Stone) letter() byte {
	c := [...]byte{'f', 'w', 'c'}[s.Kind]
	if s.Color == White {
		c -= 'a' - 'A'
	}
	return c
}
