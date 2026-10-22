package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"hilife/internal/app"
	"hilife/internal/life"
)

func TestBrailleViewUsesAllTerminalRows(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {120, 32}} {
		s, _ := app.New(1)
		s.World.Clear()
		m := New(s, "braille")
		m.width, m.height = size[0], size[1]
		cols, rows, top := m.boardSize()
		s.Camera = app.Camera{X: float64(cols), Y: float64(rows * 2), Zoom: 1}
		for y := 0; y < rows*4; y++ {
			for x := 0; x < cols*2; x++ {
				s.World.Cells[life.Point{X: x, Y: y}] = life.Cell{Age: 1, State: 1}
			}
		}
		lines := strings.Split(ansi.Strip(m.View().Content), "\n")
		if len(lines) != m.height {
			t.Fatalf("%dx%d: rendered %d rows, leaving %d unused", m.width, m.height, len(lines), m.height-len(lines))
		}
		for y := top; y < top+rows; y++ {
			if !strings.HasPrefix(lines[y], strings.Repeat("⣿", cols)) {
				t.Fatalf("board row %d has a gap: %q", y, lines[y])
			}
		}
	}
}
