package life

import (
	"fmt"
	"math/rand/v2"
	"sort"
	"strconv"
	"strings"
)

type Point struct {
	X int `json:"x"`
	Y int `json:"y"`
}
type Cell struct {
	Age   uint32 `json:"age"`
	State uint8  `json:"state"`
}
type Rule struct {
	Birth, Survive [9]bool
	Text           string
}

func ParseRule(s string) (Rule, error) {
	r := Rule{Text: strings.ToUpper(strings.TrimSpace(s))}
	p := strings.Split(r.Text, "/")
	if len(p) != 2 {
		return r, fmt.Errorf("use a rule such as B3/S23 or B36/S23")
	}
	seen := map[byte]bool{}
	for _, part := range p {
		if len(part) == 0 || (part[0] != 'B' && part[0] != 'S') || seen[part[0]] {
			return r, fmt.Errorf("invalid B/S rule %q", s)
		}
		seen[part[0]] = true
		for _, c := range part[1:] {
			if c < '0' || c > '8' {
				return r, fmt.Errorf("neighbour counts must be 0–8")
			}
			if part[0] == 'B' {
				r.Birth[c-'0'] = true
			} else {
				r.Survive[c-'0'] = true
			}
		}
	}
	if r.Birth[0] {
		return r, fmt.Errorf("B0 requires an infinite background; choose a rule without B0")
	}
	return r, nil
}

var Modes = []string{"life", "brians-brain", "langtons-ant", "larger-than-life", "faders", "forest-fire", "wildfire", "belousov"}

type Metrics struct {
	Generation uint64 `json:"generation"`
	Population int    `json:"population"`
	Births     int    `json:"births"`
	Deaths     int    `json:"deaths"`
	Stable     bool   `json:"stable"`
	Chunks     int    `json:"chunks"`
	Cache      int    `json:"cache"`
}
type World struct {
	Cells         map[Point]Cell
	Trails        map[Point]uint8
	Rule          Rule
	Mode          string
	Wrap          bool
	Width, Height int
	ShowTrails    bool
	Metrics       Metrics
	Ant           Point
	Direction     int
	rng           *rand.PCG
	fast          *hashlife
}

func New(rule string, seed uint64) (*World, error) {
	r, err := ParseRule(rule)
	if err != nil {
		return nil, err
	}
	return &World{Cells: map[Point]Cell{}, Trails: map[Point]uint8{}, Rule: r, Mode: "life", Width: 160, Height: 100, rng: rand.NewPCG(seed, seed^0x9e3779b97f4a7c15)}, nil
}
func (w *World) Clone() *World {
	c := *w
	c.Cells = cloneMap(w.Cells)
	c.Trails = cloneMap(w.Trails)
	c.fast = nil
	if w.rng != nil {
		rng := *w.rng
		c.rng = &rng
	}
	return &c
}
func cloneMap[K comparable, V any](m map[K]V) map[K]V {
	n := make(map[K]V, len(m))
	for k, v := range m {
		n[k] = v
	}
	return n
}
func (w *World) randomFloat() float64 { return float64(w.rng.Uint64()>>11) * (1.0 / (1 << 53)) }
func mod(n, m int) int                { return (n%m + m) % m }
func (w *World) normalize(p Point) Point {
	if w.Wrap {
		p.X = mod(p.X+w.Width/2, w.Width) - w.Width/2
		p.Y = mod(p.Y+w.Height/2, w.Height) - w.Height/2
	}
	return p
}
func (w *World) Set(p Point, alive bool) {
	p = w.normalize(p)
	if alive {
		w.Cells[p] = Cell{Age: 1, State: 1}
	} else {
		delete(w.Cells, p)
	}
	w.fast = nil
	w.Recount()
}
func (w *World) Paint(a, b Point, alive bool) {
	dx, dy := abs(b.X-a.X), -abs(b.Y-a.Y)
	sx, sy := -1, -1
	if a.X < b.X {
		sx = 1
	}
	if a.Y < b.Y {
		sy = 1
	}
	e := dx + dy
	for {
		p := w.normalize(a)
		if alive {
			w.Cells[p] = Cell{Age: 1, State: 1}
		} else {
			delete(w.Cells, p)
		}
		if a == b {
			break
		}
		e2 := 2 * e
		if e2 >= dy {
			e += dy
			a.X += sx
		}
		if e2 <= dx {
			e += dx
			a.Y += sy
		}
	}
	w.fast = nil
	w.Recount()
}
func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
func (w *World) Clear() {
	w.Cells = map[Point]Cell{}
	w.Trails = map[Point]uint8{}
	w.Metrics = Metrics{}
	w.fast = nil
	w.Ant = Point{}
	w.Direction = 0
}
func (w *World) SetRule(s string) error {
	r, e := ParseRule(s)
	if e == nil {
		w.Rule = r
		w.fast = nil
	}
	return e
}
func (w *World) Random(seed uint64, density float64, x, y, width, height int) {
	w.Clear()
	w.rng = rand.NewPCG(seed, seed^0x9e3779b97f4a7c15)
	for yy := y; yy < y+height; yy++ {
		for xx := x; xx < x+width; xx++ {
			if w.randomFloat() < density {
				w.Cells[w.normalize(Point{xx, yy})] = Cell{Age: 1, State: 1}
			}
		}
	}
	w.Recount()
}
func (w *World) Recount() {
	chunks := map[Point]bool{}
	for p := range w.Cells {
		chunks[Point{p.X >> 6, p.Y >> 6}] = true
	}
	w.Metrics.Population = len(w.Cells)
	w.Metrics.Chunks = len(chunks)
	if w.fast != nil {
		w.Metrics.Cache = len(w.fast.memo)
	}
}
func (w *World) Bounds() (Point, Point, bool) {
	var a, b Point
	first := true
	for p := range w.Cells {
		if first {
			a = p
			b = p
			first = false
		}
		a.X = min(a.X, p.X)
		a.Y = min(a.Y, p.Y)
		b.X = max(b.X, p.X)
		b.Y = max(b.Y, p.Y)
	}
	return a, b, !first
}

