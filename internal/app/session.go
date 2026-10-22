package app

import (
	"fmt"
	"math"
	"slices"
	"time"

	"hilife/internal/life"
)

type Camera struct {
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
	Zoom float64 `json:"zoom"`
}
type Action struct {
	Type   string  `json:"type"`
	Value  string  `json:"value"`
	Text   string  `json:"text"`
	Number float64 `json:"number"`
	X      int     `json:"x"`
	Y      int     `json:"y"`
	X2     int     `json:"x2"`
	Y2     int     `json:"y2"`
	Erase  bool    `json:"erase"`
}
type Snapshot struct {
	life.Metrics
	Name       string     `json:"name"`
	Rule       string     `json:"rule"`
	Mode       string     `json:"mode"`
	Theme      string     `json:"theme"`
	Running    bool       `json:"running"`
	TPS        int        `json:"tps"`
	Camera     Camera     `json:"camera"`
	Cells      [][4]int   `json:"cells"`
	Trails     [][3]int   `json:"trails"`
	ShowTrails bool       `json:"showTrails"`
	Grid       bool       `json:"grid"`
	Wrap       bool       `json:"wrap"`
	Width      int        `json:"width"`
	Height     int        `json:"height"`
	Undo       int        `json:"undo"`
	StepMS     float64    `json:"stepMs"`
	Revision   uint64     `json:"revision"`
	Ant        life.Point `json:"ant"`
}
type Session struct {
	World                         *life.World
	Initial                       *life.World
	History                       []*life.World
	Camera                        Camera
	Name, Theme                   string
	Running, Grid, Minimap, Focus bool
	TPS                           int
	Seed                          uint64
	Density                       float64
	Revision                      uint64
	StepMS                        float64
	accumulator                   float64
}

