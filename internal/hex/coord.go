package hex

import "sort"

// Coord is an odd-r offset hex coordinate. The grid is 20×15 with 0 ≤ Q < 20, 0 ≤ R < 15.
type Coord struct {
	Q int `json:"q"`
	R int `json:"r"`
}

const (
	GridWidth  = 20
	GridHeight = 15
)

// odd-r pointy-top neighbour offsets, keyed by row parity.
var directions = [2][6]Coord{
	{
		{1, 0}, {0, -1}, {-1, -1},
		{-1, 0}, {-1, 1}, {0, 1},
	},
	{
		{1, 0}, {1, -1}, {0, -1},
		{-1, 0}, {0, 1}, {1, 1},
	},
}

// Neighbors returns the six adjacent hex coords (may be out of bounds).
func (c Coord) Neighbors() [6]Coord {
	var out [6]Coord
	for i, d := range directions[c.R&1] {
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
	ax, ay, az := a.cube()
	bx, by, bz := b.cube()
	return max3(abs(ax-bx), abs(ay-by), abs(az-bz))
}

// Ring returns all in-bounds coords exactly radius steps from center.
// Returns empty slice for radius <= 0.
func Ring(center Coord, radius int) []Coord {
	if radius <= 0 {
		return nil
	}
	var out []Coord
	for r := 0; r < GridHeight; r++ {
		for q := 0; q < GridWidth; q++ {
			c := Coord{Q: q, R: r}
			if Distance(center, c) == radius {
				out = append(out, c)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].R != out[j].R {
			return out[i].R < out[j].R
		}
		return out[i].Q < out[j].Q
	})
	return out
}

// Circle returns all in-bounds coords within radius steps of center (inclusive).
func Circle(center Coord, radius int) []Coord {
	var out []Coord
	for r := 0; r < GridHeight; r++ {
		for q := 0; q < GridWidth; q++ {
			c := Coord{Q: q, R: r}
			if Distance(center, c) <= radius {
				out = append(out, c)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		di := Distance(center, out[i])
		dj := Distance(center, out[j])
		if di != dj {
			return di < dj
		}
		if out[i].R != out[j].R {
			return out[i].R < out[j].R
		}
		return out[i].Q < out[j].Q
	})
	return out
}

func (c Coord) cube() (x, y, z int) {
	x = c.Q - (c.R-(c.R&1))/2
	z = c.R
	y = -x - z
	return
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
