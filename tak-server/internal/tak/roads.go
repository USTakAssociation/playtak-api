package tak

// hasRoad reports whether c has a road: an orthogonally connected chain of
// c's flats and capstones (walls don't count) joining opposite edges.
func (g *Game) hasRoad(c Color) bool {
	return g.connects(c, func(square Square) bool { return square.File == 0 }, func(square Square) bool { return square.File == g.size-1 }) ||
		g.connects(c, func(square Square) bool { return square.Rank == 0 }, func(square Square) bool { return square.Rank == g.size-1 })
}

// connects runs a breadth-first search from every road square on the start
// edge and reports whether any reaches the end edge.
func (g *Game) connects(color Color, start, end func(Square) bool) bool {
	seen := make([]bool, g.size*g.size)
	var queue []Square
	for r := range g.size {
		for f := range g.size {
			square := Square{File: f, Rank: r}
			if start(square) && g.isRoad(square, color) {
				seen[g.index(square)] = true
				queue = append(queue, square)
			}
		}
	}
	for len(queue) > 0 {
		square := queue[0]
		queue = queue[1:]
		if end(square) {
			return true
		}
		for _, d := range [...]Square{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			n := Square{File: square.File + d.File, Rank: square.Rank + d.Rank}
			if g.onBoard(n) && !seen[g.index(n)] && g.isRoad(n, color) {
				seen[g.index(n)] = true
				queue = append(queue, n)
			}
		}
	}
	return false
}

func (g *Game) isRoad(square Square, c Color) bool {
	s := g.position.stacks[g.index(square)]
	if len(s) == 0 {
		return false
	}
	top := s[len(s)-1]
	return top.Color == c && top.Kind != Wall
}
