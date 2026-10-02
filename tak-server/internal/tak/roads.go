package tak

// hasRoad reports whether c has a road: an orthogonally connected chain of
// c's flats and capstones (walls don't count) joining opposite edges.
func (g *Game) hasRoad(c Color) bool {
	return g.connects(c, func(sq Square) bool { return sq.File == 0 }, func(sq Square) bool { return sq.File == g.size-1 }) ||
		g.connects(c, func(sq Square) bool { return sq.Rank == 0 }, func(sq Square) bool { return sq.Rank == g.size-1 })
}

// connects runs a breadth-first search from every road square on the start
// edge and reports whether any reaches the end edge.
func (g *Game) connects(c Color, start, end func(Square) bool) bool {
	seen := make([]bool, g.size*g.size)
	var queue []Square
	for r := range g.size {
		for f := range g.size {
			sq := Square{File: f, Rank: r}
			if start(sq) && g.isRoad(sq, c) {
				seen[g.index(sq)] = true
				queue = append(queue, sq)
			}
		}
	}
	for len(queue) > 0 {
		sq := queue[0]
		queue = queue[1:]
		if end(sq) {
			return true
		}
		for _, d := range [...]Square{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			n := Square{File: sq.File + d.File, Rank: sq.Rank + d.Rank}
			if g.onBoard(n) && !seen[g.index(n)] && g.isRoad(n, c) {
				seen[g.index(n)] = true
				queue = append(queue, n)
			}
		}
	}
	return false
}

func (g *Game) isRoad(sq Square, c Color) bool {
	s := g.pos.stacks[g.index(sq)]
	if len(s) == 0 {
		return false
	}
	top := s[len(s)-1]
	return top.Color == c && top.Kind != Wall
}
