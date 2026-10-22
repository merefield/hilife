package life

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Pattern struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Rule        string `json:"rule"`
	RLE         string `json:"rle"`
}

var Presets = []Pattern{
	{"gosper", "Gosper glider gun · a glider every 30 generations", "B3/S23", "24bo$22bobo$12b2o6b2o12b2o$11bo3bo4b2o12b2o$2o8bo5bo3b2o$2o8bo3bob2o4bobo$10bo5bo7bo$11bo3bo$12b2o!"},
	{"glider", "The smallest spaceship · period 4", "B3/S23", "bo$2bo$3o!"},
	{"acorn", "Seven cells, more than 5,000 generations of growth", "B3/S23", "bo$3bo$2o2b3o!"},
	{"rpentomino", "The five-cell methuselah", "B3/S23", "b2o$2o$bo!"},
	{"pulsar", "A symmetric period-3 oscillator", "B3/S23", "2b3o3b3o2$o4bobo4bo$o4bobo4bo$o4bobo4bo$2b3o3b3o2$2b3o3b3o$o4bobo4bo$o4bobo4bo$o4bobo4bo2$2b3o3b3o!"},
	{"lwss", "Lightweight spaceship", "B3/S23", "bo2bo$o4b$o3bo$4o!"},
	{"diehard", "Disappears after 130 generations", "B3/S23", "6bo$2o$bo3b3o!"},
	{"replicator", "HighLife's self-replicating seed", "B36/S23", "2b3o$bo2bo$o3bo$o2bo$3o!"},
	{"blinker", "A simple period-2 oscillator", "B3/S23", "3o!"},
	{"toad", "Six-cell period-2 oscillator", "B3/S23", "b3o$3o!"},
	{"beacon", "Two blocks trading corners", "B3/S23", "2o$2o$2b2o$2b2o!"},
	{"block", "A still life", "B3/S23", "2o$2o!"},
}

type BoardFile struct {
	Name      string   `json:"name"`
	Rule      string   `json:"rule"`
	Algorithm string   `json:"algorithm"`
	Cells     [][2]int `json:"cells"`
}

func (w *World) LoadPreset(name string) error {
	for _, p := range Presets {
		if p.Name == name {
			if err := w.LoadText("x = 0, y = 0, rule = " + p.Rule + "\n" + p.RLE); err != nil {
				return err
			}
			w.Mode = "life"
			return nil
		}
	}
	if name == "empty" {
		w.Clear()
		return nil
	}
	return fmt.Errorf("unknown pattern %q", name)
}
func (w *World) LoadText(text string) error {
	text = strings.TrimSpace(strings.TrimPrefix(text, "\ufeff"))
	if text == "" {
		return fmt.Errorf("pattern is empty")
	}
	temp, _ := New(w.Rule.Text, 1)
	temp.Mode = w.Mode
	if strings.HasPrefix(text, "{") {
		var b BoardFile
		if err := json.Unmarshal([]byte(text), &b); err != nil {
			return err
		}
		if len(b.Cells) > 200000 {
			return fmt.Errorf("pattern exceeds 200000 cells")
		}
		rule := b.Rule
		if rule == "" {
			if strings.EqualFold(b.Algorithm, "highlife") {
				rule = "B36/S23"
			} else {
				rule = "B3/S23"
			}
		}
		if e := temp.SetRule(rule); e != nil {
			return e
		}
		for _, p := range b.Cells {
			if abs(p[0]) > 1<<30 || abs(p[1]) > 1<<30 {
				return fmt.Errorf("coordinates exceed supported range")
			}
			temp.Cells[Point{p[0], p[1]}] = Cell{1, 1}
		}
	} else if looksRLE(text) {
		if err := temp.parseRLE(text); err != nil {
			return err
		}
	} else {
		y := 0
		for _, line := range strings.Split(text, "\n") {
			line = strings.TrimSuffix(line, "\r")
			if strings.HasPrefix(line, "!") || strings.HasPrefix(line, "#") {
				continue
			}
			for x, c := range line {
				switch c {
				case 'O', 'o', '*', '@', '1':
					temp.Cells[Point{x, y}] = Cell{1, 1}
				case '.', ' ', '0':
				default:
					return fmt.Errorf("unknown plaintext cell %q", c)
				}
			}
			y++
		}
		if len(temp.Cells) > 200000 {
			return fmt.Errorf("pattern exceeds 200000 cells")
		}
	}
	w.Clear()
	w.Rule = temp.Rule
	w.Cells = temp.Cells
	w.Recount()
	return nil
}

var rleRule = regexp.MustCompile(`(?i)rule\s*=\s*([^,\s]+)`)

