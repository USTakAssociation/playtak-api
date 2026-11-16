package tak

import (
	"fmt"
	"strings"
)

// TPS returns the position in Tak Positional System notation: rows from the
// top rank down, stacks bottom first as 1 (white) and 2 (black) with S or C
// for a wall or capstone on top, runs of empty squares as x or xN, then the
// color to move (1 or 2) and the move number.
//
//	x3,12,2S/x,22S,22C,11,21/x5/x5/1,x4 1 6
func (g *Game) TPS() string {
	rows := make([]string, 0, g.size)
	for rank := g.size - 1; rank >= 0; rank-- {
		var cells []string
		empty := 0
		flush := func() {
			switch {
			case empty == 1:
				cells = append(cells, "x")
			case empty > 1:
				cells = append(cells, fmt.Sprintf("x%d", empty))
			}
			empty = 0
		}
		for f := range g.size {
			stack := g.position.stacks[g.index(Square{File: f, Rank: rank})]
			if len(stack) == 0 {
				empty++
				continue
			}
			flush()
			var builder strings.Builder
			for _, st := range stack {
				builder.WriteByte("12"[st.Color])
			}
			switch stack[len(stack)-1].Kind {
			case Wall:
				builder.WriteByte('S')
			case Capstone:
				builder.WriteByte('C')
			}
			cells = append(cells, builder.String())
		}
		flush()
		rows = append(rows, strings.Join(cells, ","))
	}
	return fmt.Sprintf("%s %d %d", strings.Join(rows, "/"), g.Turn()+1, g.position.ply/2+1)
}
