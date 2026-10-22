package render

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"strings"

	"hilife/internal/app"
	"hilife/internal/life"
)

type Palette struct{ Bg, Young, Old, Accent color.RGBA }

func Colors(name string) Palette {
	bg := color.RGBA{11, 17, 21, 255}
	switch name {
	case "neon":
		return Palette{bg, color.RGBA{95, 231, 255, 255}, color.RGBA{190, 120, 255, 255}, color.RGBA{255, 126, 206, 255}}
	case "amber":
		return Palette{bg, color.RGBA{255, 213, 117, 255}, color.RGBA{230, 129, 58, 255}, color.RGBA{255, 213, 117, 255}}
	case "sunset":
		return Palette{bg, color.RGBA{255, 173, 124, 255}, color.RGBA{224, 92, 154, 255}, color.RGBA{251, 207, 126, 255}}
	case "mono":
		return Palette{bg, color.RGBA{240, 244, 242, 255}, color.RGBA{125, 146, 147, 255}, color.RGBA{240, 244, 242, 255}}
	}
	return Palette{bg, color.RGBA{187, 251, 206, 255}, color.RGBA{46, 164, 141, 255}, color.RGBA{180, 240, 154, 255}}
}
func mix(a, b color.RGBA, t float64) color.RGBA {
	t = max(0, min(1, t))
	return color.RGBA{uint8(float64(a.R)*(1-t) + float64(b.R)*t), uint8(float64(a.G)*(1-t) + float64(b.G)*t), uint8(float64(a.B)*(1-t) + float64(b.B)*t), 255}
}
func CellColor(p Palette, age uint32, state uint8) color.RGBA {
	if state > 1 {
		return mix(p.Old, p.Bg, float64(state)/14)
	}
	return mix(p.Young, p.Old, min(1, math.Log2(float64(age)+1)/8))
}
func Hex(c color.RGBA) string { return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B) }

var dotBits = [4][2]uint8{{1, 8}, {2, 16}, {4, 32}, {64, 128}}

type Dot struct {
	Mask  uint8
	Age   uint32
	State uint8
	Trail uint8
	Count int
}