func (w *World) Step() {
	if w.Mode == "langtons-ant" {
		w.stepAnt()
		return
	}
	counts := map[Point]int{}
	radius := 1
	if w.Mode == "larger-than-life" {
		radius = 2
	}
	for p, c := range w.Cells {
		if w.Mode == "brians-brain" && c.State != 1 {
			continue
		}
		for dy := -radius; dy <= radius; dy++ {
			for dx := -radius; dx <= radius; dx++ {
				if dx != 0 || dy != 0 {
					counts[w.normalize(Point{p.X + dx, p.Y + dy})]++
				}
			}
		}
	}
	next := map[Point]Cell{}
	for p := range w.Cells {
		counts[p] += 0
	}
	stochastic := w.Mode == "forest-fire" || w.Mode == "wildfire"
	if stochastic {
		// Regrowth can start in an empty forest, not just near existing trees.
		for y := -w.Height / 2; y < (w.Height+1)/2; y++ {
			for x := -w.Width / 2; x < (w.Width+1)/2; x++ {
				counts[Point{x, y}] += 0
			}
		}
	}
	candidates := make([]Point, 0, len(counts))
	for p := range counts {
		candidates = append(candidates, p)
	}
	if stochastic {
		// A stable iteration order makes seeded evolution and undo reproducible.
		sort.Slice(candidates, func(i, j int) bool {
			if candidates[i].Y == candidates[j].Y {
				return candidates[i].X < candidates[j].X
			}
			return candidates[i].Y < candidates[j].Y
		})
	}
	for _, p := range candidates {
		n := counts[p]
		if stochastic && !w.Wrap && (p.X < -w.Width/2 || p.X >= (w.Width+1)/2 || p.Y < -w.Height/2 || p.Y >= (w.Height+1)/2) {
			continue
		}
		old, alive := w.Cells[p]
		keep, born := false, false
		state := uint8(1)
		switch w.Mode {
		case "brians-brain":
			if alive && old.State == 1 {
				keep = true
				state = 2
			} else if !alive && n == 2 {
				born = true
			}
		case "larger-than-life":
			keep = alive && n >= 5 && n <= 11
			born = !alive && n >= 6 && n <= 9
		case "faders":
			if alive && old.State > 1 {
				if old.State < 8 {
					keep = true
					state = old.State + 1
				}
			} else {
				keep = alive && (n == 2 || n == 3)
				born = !alive && n == 3
				if alive && !keep {
					keep = true
					state = 2
				}
			}
		case "forest-fire", "wildfire":
			if alive && old.State == 2 {
				break
			}
			if alive {
				keep = true
				lightning := 0.00001
				if w.Mode == "wildfire" {
					lightning = 0.0002
				}
				if w.burningNeighbor(p) || w.randomFloat() < lightning {
					state = 2
				}
			} else if w.randomFloat() < 0.015 {
				born = true
			}
		case "belousov":
			if alive {
				keep = true
				state = old.State%12 + 1
				if state == 1 {
					keep = false
				}
			} else if w.hasStateNeighbor(p, 1) {
				born = true
			}
		default:
			keep = alive && w.Rule.Survive[n]
			born = !alive && w.Rule.Birth[n]
		}
		if keep || born {
			age := uint32(1)
			if alive {
				age = old.Age + 1
			}
			next[p] = Cell{age, state}
		}
	}
	w.commit(next, 1)
}
func (w *World) hasStateNeighbor(p Point, state uint8) bool {
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			if dx == 0 && dy == 0 {
				continue
			}
			if c, ok := w.Cells[w.normalize(Point{p.X + dx, p.Y + dy})]; ok && c.State == state {
				return true
			}
		}
	}
	return false
}
func (w *World) burningNeighbor(p Point) bool { return w.hasStateNeighbor(p, 2) }
func (w *World) stepAnt() {
	p := w.normalize(w.Ant)
	if _, ok := w.Cells[p]; ok {
		delete(w.Cells, p)
		w.Direction = (w.Direction + 3) % 4
	} else {
		w.Cells[p] = Cell{1, 1}
		w.Direction = (w.Direction + 1) % 4
	}
	d := []Point{{0, -1}, {1, 0}, {0, 1}, {-1, 0}}[w.Direction]
	w.Ant = w.normalize(Point{p.X + d.X, p.Y + d.Y})
	w.Metrics.Generation++
	w.Recount()
}
func (w *World) commit(next map[Point]Cell, steps uint64) {
	w.Metrics.Births = 0
	w.Metrics.Deaths = 0
	w.Metrics.Stable = true
	for p := range next {
		if _, ok := w.Cells[p]; !ok {
			w.Metrics.Births++
			w.Metrics.Stable = false
		} else if next[p].State != w.Cells[p].State {
			w.Metrics.Stable = false
		}
	}
	for p := range w.Cells {
		if _, ok := next[p]; !ok {
			w.Metrics.Deaths++
			w.Metrics.Stable = false
			if w.ShowTrails {
				w.Trails[p] = 16
			}
		}
	}
	for p, t := range w.Trails {
		if t <= 1 {
			delete(w.Trails, p)
		} else {
			w.Trails[p] = t - 1
		}
	}
	w.Cells = next
	w.Metrics.Generation += steps
	w.Recount()
}
func (w *World) Advance(n uint64) error {
	if n > 1<<20 {
		return fmt.Errorf("advance at most 1048576 generations at once")
	}
	if n < 16 || w.Mode != "life" || w.Wrap || w.ShowTrails {
		if n > 4096 {
			return fmt.Errorf("use Life mode without wrapping or trails for jumps above 4096")
		}
		for range n {
			w.Step()
		}
		return nil
	}
	if w.fast == nil {
		w.fast = newHashlife(w.Rule)
	}
	next, err := w.fast.advance(w.Cells, n)
	if err != nil {
		return err
	}
	w.commit(next, n)
	return nil
}
func (w *World) SortedPoints() []Point {
	p := make([]Point, 0, len(w.Cells))
	for k := range w.Cells {
		p = append(p, k)
	}
	sort.Slice(p, func(i, j int) bool {
		if p[i].Y == p[j].Y {
			return p[i].X < p[j].X
		}
		return p[i].Y < p[j].Y
	})
	return p
}
func PointKey(p Point) string { return strconv.Itoa(p.X) + "," + strconv.Itoa(p.Y) }
