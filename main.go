package main

import (
	"context"
	"flag"
	"fmt"
	"hilife/internal/app"
	"hilife/internal/life"
	"hilife/internal/tui"
	"hilife/internal/webui"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "hilife:", err)
		os.Exit(1)
	}
}
func run(args []string) error {
	web := false
	if len(args) > 0 && args[0] == "web" {
		web = true
		args = args[1:]
	}
	f := flag.NewFlagSet("hilife", flag.ContinueOnError)
	pattern := f.String("pattern", "gosper", "built-in pattern, file path, or HTTP(S) URL")
	renderer := f.String("renderer", "auto", "auto, kitty, or braille")
	rule := f.String("rule", "", "Life-like B/S rule override")
	mode := f.String("mode", "life", "cellular automaton (see --list)")
	seed := f.Uint64("seed", 42, "random seed")
	density := f.Float64("density", .28, "random density, 0–1")
	fps := f.Int("fps", 20, "generations/sec, 0 starts paused")
	play := f.Bool("play", false, "start running")
	theme := f.String("theme", "mint", "mint, neon, amber, sunset, mono")
	trails := f.Bool("trails", false, "show fading cell trails")
	wrap := f.Bool("wrap", false, "wrap around a bounded board")
	width := f.Int("width", 160, "bounded/random board width")
	height := f.Int("height", 96, "bounded/random board height")
	focus := f.Bool("focus", false, "fullscreen board without controls")
	listen := f.String("listen", "127.0.0.1:8787", "web server address")
	open := f.Bool("open", false, "open web interface in browser")
	list := f.Bool("list", false, "list patterns and automata")
	headless := f.Uint64("headless", 0, "advance generations, print JSON, and exit")
	version := f.Bool("version", false, "show version")
	f.Usage = func() {
		fmt.Fprint(f.Output(), "hilife — high-resolution cellular automata\n\nUsage: hilife [options] [pattern-file-or-url]\n       hilife web [options]\n\nTerminal: space run · p patterns · ? help · : commands\n")
		f.PrintDefaults()
	}
	if err := f.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	if *version {
		fmt.Println("hilife 0.1.0 ·", runtime.Version())
		return nil
	}
	if *list {
		for _, p := range life.Presets {
			fmt.Printf("%-14s %s\n", p.Name, p.Description)
		}
		fmt.Println("\nAutomata:", strings.Join(life.Modes, ", "))
		return nil
	}
	if f.NArg() > 0 {
		*pattern = f.Arg(0)
	}
	if *renderer != "auto" && *renderer != "kitty" && *renderer != "braille" {
		return fmt.Errorf("renderer must be auto, kitty, or braille")
	}
	if *density < 0 || *density > 1 {
		return fmt.Errorf("density must be 0–1")
	}
	if *width < 4 || *height < 4 || *width > 2000 || *height > 2000 {
		return fmt.Errorf("board dimensions must be 4–2000")
	}
	s, err := app.New(*seed)
	if err != nil {
		return err
	}
	s.Seed = *seed
	s.Density = *density
	s.TPS = max(1, min(1000, *fps))
	s.Running = *play && *fps > 0
	s.Focus = *focus
	if *pattern == "random" {
		s.World.Random(*seed, *density, -*width/2, -*height/2, *width, *height)
		s.Name = "random"
	} else if err := s.World.LoadPreset(*pattern); err != nil {
		text, e := life.ReadSource(context.Background(), *pattern)
		if e != nil {
			return e
		}
		if e = s.World.LoadText(text); e != nil {
			return e
		}
		s.Name = *pattern
	} else {
		s.Name = *pattern
	}
	if *rule != "" {
		if err = s.World.SetRule(*rule); err != nil {
			return err
		}
	}
	if err = s.Do(app.Action{Type: "mode", Value: *mode}); err != nil {
		return err
	}
	if err = s.Do(app.Action{Type: "theme", Value: *theme}); err != nil {
		return err
	}
	if err = s.Do(app.Action{Type: "bounds", X: *width, Y: *height}); err != nil {
		return err
	}
	if *wrap {
		s.Do(app.Action{Type: "wrap"})
	}
	s.World.ShowTrails = *trails
	s.Center()
	s.Remember()
	s.History = nil
	if *headless > 0 {
		if err = s.World.Advance(*headless); err != nil {
			return err
		}
		fmt.Print(s.World.ExportJSON())
		return nil
	}
	if !web {
		return tui.Run(s, *renderer)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	service := webui.New(s)
	go service.Run(ctx)
	listener, err := net.Listen("tcp", *listen)
	if err != nil {
		return err
	}
	server := &http.Server{Handler: service.Handler(), ReadHeaderTimeout: 5 * time.Second}
	url := "http://" + listener.Addr().String()
	fmt.Println("hilife is growing at", url)
	if *open {
		if err := openBrowser(url); err != nil {
			fmt.Fprintln(os.Stderr, "Open", url, "in your browser:", err)
		}
	}
	go func() {
		<-ctx.Done()
		stop, c := context.WithTimeout(context.Background(), 3*time.Second)
		defer c()
		server.Shutdown(stop)
	}()
	err = server.Serve(listener)
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}
func openBrowser(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}
