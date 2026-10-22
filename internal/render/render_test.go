package render

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"github.com/charmbracelet/x/ansi"
	"hilife/internal/app"
	"hilife/internal/life"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
)

func TestBrailleMapsAllEightDots(t *testing.T) {
	s, _ := app.New(1)
	s.World.Clear()
	s.Camera = app.Camera{X: 1, Y: 2, Zoom: 1}
	for y := 0; y < 4; y++ {
		for x := 0; x < 2; x++ {
			s.World.Set(life.Point{X: x, Y: y}, true)
		}
	}
	if got := ansi.Strip(Braille(s, 1, 1, nil)); got != "⣿" {
		t.Fatal(got)
	}
}

func TestBrailleFilledViewportHasNoMissingRows(t *testing.T) {
	for _, zoom := range []float64{.25, .8, 1, 1.25, 2} {
		t.Run(fmt.Sprint(zoom), func(t *testing.T) {
			s, _ := app.New(1)
			s.World.Clear()
			s.Camera = app.Camera{X: .5, Y: -.5, Zoom: zoom}
			const cols, rows = 12, 9
			for y := -100; y <= 100; y++ {
				for x := -100; x <= 100; x++ {
					s.World.Cells[life.Point{X: x, Y: y}] = life.Cell{Age: 1, State: 1}
				}
			}
			lines := strings.Split(ansi.Strip(Braille(s, cols, rows, nil)), "\n")
			if len(lines) != rows {
				t.Fatalf("got %d rows, want %d", len(lines), rows)
			}
			for y, line := range lines {
				if line != strings.Repeat("⣿", cols) {
					t.Fatalf("row %d contains a gap: %q", y, line)
				}
			}
		})
	}
}
func TestKittyPayloadIsValidChunkedPNG(t *testing.T) {
	im := image.NewRGBA(image.Rect(0, 0, 120, 80))
	for y := 0; y < 80; y++ {
		for x := 0; x < 120; x++ {
			im.SetRGBA(x, y, color.RGBA{uint8(x * y), uint8(x + y), uint8(y * 9), 255})
		}
	}
	s, e := KittyFrame(im, 12, 4, 2)
	if e != nil {
		t.Fatal(e)
	}
	var data strings.Builder
	chunks := strings.Split(s, "\x1b_G")[1:]
	if len(chunks) < 2 {
		t.Fatal("test should exercise chunking")
	}
	for _, chunk := range chunks {
		payload := strings.SplitN(chunk, ";", 2)[1]
		payload = strings.SplitN(payload, "\x1b\\", 2)[0]
		data.WriteString(payload)
	}
	raw, e := base64.StdEncoding.DecodeString(data.String())
	if e != nil {
		t.Fatal(e)
	}
	out, e := png.Decode(bytes.NewReader(raw))
	if e != nil {
		t.Fatal(e)
	}
	if out.Bounds() != im.Bounds() {
		t.Fatal(out.Bounds())
	}
	if !strings.Contains(s, "c=12,r=4,C=1,z=1,q=2") {
		t.Fatal("incorrect placement")
	}
}
