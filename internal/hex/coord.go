package hex

// Coord is an axial hex coordinate. The grid is 20×15 with 0 ≤ Q < 20, 0 ≤ R < 15.
type Coord struct {
	Q int `json:"q"`
	R int `json:"r"`
}

const (
	GridWidth  = 20
	GridHeight = 15
)

// directions are the six axial neighbour offsets.
var directions = [6]Coord{
	{1, 0}, {1, -1}, {0, -1},
	{-1, 0}, {-1, 1}, {0, 1},
}

// Neighbors returns the six adjacent hex coords (may be out of bounds).
func (c Coord) Neighbors() [6]Coord {
	var out [6]Coord
	for i, d := range directions {
		out[i] = Coord{c.Q + d.Q, c.R + d.R}
	}
	return out
}

// InBounds reports whether c lies within the 20×15 grid.
func InBounds(c Coord) bool {
	return c.Q >= 0 && c.Q < GridWidth && c.R >= 0 && c.R < GridHeight
}

// Distance returns the hex grid distance between a and b.
func Distance(a, b Coord) int {
	dq := a.Q - b.Q
	dr := a.R - b.R
	ds := (-a.Q - a.R) - (-b.Q - b.R)
	return max3(abs(dq), abs(dr), abs(ds))
}

// Ring returns all in-bounds coords exactly radius steps from center.
// Returns empty slice for radius <= 0.
func Ring(center Coord, radius int) []Coord {
	if radius <= 0 {
		return nil
	}
	// Start at the "bottom-left" corner of the ring.
	cur := Coord{center.Q + directions[4].Q*radius, center.R + directions[4].R*radius}
	var out []Coord
	for i := 0; i < 6; i++ {
		for j := 0; j < radius; j++ {
			if InBounds(cur) {
				out = append(out, cur)
			}
			cur = Coord{cur.Q + directions[i].Q, cur.R + directions[i].R}
		}
	}
	return out
}

// Circle returns all in-bounds coords within radius steps of center (inclusive).
func Circle(center Coord, radius int) []Coord {
	var out []Coord
	if InBounds(center) {
		out = append(out, center)
	}
	for r := 1; r <= radius; r++ {
		out = append(out, Ring(center, r)...)
	}
	return out
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func max3(a, b, c int) int {
	if a >= b && a >= c {
		return a
	}
	if b >= c {
		return b
	}
	return c
}
