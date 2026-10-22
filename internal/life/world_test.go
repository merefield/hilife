package life

import (
	"reflect"
	"testing"
)

func world(t *testing.T, name string) *World {
	t.Helper()
	w, e := New("B3/S23", 1)
	if e != nil {
		t.Fatal(e)
	}
	if e = w.LoadPreset(name); e != nil {
		t.Fatal(e)
	}
	return w
}
func TestBlinkerAndUndoSnapshot(t *testing.T) {
	w := world(t, "blinker")
	start := w.Clone()
	w.Step()
	if len(w.Cells) != 3 {
		t.Fatal(w.Cells)
	}
	w.Step()
	if !reflect.DeepEqual(w.SortedPoints(), start.SortedPoints()) {
		t.Fatal("blinker did not return")
	}
	if start.Metrics.Generation != 0 {
		t.Fatal("snapshot mutated")
	}
}
func TestHashlifeMatchesSparse(t *testing.T) {
	for _, name := range []string{"glider", "gosper", "acorn", "replicator", "pulsar"} {
		for _, steps := range []uint64{16, 31, 64, 129} {
			t.Run(name, func(t *testing.T) {
				a := world(t, name)
				b := a.Clone()
				if err := a.Advance(steps); err != nil {
					t.Fatal(err)
				}
				for range steps {
					b.Step()
				}
				if !reflect.DeepEqual(a.SortedPoints(), b.SortedPoints()) {
					t.Fatalf("%s after %d: fast=%d sparse=%d", name, steps, len(a.Cells), len(b.Cells))
				}
			})
		}
	}
}
func TestHashlifeRandomAndNegativeCoordinates(t *testing.T) {
	a, _ := New("B3/S23", 7)
	a.Random(7, .28, -19, -17, 37, 31)
	b := a.Clone()
	if err := a.Advance(97); err != nil {
		t.Fatal(err)
	}
	for range 97 {
		b.Step()
	}
	if !reflect.DeepEqual(a.SortedPoints(), b.SortedPoints()) {
		t.Fatal("random accelerated world differs")
	}
}
func TestMillionGenerationGlider(t *testing.T) {
	a := world(t, "glider")
	if err := a.Advance(1 << 20); err != nil {
		t.Fatal(err)
	}
	b := world(t, "glider")
	want := map[Point]bool{}
	for p := range b.Cells {
		want[Point{p.X + (1 << 18), p.Y + (1 << 18)}] = true
	}
	for p := range a.Cells {
		if !want[p] {
			t.Fatal(p)
		}
	}
	if len(a.Cells) != 5 {
		t.Fatal(len(a.Cells))
	}
}
func TestPatternRoundTrips(t *testing.T) {
	for _, name := range []string{"gosper", "pulsar", "replicator"} {
		a := world(t, name)
		text, e := a.ExportRLE()
		if e != nil {
			t.Fatal(e)
		}
		b, _ := New("B3/S23", 1)
		if e = b.LoadText(text); e != nil {
			t.Fatal(e)
		}
		if !reflect.DeepEqual(a.SortedPoints(), b.SortedPoints()) {
			t.Fatalf("RLE roundtrip %s", name)
		}
		if e = b.LoadText(a.ExportJSON()); e != nil {
			t.Fatal(e)
		}
		if a.Rule.Text != b.Rule.Text || !reflect.DeepEqual(a.SortedPoints(), b.SortedPoints()) {
			t.Fatal("JSON roundtrip")
		}
	}
}
func TestInvalidPatterns(t *testing.T) {
	for _, text := range []string{"x = 2, y = 2\n3o", "x = 2, y = 2\n999999999o!", "x = 1, y = 1, rule = B0/S23\no!", "garbage"} {
		w := world(t, "glider")
		before := w.ExportJSON()
		if w.LoadText(text) == nil {
			t.Fatalf("accepted %q", text)
		}
		if w.ExportJSON() != before {
			t.Fatal("failed import mutated board")
		}
	}
}
func TestWrapping(t *testing.T) {
	w, _ := New("B3/S23", 1)
	w.Wrap = true
	w.Width = 8
	w.Height = 8
	w.Set(Point{4, 0}, true)
	if _, ok := w.Cells[Point{-4, 0}]; !ok {
		t.Fatal("edge did not wrap")
	}
}

func TestPulsarPeriodAndPlaintextComments(t *testing.T) {
	w := world(t, "pulsar")
	start := w.SortedPoints()
	if len(start) != 48 {
		t.Fatalf("pulsar has %d cells", len(start))
	}
	for range 3 {
		w.Step()
	}
	if !reflect.DeepEqual(start, w.SortedPoints()) {
		t.Fatal("pulsar is not period 3")
	}
	if err := w.LoadText("!Name: glider\n!A plaintext pattern\n.O.\n..O\nOOO\n"); err != nil {
		t.Fatal(err)
	}
	if len(w.Cells) != 5 {
		t.Fatal("plaintext comments were treated as RLE")
	}
}
func TestBrianRefractoryAndAnt(t *testing.T) {
	w := world(t, "block")
	w.Mode = "brians-brain"
	w.Step()
	if w.Cells[Point{0, 0}].State != 2 {
		t.Fatal("missing refractory state")
	}
	w = world(t, "empty")
	w.Mode = "langtons-ant"
	w.Step()
	if len(w.Cells) != 1 || w.Ant != (Point{1, 0}) {
		t.Fatal("ant turn")
	}
}
