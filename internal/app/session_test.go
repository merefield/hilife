package app

import (
	"testing"
	"time"
)

func TestUndoResetAndDeterministicRandom(t *testing.T) {
	s, _ := New(42)
	initial := s.World.ExportJSON()
	s.Do(Action{Type: "step"})
	s.Do(Action{Type: "undo"})
	if s.World.ExportJSON() != initial {
		t.Fatal("undo differs")
	}
	s.Do(Action{Type: "random", X: 20, Y: 20})
	random := s.World.ExportJSON()
	s.Do(Action{Type: "random", X: 20, Y: 20})
	if s.World.ExportJSON() != random {
		t.Fatal("seed is not reproducible")
	}
	s.Do(Action{Type: "step"})
	s.Do(Action{Type: "reset"})
	if s.World.ExportJSON() != random {
		t.Fatal("reset differs")
	}
}
func TestSimulationClockAndFailureAtomicity(t *testing.T) {
	s, _ := New(1)
	s.TPS = 20
	s.Running = true
	s.Tick(100 * time.Millisecond)
	if s.World.Metrics.Generation != 2 {
		t.Fatal(s.World.Metrics.Generation)
	}
	before := s.World.ExportJSON()
	if e := s.Do(Action{Type: "load", Text: "invalid"}); e == nil {
		t.Fatal("invalid import accepted")
	}
	if s.World.ExportJSON() != before {
		t.Fatal("failed action changed world")
	}
}
