package life

import (
	"fmt"
	"math/bits"
)

// Canonical quadtrees share identical regions; memoized successors skip repeated
// space-time evolution. Each successor returns the central half of its input.
type node struct {
	id, level, pop int
	nw, ne, sw, se *node
}
type quad [4]*node
type successorKey struct {
	n     *node
	power int
}
type hashlife struct {
	rule       Rule
	dead, live *node
	intern     map[quad]*node
	memo       map[successorKey]*node
	empties    []*node
	serial     int
}

func newHashlife(r Rule) *hashlife {
	h := &hashlife{rule: r, intern: map[quad]*node{}, memo: map[successorKey]*node{}}
	h.dead = &node{id: 1}
	h.live = &node{id: 2, pop: 1}
	h.serial = 2
	h.empties = []*node{h.dead}
	return h
}
func (h *hashlife) join(a, b, c, d *node) *node {
	k := quad{a, b, c, d}
	if n := h.intern[k]; n != nil {
		return n
	}
	h.serial++
	n := &node{id: h.serial, level: a.level + 1, pop: a.pop + b.pop + c.pop + d.pop, nw: a, ne: b, sw: c, se: d}
	h.intern[k] = n
	return n
}
func (h *hashlife) empty(level int) *node {
	for len(h.empties) <= level {
		n := h.empties[len(h.empties)-1]
		h.empties = append(h.empties, h.join(n, n, n, n))
	}
	return h.empties[level]
}
func (h *hashlife) set(n *node, x, y int) *node {
	if n.level == 0 {
		return h.live
	}
	half := 1 << (n.level - 1)
	a, b, c, d := n.nw, n.ne, n.sw, n.se
	if y < half {
		if x < half {
			a = h.set(a, x, y)
		} else {
			b = h.set(b, x-half, y)
		}
	} else {
		if x < half {
			c = h.set(c, x, y-half)
		} else {
			d = h.set(d, x-half, y-half)
		}
	}
	return h.join(a, b, c, d)
}
func (h *hashlife) get(n *node, x, y int) bool {
	if n.pop == 0 {
		return false
	}
	if n.level == 0 {
		return true
	}
	half := 1 << (n.level - 1)
	if y < half {
		if x < half {
			return h.get(n.nw, x, y)
		}
		return h.get(n.ne, x-half, y)
	}
	if x < half {
		return h.get(n.sw, x, y-half)
	}
	return h.get(n.se, x-half, y-half)
}
func (h *hashlife) result(n *node, power int) *node {
	if n.pop == 0 {
		return h.empty(n.level - 1)
	}
	k := successorKey{n, power}
	if v := h.memo[k]; v != nil {
		return v
	}
	var out *node
	if n.level == 2 {
		v := [4]*node{}
		i := 0
		for y := 1; y <= 2; y++ {
			for x := 1; x <= 2; x++ {
				count := 0
				for dy := -1; dy <= 1; dy++ {
					for dx := -1; dx <= 1; dx++ {
						if (dx != 0 || dy != 0) && h.get(n, x+dx, y+dy) {
							count++
						}
					}
				}
				alive := h.get(n, x, y)
				v[i] = h.dead
				if alive && h.rule.Survive[count] || !alive && h.rule.Birth[count] {
					v[i] = h.live
				}
				i++
			}
		}
		out = h.join(v[0], v[1], v[2], v[3])
	} else {
		a, b, c, d := n.nw, n.ne, n.sw, n.se
		overlap := [9]*node{a, h.join(a.ne, b.nw, a.se, b.sw), b, h.join(a.sw, a.se, c.nw, c.ne), h.join(a.se, b.sw, c.ne, d.nw), h.join(b.sw, b.se, d.nw, d.ne), c, h.join(c.ne, d.nw, c.se, d.sw), d}
		v := [9]*node{}
		for i, s := range overlap {
			v[i] = h.result(s, min(power, n.level-3))
		}
		if power == n.level-2 {
			out = h.join(h.result(h.join(v[0], v[1], v[3], v[4]), power-1), h.result(h.join(v[1], v[2], v[4], v[5]), power-1), h.result(h.join(v[3], v[4], v[6], v[7]), power-1), h.result(h.join(v[4], v[5], v[7], v[8]), power-1))
		} else {
			out = h.join(h.join(v[0].se, v[1].sw, v[3].ne, v[4].nw), h.join(v[1].se, v[2].sw, v[4].ne, v[5].nw), h.join(v[3].se, v[4].sw, v[6].ne, v[7].nw), h.join(v[4].se, v[5].sw, v[7].ne, v[8].nw))
		}
	}
	h.memo[k] = out
	return out
}
func (h *hashlife) collect(n *node, x, y int, out map[Point]Cell) {
	if n.pop == 0 {
		return
	}
	if n.level == 0 {
		out[Point{x, y}] = Cell{1, 1}
		return
	}
	half := 1 << (n.level - 1)
	h.collect(n.nw, x, y, out)
	h.collect(n.ne, x+half, y, out)
	h.collect(n.sw, x, y+half, out)
	h.collect(n.se, x+half, y+half, out)
}
func (h *hashlife) advance(cells map[Point]Cell, steps uint64) (map[Point]Cell, error) {
	if len(cells) == 0 {
		return map[Point]Cell{}, nil
	}
	// A cache budget bounds memory while retaining reuse across nearby jumps.
	if len(h.memo) > 200000 {
		fresh := newHashlife(h.rule)
		*h = *fresh
	}
	current := cells
	for steps > 0 {
		power := bits.Len64(steps) - 1
		jump := uint64(1) << power
		minX, minY, maxX, maxY := 0, 0, 0, 0
		first := true
		for p := range current {
			if first {
				minX, maxX, minY, maxY = p.X, p.X, p.Y, p.Y
				first = false
			}
			minX = min(minX, p.X)
			maxX = max(maxX, p.X)
			minY = min(minY, p.Y)
			maxY = max(maxY, p.Y)
		}
		span := max(maxX-minX+1, maxY-minY+1)
		required := 2*span + 4*int(jump) + 8
		level := max(3, bits.Len(uint(required-1)))
		if level > 40 {
			return nil, fmt.Errorf("pattern extent is too large")
		}
		size := 1 << level
		ox := (minX+maxX)/2 - size/2
		oy := (minY+maxY)/2 - size/2
		n := h.empty(level)
		for p := range current {
			n = h.set(n, p.X-ox, p.Y-oy)
		}
		n = h.result(n, power)
		next := map[Point]Cell{}
		h.collect(n, ox+size/4, oy+size/4, next)
		current = next
		steps -= jump
	}
	return current, nil
}