func New(seed uint64) (*Session, error) {
	w, e := life.New("B3/S23", seed)
	if e != nil {
		return nil, e
	}
	if e = w.LoadPreset("gosper"); e != nil {
		return nil, e
	}
	s := &Session{World: w, Camera: Camera{Zoom: 1}, Name: "gosper", Theme: "mint", Running: false, TPS: 20, Seed: seed, Density: .28, Minimap: true}
	s.Center()
	s.Initial = w.Clone()
	return s, nil
}
func (s *Session) Save() {
	s.History = append(s.History, s.World.Clone())
	if len(s.History) > 32 {
		s.History = s.History[len(s.History)-32:]
	}
}
func (s *Session) Remember() { s.Initial = s.World.Clone() }
func (s *Session) Center() {
	a, b, ok := s.World.Bounds()
	if ok {
		s.Camera.X = float64(a.X+b.X) / 2
		s.Camera.Y = float64(a.Y+b.Y) / 2
	} else {
		s.Camera.X = 0
		s.Camera.Y = 0
	}
}
func (s *Session) Fit(width, height float64) {
	s.Center()
	a, b, ok := s.World.Bounds()
	if ok {
		s.Camera.Zoom = math.Max(.125, math.Max(float64(b.X-a.X+9)/math.Max(width, 1), float64(b.Y-a.Y+9)/math.Max(height, 1)))
	}
}
func (s *Session) Tick(elapsed time.Duration) {
	if !s.Running {
		return
	}
	s.accumulator += elapsed.Seconds() * float64(s.TPS)
	steps := min(64, int(s.accumulator))
	if steps < 1 {
		return
	}
	s.accumulator -= float64(steps)
	s.Save()
	start := time.Now()
	for range steps {
		s.World.Step()
	}
	s.StepMS = float64(time.Since(start).Microseconds()) / 1000
	s.Revision++
}
func (s *Session) Do(a Action) error {
	if math.IsNaN(a.Number) || math.IsInf(a.Number, 0) {
		return fmt.Errorf("enter a finite number")
	}
	if a.Type == "paint" && (math.Abs(float64(a.X-a.X2)) > 8192 || math.Abs(float64(a.Y-a.Y2)) > 8192) {
		return fmt.Errorf("paint strokes may span at most 8192 cells")
	}
	mutates := slices.Contains([]string{"step", "advance", "paint", "toggle-cell", "clear", "random", "pattern", "load", "rule", "mode", "wrap"}, a.Type)
	if mutates {
		s.Save()
	}
	var err error
	switch a.Type {
	case "toggle":
		s.Running = !s.Running
		s.accumulator = 0
	case "pause":
		s.Running = false
	case "step":
		s.Running = false
		start := time.Now()
		s.World.Step()
		s.StepMS = float64(time.Since(start).Microseconds()) / 1000
	case "advance":
		s.Running = false
		if a.Number < 1 || a.Number > 1<<20 {
			err = fmt.Errorf("jump must be 1–1048576 generations")
			break
		}
		start := time.Now()
		err = s.World.Advance(uint64(a.Number))
		s.StepMS = float64(time.Since(start).Microseconds()) / 1000
	case "undo":
		s.Running = false
		if len(s.History) > 0 {
			s.World = s.History[len(s.History)-1]
			s.History = s.History[:len(s.History)-1]
		}
	case "reset":
		s.Save()
		s.World = s.Initial.Clone()
		s.Running = false
		s.Center()
	case "clear":
		s.World.Clear()
		s.Name = "empty"
		s.Running = false
	case "paint":
		s.World.Paint(life.Point{X: a.X, Y: a.Y}, life.Point{X: a.X2, Y: a.Y2}, !a.Erase)
	case "toggle-cell":
		p := life.Point{X: a.X, Y: a.Y}
		_, alive := s.World.Cells[p]
		s.World.Set(p, !alive)
	case "pattern":
		err = s.World.LoadPreset(a.Value)
		if err == nil {
			s.Name = a.Value
			s.Remember()
			s.Running = false
			s.Center()
		}
	case "load":
		err = s.World.LoadText(a.Text)
		if err == nil {
			s.Name = "imported"
			s.Remember()
			s.Running = false
			s.Center()
		}
	case "random":
		width, height := a.X, a.Y
		if width < 1 {
			width = 160
		}
		if height < 1 {
			height = 96
		}
		width = min(width, 500)
		height = min(height, 400)
		s.World.Random(s.Seed, s.Density, int(s.Camera.X)-width/2, int(s.Camera.Y)-height/2, width, height)
		s.Name = "random"
		s.Remember()
	case "seed":
		if a.Number < 0 {
			err = fmt.Errorf("seed must be positive")
		} else {
			s.Seed = uint64(a.Number)
		}
	case "density":
		if a.Number < 0 || a.Number > 1 {
			err = fmt.Errorf("density must be 0–1")
		} else {
			s.Density = a.Number
		}
	case "rule":
		err = s.World.SetRule(a.Value)
	case "mode":
		if !slices.Contains(life.Modes, a.Value) {
			err = fmt.Errorf("unknown automaton")
			break
		}
		s.World.Mode = a.Value
		for p, c := range s.World.Cells {
			c.State = 1
			s.World.Cells[p] = c
		}
		s.World.Trails = map[life.Point]uint8{}
		s.Remember()
	case "wrap":
		s.World.Wrap = !s.World.Wrap
		cells := s.World.Cells
		s.World.Cells = map[life.Point]life.Cell{}
		for p := range cells {
			s.World.Set(p, true)
		}
	case "bounds":
		if a.X < 4 || a.Y < 4 || a.X > 2000 || a.Y > 2000 {
			err = fmt.Errorf("board dimensions must be 4–2000")
		} else {
			s.World.Width = a.X
			s.World.Height = a.Y
		}
	case "speed":
		s.TPS = max(1, min(1000, int(a.Number)))
		s.accumulator = 0
	case "theme":
		if !slices.Contains([]string{"mint", "neon", "amber", "sunset", "mono"}, a.Value) {
			err = fmt.Errorf("unknown theme")
		} else {
			s.Theme = a.Value
		}
	case "trails":
		s.World.ShowTrails = !s.World.ShowTrails
		if !s.World.ShowTrails {
			s.World.Trails = map[life.Point]uint8{}
		}
	case "grid":
		s.Grid = !s.Grid
	case "minimap":
		s.Minimap = !s.Minimap
	case "focus":
		s.Focus = !s.Focus
	case "pan":
		s.Camera.X += float64(a.X) * s.Camera.Zoom
		s.Camera.Y += float64(a.Y) * s.Camera.Zoom
	case "jump":
		s.Camera.X = float64(a.X)
		s.Camera.Y = float64(a.Y)
	case "home":
		s.Camera.X = 0
		s.Camera.Y = 0
	case "center":
		s.Center()
	case "zoom":
		s.Camera.Zoom = math.Max(1.0/32, math.Min(128, s.Camera.Zoom*a.Number))
	case "fit":
		s.Fit(float64(a.X), float64(a.Y))
	default:
		err = fmt.Errorf("unknown action %q", a.Type)
	}
	if err != nil && mutates && len(s.History) > 0 {
		s.World = s.History[len(s.History)-1]
		s.History = s.History[:len(s.History)-1]
	}
	s.Revision++
	return err
}
func (s *Session) Snapshot() Snapshot {
	v := Snapshot{Metrics: s.World.Metrics, Name: s.Name, Rule: s.World.Rule.Text, Mode: s.World.Mode, Theme: s.Theme, Running: s.Running, TPS: s.TPS, Camera: s.Camera, ShowTrails: s.World.ShowTrails, Grid: s.Grid, Wrap: s.World.Wrap, Width: s.World.Width, Height: s.World.Height, Undo: len(s.History), StepMS: s.StepMS, Revision: s.Revision, Ant: s.World.Ant, Cells: make([][4]int, 0, len(s.World.Cells)), Trails: make([][3]int, 0, len(s.World.Trails))}
	for p, c := range s.World.Cells {
		v.Cells = append(v.Cells, [4]int{p.X, p.Y, int(c.Age), int(c.State)})
	}
	for p, t := range s.World.Trails {
		v.Trails = append(v.Trails, [3]int{p.X, p.Y, int(t)})
	}
	return v
}