// Rasterize iterates occupied world cells, rather than doing eight map lookups
// for every terminal character. Empty space costs only the final string pass.
func Braille(s *app.Session, cols, rows int, cursor *life.Point) string {
	if cols < 1 || rows < 1 {
		return ""
	}
	dots := make([]Dot, cols*rows)
	scale := s.Camera.Zoom
	left := s.Camera.X - float64(cols)
	top := s.Camera.Y - float64(rows*2)
	left = s.Camera.X - float64(cols)*scale
	top = s.Camera.Y - float64(rows*2)*scale
	plot := func(p life.Point, c life.Cell, trail uint8) {
		x0 := int(math.Floor((float64(p.X) - left) / scale))
		y0 := int(math.Floor((float64(p.Y) - top) / scale))
		x1 := max(x0+1, int(math.Ceil((float64(p.X+1)-left)/scale)))
		y1 := max(y0+1, int(math.Ceil((float64(p.Y+1)-top)/scale)))
		for y := max(0, y0); y < min(rows*4, y1); y++ {
			for x := max(0, x0); x < min(cols*2, x1); x++ {
				d := &dots[(y/4)*cols+x/2]
				d.Mask |= dotBits[y%4][x%2]
				d.Age = max(d.Age, c.Age)
				d.State = max(d.State, c.State)
				d.Trail = max(d.Trail, trail)
				d.Count++
			}
		}
	}
	for p, t := range s.World.Trails {
		if _, alive := s.World.Cells[p]; !alive {
			plot(p, life.Cell{}, t)
		}
	}
	for p, c := range s.World.Cells {
		plot(p, c, 0)
	}
	p := Colors(s.Theme)
	var b strings.Builder
	b.Grow(cols * rows * 7)
	last := ""
	for y := 0; y < rows; y++ {
		for x := 0; x < cols; x++ {
			d := dots[y*cols+x]
			c := CellColor(p, d.Age, d.State)
			glyph := ' '
			if d.Mask != 0 {
				glyph = rune(0x2800) + rune(d.Mask)
				if d.Age == 0 {
					c = mix(p.Bg, p.Old, float64(d.Trail)/24)
				}
			} else if s.Grid {
				wx := int(left + float64(x*2)*scale)
				wy := int(top + float64(y*4)*scale)
				if wx%64 == 0 || wy%64 == 0 {
					glyph = '·'
					c = mix(p.Bg, p.Old, .3)
				}
			}
			if cursor != nil {
				cx := int(math.Floor((float64(cursor.X)-left)/scale)) / 2
				cy := int(math.Floor((float64(cursor.Y)-top)/scale)) / 4
				if x == cx && y == cy {
					glyph = '╳'
					c = p.Accent
				}
			}
			ansi := fmt.Sprintf("\x1b[38;2;%d;%d;%dm", c.R, c.G, c.B)
			if ansi != last {
				b.WriteString(ansi)
				last = ansi
			}
			b.WriteRune(glyph)
		}
		b.WriteString("\x1b[0m")
		last = ""
		if y < rows-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}
func Image(s *app.Session, cols, rows, cellW, cellH int, cursor *life.Point) *image.RGBA {
	w, h := min(2400, max(1, cols*cellW)), min(1600, max(1, rows*cellH))
	im := image.NewRGBA(image.Rect(0, 0, w, h))
	palette := Colors(s.Theme)
	draw.Draw(im, im.Bounds(), image.NewUniform(palette.Bg), image.Point{}, draw.Src)
	worldW, worldH := float64(cols*2)*s.Camera.Zoom, float64(rows*4)*s.Camera.Zoom
	left, top := s.Camera.X-worldW/2, s.Camera.Y-worldH/2
	sx, sy := float64(w)/worldW, float64(h)/worldH
	if s.Grid {
		c := mix(palette.Bg, palette.Old, .15)
		for x := int(math.Floor(left/64)) * 64; float64(x) < left+worldW; x += 64 {
			px := int((float64(x) - left) * sx)
			draw.Draw(im, image.Rect(px, 0, px+1, h), image.NewUniform(c), image.Point{}, draw.Src)
		}
		for y := int(math.Floor(top/64)) * 64; float64(y) < top+worldH; y += 64 {
			py := int((float64(y) - top) * sy)
			draw.Draw(im, image.Rect(0, py, w, py+1), image.NewUniform(c), image.Point{}, draw.Src)
		}
	}
	plot := func(p life.Point, c color.RGBA) {
		x, y := int(math.Floor((float64(p.X)-left)*sx)), int(math.Floor((float64(p.Y)-top)*sy))
		x2, y2 := max(x+1, int(math.Ceil((float64(p.X+1)-left)*sx))), max(y+1, int(math.Ceil((float64(p.Y+1)-top)*sy)))
		if x >= w || y >= h || x2 <= 0 || y2 <= 0 {
			return
		}
		draw.Draw(im, image.Rect(x, y, x2, y2), image.NewUniform(c), image.Point{}, draw.Src)
	}
	for p, t := range s.World.Trails {
		plot(p, mix(palette.Bg, palette.Old, float64(t)/24))
	}
	for p, c := range s.World.Cells {
		plot(p, CellColor(palette, c.Age, c.State))
	}
	if s.World.Mode == "langtons-ant" {
		plot(s.World.Ant, palette.Accent)
	}
	if cursor != nil {
		plot(*cursor, palette.Accent)
	}
	return im
}

const QueryKitty = "\x1b_Gi=31,s=1,v=1,a=q,t=d,f=24;AAAA\x1b\\"
const DeleteKitty = "\x1b_Ga=d,d=I,i=4242,q=2\x1b\\"

func KittyFrame(im image.Image, cols, rows, top int) (string, error) {
	var compressed bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := enc.Encode(&compressed, im); err != nil {
		return "", err
	}
	data := base64.StdEncoding.EncodeToString(compressed.Bytes())
	var out strings.Builder
	out.WriteString(fmt.Sprintf("\x1b7\x1b[%d;1H", top+1))
	first := true
	for len(data) > 0 {
		n := min(4096, len(data))
		more := 0
		if n < len(data) {
			more = 1
		}
		if first {
			fmt.Fprintf(&out, "\x1b_Ga=T,t=d,f=100,i=4242,p=1,c=%d,r=%d,C=1,z=1,q=2,m=%d;", cols, rows, more)
			first = false
		} else {
			fmt.Fprintf(&out, "\x1b_Gm=%d;", more)
		}
		out.WriteString(data[:n])
		out.WriteString("\x1b\\")
		data = data[n:]
	}
	out.WriteString("\x1b8")
	return out.String(), nil
}
func Passthrough(s string, tmux bool) string {
	if !tmux {
		return s
	}
	return "\x1bPtmux;" + strings.ReplaceAll(s, "\x1b", "\x1b\x1b") + "\x1b\\"
}
func Minimap(s *app.Session, width, height int) string {
	a, b, ok := s.World.Bounds()
	if !ok {
		return "  empty world"
	}
	spanX, spanY := max(1, b.X-a.X+1), max(1, b.Y-a.Y+1)
	grid := make([][]rune, height)
	for y := range grid {
		grid[y] = []rune(strings.Repeat("·", width))
	}
	for p := range s.World.Cells {
		x := (p.X - a.X) * (width - 1) / spanX
		y := (p.Y - a.Y) * (height - 1) / spanY
		grid[y][x] = '▪'
	}
	cx := int((s.Camera.X - float64(a.X)) * float64(width-1) / float64(spanX))
	cy := int((s.Camera.Y - float64(a.Y)) * float64(height-1) / float64(spanY))
	if cx >= 0 && cy >= 0 && cx < width && cy < height {
		grid[cy][cx] = '+'
	}
	var lines []string
	for _, row := range grid {
		lines = append(lines, string(row))
	}
	return strings.Join(lines, "\n")
}