func (w *World) parseRLE(text string) error {
	var body strings.Builder
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(strings.ToLower(line), "x") {
			if m := rleRule.FindStringSubmatch(line); len(m) > 1 {
				if err := w.SetRule(m[1]); err != nil {
					return err
				}
			}
			continue
		}
		body.WriteString(line)
	}
	x, y, count := 0, 0, 0
	ended := false
	for _, c := range body.String() {
		if c >= '0' && c <= '9' {
			count = count*10 + int(c-'0')
			if count > 1000000 {
				return fmt.Errorf("RLE run is too large")
			}
			continue
		}
		n := max(1, count)
		count = 0
		switch c {
		case 'b', '.':
			x += n
		case 'o', 'A':
			if len(w.Cells)+n > 200000 {
				return fmt.Errorf("pattern exceeds 200000 cells")
			}
			for i := 0; i < n; i++ {
				w.Cells[Point{x + i, y}] = Cell{1, 1}
			}
			x += n
		case '$':
			y += n
			x = 0
		case '!':
			ended = true
		case ' ', '\t', '\r', '\n':
		default:
			return fmt.Errorf("unsupported RLE symbol %q", c)
		}
		if ended {
			break
		}
		if x > 1<<30 || y > 1<<30 {
			return fmt.Errorf("pattern extent is too large")
		}
	}
	if !ended {
		return fmt.Errorf("RLE must end with !")
	}
	return nil
}
func (w *World) ExportJSON() string {
	cells := make([][2]int, 0, len(w.Cells))
	for _, p := range w.SortedPoints() {
		cells = append(cells, [2]int{p.X, p.Y})
	}
	b, _ := json.MarshalIndent(BoardFile{Name: "hilife", Rule: w.Rule.Text, Cells: cells}, "", "  ")
	return string(b) + "\n"
}
func (w *World) ExportRLE() (string, error) {
	a, b, ok := w.Bounds()
	if !ok {
		return "x = 0, y = 0, rule = " + w.Rule.Text + "\n!\n", nil
	}
	if b.X-a.X > 100000 || b.Y-a.Y > 100000 || int64(b.X-a.X+1)*int64(b.Y-a.Y+1) > 10000000 {
		return "", fmt.Errorf("pattern is too spread out for RLE; export JSON instead")
	}
	var body strings.Builder
	run := func(n int, c byte) {
		if n > 1 {
			body.WriteString(strconv.Itoa(n))
		}
		body.WriteByte(c)
	}
	for y := a.Y; y <= b.Y; y++ {
		last := a.X - 1
		for x := b.X; x >= a.X; x-- {
			if _, ok := w.Cells[Point{x, y}]; ok {
				last = x
				break
			}
		}
		for x := a.X; x <= last; {
			_, alive := w.Cells[Point{x, y}]
			end := x + 1
			for end <= last {
				_, v := w.Cells[Point{end, y}]
				if v != alive {
					break
				}
				end++
			}
			c := byte('b')
			if alive {
				c = 'o'
			}
			run(end-x, c)
			x = end
		}
		if y < b.Y {
			body.WriteByte('$')
		}
	}
	body.WriteByte('!')
	return fmt.Sprintf("#N hilife\n#C Original origin: %d,%d; JSON preserves absolute coordinates.\nx = %d, y = %d, rule = %s\n%s\n", a.X, a.Y, b.X-a.X+1, b.Y-a.Y+1, w.Rule.Text, body.String()), nil
}

const MaxPatternBytes = 4 << 20

var patternLink = regexp.MustCompile(`(?i)href\s*=\s*["']([^"']+\.(?:rle|cells)(?:\?[^"']*)?)["']`)

func ReadSource(ctx context.Context, source string) (string, error) {
	if !strings.HasPrefix(source, "http://") && !strings.HasPrefix(source, "https://") {
		if strings.HasPrefix(source, "~/") {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			source = home + source[1:]
		}
		f, err := os.Open(source)
		if err != nil {
			return "", err
		}
		defer f.Close()
		return readLimited(f)
	}
	client := &http.Client{Timeout: 12 * time.Second}
	current := source
	for range 3 {
		u, err := url.Parse(current)
		if err != nil {
			return "", err
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			return "", fmt.Errorf("only HTTP(S) pattern URLs are supported")
		}
		req, err := http.NewRequestWithContext(ctx, "GET", current, nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("User-Agent", "hilife/0.1")
		resp, err := client.Do(req)
		if err != nil {
			return "", err
		}
		text, readErr := readLimited(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 200 {
			return "", fmt.Errorf("pattern server returned %s", resp.Status)
		}
		if readErr != nil {
			return "", readErr
		}
		if strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/html") {
			m := patternLink.FindStringSubmatch(text)
			if len(m) < 2 {
				return "", fmt.Errorf("page has no RLE or plaintext pattern links")
			}
			link, e := url.Parse(m[1])
			if e != nil {
				return "", e
			}
			current = resp.Request.URL.ResolveReference(link).String()
			continue
		}
		return text, nil
	}
	return "", fmt.Errorf("too many pattern page redirects")
}
func readLimited(r io.Reader) (string, error) {
	b, e := io.ReadAll(io.LimitReader(r, MaxPatternBytes+1))
	if len(b) > MaxPatternBytes {
		return "", fmt.Errorf("pattern file exceeds 4 MiB")
	}
	return string(b), e
}

func looksRLE(text string) bool {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") {
			continue
		}
		return strings.HasPrefix(strings.ToLower(line), "x") || strings.ContainsAny(line, "$!")
	}
	return text == "!"
}
