package webui

import (
	"encoding/json"
	"hilife/internal/app"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWebActionsAndExport(t *testing.T) {
	s, _ := app.New(1)
	h := New(s).Handler()
	for _, body := range []string{`{"type":"pattern","value":"glider"}`, `{"type":"advance","number":64}`} {
		r := httptest.NewRequest("POST", "/api/action", strings.NewReader(body))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	r := httptest.NewRequest("GET", "/api/state", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	var state app.Snapshot
	if e := json.Unmarshal(w.Body.Bytes(), &state); e != nil {
		t.Fatal(e)
	}
	if state.Generation != 64 || state.Population != 5 {
		t.Fatal(state.Generation, state.Population)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/export?format=rle", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), "B3/S23") {
		t.Fatal(w.Body.String())
	}
}
func TestRejectCrossOriginAndLocalFileImport(t *testing.T) {
	s, _ := app.New(1)
	h := New(s).Handler()
	r := httptest.NewRequest("POST", "/api/action", strings.NewReader(`{"type":"clear"}`))
	r.Header.Set("Origin", "https://example.org")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal(w.Code)
	}
	r = httptest.NewRequest("POST", "/api/action", strings.NewReader(`{"type":"source","value":"/etc/passwd"}`))
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 400 {
		t.Fatal(w.Code)
	}
}
