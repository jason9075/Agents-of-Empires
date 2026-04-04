package hex

import "testing"

func TestDistance(t *testing.T) {
	cases := []struct {
		a, b Coord
		want int
	}{
		{Coord{0, 0}, Coord{0, 0}, 0},
		{Coord{0, 0}, Coord{1, 0}, 1},
		{Coord{0, 0}, Coord{3, 0}, 3},
		{Coord{0, 0}, Coord{0, 3}, 3},
		{Coord{5, 5}, Coord{5, 5}, 0},
		{Coord{0, 0}, Coord{2, -2}, 3},
		{Coord{5, 4}, Coord{8, 8}, 5},
	}
	for _, c := range cases {
		if got := Distance(c.a, c.b); got != c.want {
			t.Errorf("Distance(%v, %v) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestNeighbors(t *testing.T) {
	cases := []struct {
		center Coord
		want   [6]Coord
	}{
		{
			center: Coord{5, 4},
			want: [6]Coord{
				{6, 4}, {5, 3}, {4, 3},
				{4, 4}, {4, 5}, {5, 5},
			},
		},
		{
			center: Coord{5, 5},
			want: [6]Coord{
				{6, 5}, {6, 4}, {5, 4},
				{4, 5}, {5, 6}, {6, 6},
			},
		},
	}

	for _, tc := range cases {
		neighbors := tc.center.Neighbors()
		if neighbors != tc.want {
			t.Fatalf("Neighbors(%v) = %v, want %v", tc.center, neighbors, tc.want)
		}
		for _, n := range neighbors {
			if Distance(tc.center, n) != 1 {
				t.Errorf("neighbor %v is not distance 1 from %v", n, tc.center)
			}
		}
	}
}

func TestInBounds(t *testing.T) {
	cases := []struct {
		c    Coord
		want bool
	}{
		{Coord{0, 0}, true},
		{Coord{19, 14}, true},
		{Coord{10, 10}, true},
		{Coord{-1, 0}, false},
		{Coord{0, -1}, false},
		{Coord{20, 0}, false},
		{Coord{0, 15}, false},
	}
	for _, c := range cases {
		if got := InBounds(c.c); got != c.want {
			t.Errorf("InBounds(%v) = %v, want %v", c.c, got, c.want)
		}
	}
}

func TestRing(t *testing.T) {
	center := Coord{10, 10}
	for r := 1; r <= 5; r++ {
		ring := Ring(center, r)
		// All tiles in the ring must be exactly r away.
		for _, c := range ring {
			if d := Distance(center, c); d != r {
				t.Errorf("Ring(%v, %d) contains %v at distance %d", center, r, c, d)
			}
		}
	}
}

func TestCircle(t *testing.T) {
	center := Coord{10, 10}
	circle := Circle(center, 3)
	for _, c := range circle {
		if Distance(center, c) > 3 {
			t.Errorf("Circle(%v, 3) contains %v at distance %d", center, c, Distance(center, c))
		}
	}
}
