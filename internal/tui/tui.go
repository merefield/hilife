package tui

import (
	"context"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"hilife/internal/app"
	"hilife/internal/life"
	"hilife/internal/render"
)

type tickMsg time.Time
type loadedMsg struct {
	text string
	err  error
}
type probeTimeout struct{}
type Model struct {
	s                       *app.Session
	width, height           int
	backend                 string
	kitty, tmux, pixelMouse bool
	cellW, cellH            int
	last                    time.Time
	input                   textinput.Model
	helper                  help.Model
	prompt, notice          string
	palette, showHelp       bool
	selected                int
	cursor                  life.Point
	showCursor              bool
	dragging, erase         bool
	lastPaint               life.Point
	loading                 bool
}

func New(s *app.Session, backend string) *Model {
	in := textinput.New()
	in.CharLimit = 4096
	in.Placeholder = "pattern glider · load path-or-url · rule B36/S23 · save scene.rle"
	return &Model{s: s, backend: backend, tmux: os.Getenv("TMUX") != "", cellW: 10, cellH: 20, input: in, helper: help.New(), last: time.Now()}
}
func frame() tea.Cmd {
	return tea.Tick(time.Second/60, func(t time.Time) tea.Msg { return tickMsg(t) })
}
func (m *Model) raw(s string) tea.Cmd { return tea.Raw(render.Passthrough(s, m.tmux)) }
func (m *Model) Init() tea.Cmd {
	cmds := []tea.Cmd{frame(), m.raw("\x1b[16t"), tea.Tick(time.Second, func(time.Time) tea.Msg { return probeTimeout{} })}
	if m.backend != "braille" {
		cmds = append(cmds, m.raw(render.QueryKitty))
	}
	return tea.Batch(cmds...)
}
func (m *Model) boardSize() (int, int, int) {
	top := 2
	// Two header rows and two footer rows; use every remaining board row.
	rows := m.height - 4
	if m.s.Focus {
		top = 0
		rows = m.height
	}
	cols := m.width
	if m.s.Minimap && !m.s.Focus && m.width >= 110 {
		cols -= 28
	}
	return max(1, cols), max(1, rows), top
}
func (m *Model) kittyActive() bool {
	return m.kitty && m.backend != "braille" && !m.showHelp && !m.palette && m.prompt == ""
}
func (m *Model) imageCmd() tea.Cmd {
	if !m.kittyActive() {
		if m.kitty {
			return m.raw(render.DeleteKitty)
		}
		return nil
	}
	cols, rows, top := m.boardSize()
	var cursor *life.Point
	if m.showCursor {
		cursor = &m.cursor
	}
	data, err := render.KittyFrame(render.Image(m.s, cols, rows, m.cellW, m.cellH, cursor), cols, rows, top)
	if err != nil {
		m.notice = err.Error()
		m.backend = "braille"
		return m.raw(render.DeleteKitty)
	}
	return m.raw(data)
}
func (m *Model) action(a app.Action) {
	if err := m.s.Do(a); err != nil {
		m.notice = err.Error()
	} else {
		m.notice = ""
	}
}
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.input.SetWidth(max(10, msg.Width-8))
	case uv.CellSizeEvent:
		if msg.Width > 0 && msg.Height > 0 {
			m.cellW, m.cellH = msg.Width, msg.Height
		}
		if m.kitty && !m.pixelMouse {
			m.pixelMouse = true
			return m, tea.Batch(m.raw("\x1b[?1016h"), m.imageCmd())
		}
	case uv.KittyGraphicsEvent:
		if msg.Options.ID == 31 && string(msg.Payload) == "OK" {
			m.kitty = true
			m.notice = "Kitty graphics connected"
			return m, tea.Batch(m.raw("\x1b[16t"), m.imageCmd())
		}
	case probeTimeout:
		if m.backend == "kitty" && !m.kitty {
			m.notice = "No Kitty response; using Braille"
		}
	case loadedMsg:
		m.loading = false
		if msg.err != nil {
			m.notice = msg.err.Error()
		} else {
			m.action(app.Action{Type: "load", Text: msg.text})
			m.notice = "Pattern loaded"
		}
	case tickMsg:
		now := time.Time(msg)
		revision := m.s.Revision
		m.s.Tick(min(now.Sub(m.last), time.Second/4))
		m.last = now
		if revision != m.s.Revision {
			return m, tea.Batch(frame(), m.imageCmd())
		}
		return m, frame()
	case tea.KeyPressMsg:
		k := msg.String()
		if k == "ctrl+c" {
			return m, tea.Quit
		}
		if m.prompt != "" {
			switch k {
			case "esc":
				m.prompt = ""
				m.input.Blur()
			case "enter":
				line := strings.TrimSpace(m.input.Value())
				m.prompt = ""
				m.input.Blur()
				return m, tea.Batch(m.command(line), m.imageCmd())
			default:
				m.input, cmd = m.input.Update(msg)
				return m, cmd
			}
			break
		}
		if m.showHelp {
			m.showHelp = false
			break
		}
		if m.palette {
			switch k {
			case "esc", "p":
				m.palette = false
			case "up", "k":
				m.selected = (m.selected + len(life.Presets) - 1) % len(life.Presets)
			case "down", "j":
				m.selected = (m.selected + 1) % len(life.Presets)
			case "enter":
				m.action(app.Action{Type: "pattern", Value: life.Presets[m.selected].Name})
				m.palette = false
			}
			break
		}
		switch k {
		case "q", "esc":
			return m, tea.Quit
		case "space":
			m.action(app.Action{Type: "toggle"})
		case "t", "n":
			m.action(app.Action{Type: "step"})
		case "u":
			m.action(app.Action{Type: "undo"})
		case "r":
			m.action(app.Action{Type: "reset"})
		case "R":
			m.action(app.Action{Type: "random"})
		case "c":
			m.action(app.Action{Type: "clear"})
		case "+", "=":
			m.action(app.Action{Type: "zoom", Number: .8})
		case "-", "_":
			m.action(app.Action{Type: "zoom", Number: 1.25})
		case ">", ".":
			m.action(app.Action{Type: "speed", Number: float64(m.s.TPS + 5)})
		case "<", ",":
			m.action(app.Action{Type: "speed", Number: float64(m.s.TPS - 5)})
		case "w", "a", "s", "d", "W", "A", "S", "D":
			dx, dy := 0, 0
			switch strings.ToLower(k) {
			case "w":
				dy = -8
			case "s":
				dy = 8
			case "a":
				dx = -8
			case "d":
				dx = 8
			}
			if k == strings.ToUpper(k) {
				dx *= 8
				dy *= 8
			}
			m.action(app.Action{Type: "pan", X: dx, Y: dy})
		case "up", "down", "left", "right":
			m.showCursor = true
			switch k {
			case "up":
				m.cursor.Y--
			case "down":
				m.cursor.Y++
			case "left":
				m.cursor.X--
			case "right":
				m.cursor.X++
			}
			m.ensureCursor()
		case "x", "enter":
			m.showCursor = true
			m.action(app.Action{Type: "toggle-cell", X: m.cursor.X, Y: m.cursor.Y})
		case "h":
			m.action(app.Action{Type: "home"})
		case "0":
			cols, rows, _ := m.boardSize()
			m.action(app.Action{Type: "fit", X: cols * 2, Y: rows * 4})
		case "g":
			m.action(app.Action{Type: "grid"})
		case "m":
			m.action(app.Action{Type: "minimap"})
		case "f":
			m.action(app.Action{Type: "focus"})
		case "l":
			m.action(app.Action{Type: "trails"})
		case "b":
			if m.backend == "braille" && m.kitty {
				m.backend = "auto"
			} else {
				m.backend = "braille"
			}
		case "y":
			themes := []string{"mint", "neon", "amber", "sunset", "mono"}
			for i, t := range themes {
				if t == m.s.Theme {
					m.action(app.Action{Type: "theme", Value: themes[(i+1)%len(themes)]})
					break
				}
			}
		case "p":
			m.palette = true
			m.s.Running = false
		case "?":
			m.showHelp = true
		case ":", "o", "j", "ctrl+s", "e":
			m.prompt = ":"
			value := ""
			switch k {
			case "o":
				value = "load "
			case "j":
				value = "jump "
			case "ctrl+s":
				value = "save hilife.rle"
			case "e":
				value = "rule " + m.s.World.Rule.Text
			}
			m.input.SetValue(value)
			cmd = m.input.Focus()
			m.s.Running = false
		}
	case tea.MouseMsg:
		if m.palette || m.showHelp || m.prompt != "" {
			break
		}
		mouse := msg.Mouse()
		px, py := float64(mouse.X), float64(mouse.Y)
		if m.pixelMouse {
			px /= float64(m.cellW)
			py /= float64(m.cellH)
		}
		cols, rows, top := m.boardSize()
		if _, ok := msg.(tea.MouseWheelMsg); ok {
			factor := 1.25
			if mouse.Button == tea.MouseWheelUp {
				factor = .8
			}
			m.action(app.Action{Type: "zoom", Number: factor})
			break
		}
		if _, ok := msg.(tea.MouseReleaseMsg); ok {
			m.dragging = false
			break
		}
		if px >= float64(cols) && m.s.Minimap && py >= float64(top) && py < float64(top+12) {
			if _, ok := msg.(tea.MouseClickMsg); ok {
				a, b, exists := m.s.World.Bounds()
				if exists {
					m.action(app.Action{Type: "jump", X: a.X + int((px-float64(cols)-2)/23*float64(b.X-a.X+1)), Y: a.Y + int((py-float64(top)-1)/10*float64(b.Y-a.Y+1))})
				}
			}
			break
		}
		if px < 0 || px >= float64(cols) || py < float64(top) || py >= float64(rows+top) {
			break
		}
		p := life.Point{X: int(math.Floor(m.s.Camera.X + (px-float64(cols)/2)*2*m.s.Camera.Zoom)), Y: int(math.Floor(m.s.Camera.Y + (py-float64(top)-float64(rows)/2)*4*m.s.Camera.Zoom))}
		if mouse.Button == tea.MouseLeft || mouse.Button == tea.MouseRight {
			if !m.dragging {
				m.s.Save()
				m.lastPaint = p
				m.dragging = true
				m.erase = mouse.Button == tea.MouseRight
			}
			m.s.World.Paint(m.lastPaint, p, !m.erase)
			m.s.Revision++
			m.lastPaint = p
		}
	}
	return m, tea.Batch(cmd, m.imageCmd())
}
func (m *Model) ensureCursor() {
	cols, rows, _ := m.boardSize()
	halfW, halfH := float64(cols)*m.s.Camera.Zoom, float64(rows*2)*m.s.Camera.Zoom
	if math.Abs(float64(m.cursor.X)-m.s.Camera.X) > halfW-1 {
		m.s.Camera.X = float64(m.cursor.X)
	}
	if math.Abs(float64(m.cursor.Y)-m.s.Camera.Y) > halfH-1 {
		m.s.Camera.Y = float64(m.cursor.Y)
	}
}
func (m *Model) command(line string) tea.Cmd {
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return nil
	}
	verb := parts[0]
	value := strings.TrimSpace(strings.TrimPrefix(line, verb))
	a := app.Action{Type: verb, Value: value}
	switch verb {
	case "load":
		m.loading = true
		return func() tea.Msg { text, err := life.ReadSource(context.Background(), value); return loadedMsg{text, err} }
	case "save":
		text := m.s.World.ExportJSON()
		if strings.HasSuffix(value, ".rle") {
			var err error
			text, err = m.s.World.ExportRLE()
			if err != nil {
				m.notice = err.Error()
				return nil
			}
		}
		if value == "" {
			m.notice = "save requires a filename"
			return nil
		}
		if strings.HasPrefix(value, "~/") {
			home, _ := os.UserHomeDir()
			value = home + value[1:]
		}
		err := os.WriteFile(value, []byte(text), 0644)
		if err != nil {
			m.notice = err.Error()
		} else {
			m.notice = "Saved " + value
		}
		return nil
	case "jump":
		if _, err := fmt.Sscanf(strings.ReplaceAll(value, ",", " "), "%d %d", &a.X, &a.Y); err != nil {
			m.notice = "jump X,Y"
			return nil
		}
	case "advance", "speed", "density", "seed":
		n, e := strconv.ParseFloat(value, 64)
		if e != nil {
			m.notice = "enter a number"
			return nil
		}
		a.Number = n
	case "bounds":
		if _, err := fmt.Sscanf(strings.ReplaceAll(value, "x", " "), "%d %d", &a.X, &a.Y); err != nil {
			m.notice = "bounds WIDTHxHEIGHT"
			return nil
		}
	}
	m.action(a)
	return nil
}
func (m *Model) View() tea.View {
	if m.width < 1 {
		return tea.NewView("Starting hilife…")
	}
	cols, rows, _ := m.boardSize()
	palette := render.Colors(m.s.Theme)
	accent := lipgloss.NewStyle().Foreground(palette.Accent).Bold(true)
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("#7d969b"))
	var content string
	if m.showHelp {
		content = accent.Render("  HILIFE / FIELD GUIDE") + "\n\n" + strings.Join([]string{"  space      Run / pause       t / n      Single generation", "  u          Undo              r / R      Reset / randomize", "  WASD       Pan (Shift: ×8)   arrows     Move editing cursor", "  x / enter  Toggle cell       mouse      Paint / right-drag erase", "  + / -      Zoom              < / >      Simulation speed", "  0          Fit pattern       h          World origin", "  p          Pattern library   o          Import file / URL", "  ctrl+s     Save JSON / RLE   e          Edit B/S rule", "  j          Jump to X,Y       m          Minimap (click to jump)", "  g          Chunk grid        l          Fading trails", "  y          Cycle themes      b          Kitty / Braille", "  f          Focus mode        :          Command prompt", "  q          Quit              ?          This guide", "", "  Commands: mode brians-brain · rule B36/S23 · advance 1048576", "  density 0.3 · seed 42 · random · wrap · bounds 160x100", "  theme neon · pattern gosper · load https://… · save scene.rle", "", "  Eight automata: life, brians-brain, langtons-ant, larger-than-life,", "  faders, forest-fire, wildfire, belousov.", "", "  Fast-forward uses HashLife in Life mode, without wrapping/trails.", "  Press any key to return."}, "\n")
	} else if m.palette {
		var b strings.Builder
		b.WriteString(accent.Render("  PATTERN LIBRARY") + "\n\n")
		for i, p := range life.Presets {
			line := fmt.Sprintf("  %-14s %s", p.Name, p.Description)
			if i == m.selected {
				line = accent.Render("› " + line[2:])
			}
			b.WriteString(line + "\n")
		}
		b.WriteString("\n  ↑ / ↓ choose · enter plant · esc close")
		content = b.String()
	} else {
		var cursor *life.Point
		if m.showCursor {
			cursor = &m.cursor
		}
		board := ""
		if m.kittyActive() {
			board = strings.Repeat(strings.Repeat(" ", cols)+"\n", max(0, rows-1)) + strings.Repeat(" ", cols)
		} else {
			board = render.Braille(m.s, cols, rows, cursor)
		}
		if m.s.Minimap && !m.s.Focus && m.width >= 110 {
			side := accent.Render("  OVERVIEW") + "\n  " + strings.ReplaceAll(render.Minimap(m.s, 23, min(10, max(1, rows-6))), "\n", "\n  ") + fmt.Sprintf("\n\n  camera %.0f, %.0f\n  scale  %.3g\n  chunks %d\n  cache  %d", m.s.Camera.X, m.s.Camera.Y, m.s.Camera.Zoom, m.s.World.Metrics.Chunks, m.s.World.Metrics.Cache)
			board = lipgloss.JoinHorizontal(lipgloss.Top, board, muted.Render(side))
		}
		if m.s.Focus {
			content = board
		} else {
			state := "PAUSED"
			if m.s.Running {
				state = "RUNNING"
			}
			backend := "BRAILLE"
			if m.kittyActive() {
				backend = "KITTY"
			}
			header := accent.Render("  hiLife") + muted.Render("  /  "+strings.ToUpper(m.s.Name)) + "    " + accent.Render(state) + muted.Render(fmt.Sprintf("  %s · %s · %s", m.s.World.Rule.Text, m.s.World.Mode, backend))
			stats := muted.Render(fmt.Sprintf("  GEN %d    ALIVE %d    +%d / −%d    %d gen/s    %.2fms", m.s.World.Metrics.Generation, len(m.s.World.Cells), m.s.World.Metrics.Births, m.s.World.Metrics.Deaths, m.s.TPS, m.s.StepMS))
			bindings := []key.Binding{key.NewBinding(key.WithKeys("space"), key.WithHelp("space", "run")), key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "patterns")), key.NewBinding(key.WithKeys("+"), key.WithHelp("+/−", "zoom")), key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")), key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit"))}
			footer := m.helper.ShortHelpView(bindings)
			if m.notice != "" {
				footer = accent.Render(m.notice)
			}
			if m.loading {
				footer = "Loading pattern…"
			}
			if m.prompt != "" {
				footer = m.input.View()
			}
			content = header + "\n" + stats + "\n" + board + "\n" + muted.Render("  "+strings.Repeat("─", max(0, m.width-4))) + "\n  " + footer
		}
	}
	lines := strings.Split(content, "\n")
	if len(lines) > m.height {
		lines = lines[:m.height]
	}
	for i, line := range lines {
		lines[i] = ansi.Truncate(line, m.width, "")
	}
	v := tea.NewView(strings.Join(lines, "\n"))
	v.AltScreen = true
	v.MouseMode = tea.MouseModeAllMotion
	v.BackgroundColor = palette.Bg
	v.WindowTitle = "hilife · cellular automata"
	return v
}
func Run(s *app.Session, backend string) error {
	m := New(s, backend)
	defer func() {
		if m.kitty {
			fmt.Print(render.Passthrough(render.DeleteKitty, m.tmux))
		}
		if m.pixelMouse {
			fmt.Print("\x1b[?1016l")
		}
	}()
	_, err := tea.NewProgram(m).Run()
	return err
}
