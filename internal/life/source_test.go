package life

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPatternURLDiscovery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/page" {
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`<a href="/glider.rle">download</a>`))
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("x = 3, y = 3\nbo$2bo$3o!"))
	}))
	defer server.Close()
	text, err := ReadSource(context.Background(), server.URL+"/page")
	if err != nil {
		t.Fatal(err)
	}
	w := world(t, "empty")
	if err = w.LoadText(text); err != nil {
		t.Fatal(err)
	}
	if len(w.Cells) != 5 {
		t.Fatal(len(w.Cells))
	}
}
func TestPatternReadLimit(t *testing.T) {
	if _, err := readLimited(strings.NewReader(strings.Repeat("x", MaxPatternBytes+1))); err == nil {
		t.Fatal("size limit not enforced")
	}
}
func TestStochasticReplayAndEmptyForest(t *testing.T) {
	w := world(t, "empty")
	w.Mode = "forest-fire"
	w.Step()
	if len(w.Cells) == 0 {
		t.Fatal("empty forest cannot regrow")
	}
	a := w.Clone()
	b := w.Clone()
	for range 5 {
		a.Step()
		b.Step()
	}
	if a.ExportJSON() != b.ExportJSON() {
		t.Fatal("seeded forest replay differs")
	}
}
