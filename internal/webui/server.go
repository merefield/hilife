package webui

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"strings"
	"sync"
	"time"

	"hilife/internal/app"
	"hilife/internal/life"
)

//go:embed all:dist
var assets embed.FS

type Server struct {
	mu      sync.Mutex
	session *app.Session
}

func New(s *app.Session) *Server { return &Server{session: s} }
func (s *Server) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Second / 60)
	defer ticker.Stop()
	last := time.Now()
	for {
		select {
		case now := <-ticker.C:
			s.mu.Lock()
			s.session.Tick(min(now.Sub(last), time.Second/4))
			s.mu.Unlock()
			last = now
		case <-ctx.Done():
			return
		}
	}
}
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(v)
}
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/config", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"presets": life.Presets, "modes": life.Modes, "themes": []string{"mint", "neon", "amber", "sunset", "mono"}})
	})
	mux.HandleFunc("GET /api/state", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		writeJSON(w, s.session.Snapshot())
	})
	mux.HandleFunc("POST /api/action", func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" && origin != "http://"+r.Host && origin != "https://"+r.Host {
			http.Error(w, "cross-origin actions are disabled", http.StatusForbidden)
			return
		}
		var a app.Action
		r.Body = http.MaxBytesReader(w, r.Body, life.MaxPatternBytes+1024)
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&a); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		if a.Type == "source" {
			if !strings.HasPrefix(a.Value, "https://") && !strings.HasPrefix(a.Value, "http://") {
				http.Error(w, "web imports accept HTTP(S) URLs or uploaded pattern text", 400)
				return
			}
			text, err := life.ReadSource(r.Context(), a.Value)
			if err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
			a = app.Action{Type: "load", Text: text}
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		if err := s.session.Do(a); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		writeJSON(w, s.session.Snapshot())
	})
	mux.HandleFunc("GET /api/export", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		text := s.session.World.ExportJSON()
		format := "json"
		if r.URL.Query().Get("format") == "rle" {
			var err error
			text, err = s.session.World.ExportRLE()
			if err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
			format = "rle"
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="hilife.`+format+`"`)
		fmt.Fprint(w, text)
	})
	mux.HandleFunc("GET /api/events", func(w http.ResponseWriter, r *http.Request) {
		f, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unavailable", 500)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("X-Accel-Buffering", "no")
		ticker := time.NewTicker(time.Second / 20)
		defer ticker.Stop()
		last := ^uint64(0)
		for {
			s.mu.Lock()
			rev := s.session.Revision
			var payload []byte
			if rev != last {
				payload, _ = json.Marshal(s.session.Snapshot())
			}
			s.mu.Unlock()
			if payload != nil {
				http.NewResponseController(w).SetWriteDeadline(time.Now().Add(10 * time.Second))
				if _, err := fmt.Fprintf(w, "data: %s\n\n", payload); err != nil {
					return
				}
				f.Flush()
				last = rev
			}
			select {
			case <-ticker.C:
			case <-r.Context().Done():
				return
			}
		}
	})
	static, _ := fs.Sub(assets, "dist")
	files := http.FileServer(http.FS(static))
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if _, err := fs.Stat(static, "index.html"); err != nil {
			http.Error(w, "Web assets are missing. Run make web, then rebuild hilife.", 503)
			return
		}
		files.ServeHTTP(w, r)
	})
	return mux
}
