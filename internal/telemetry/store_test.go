package telemetry

import (
	"path/filepath"
	"testing"
	"time"
)

func TestStoreRecordAndScore(t *testing.T) {
	dir := t.TempDir()
	s, err := NewStoreAt(filepath.Join(dir, "sub"))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RecordCall("openai", "gpt-4o", 1000, 100, 0.001, true, 0); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordCall("openai", "gpt-4o", 3000, 200, 0.002, true, 1); err != nil {
		t.Fatal(err)
	}
	m, ok := s.MetricFor("openai", "gpt-4o")
	if !ok {
		t.Fatal("metric missing")
	}
	if m.Calls != 2 || m.Successes != 2 {
		t.Fatalf("unexpected metric: %+v", m)
	}
	if m.Score() <= 0 || m.Score() > 1 {
		t.Fatalf("score out of range: %f", m.Score())
	}

	if err := s.RecordEvent(Event{Type: "test", OK: true, Detail: "hello"}); err != nil {
		t.Fatal(err)
	}
	events := s.Events()
	if len(events) != 1 || events[0].Type != "test" {
		t.Fatalf("unexpected events: %+v", events)
	}

	// Reload persistence.
	s2, err := NewStoreAt(filepath.Join(dir, "sub"))
	if err != nil {
		t.Fatal(err)
	}
	m2, ok := s2.MetricFor("openai", "gpt-4o")
	if !ok || m2.Calls != 2 {
		t.Fatalf("persistence failed: %+v", m2)
	}
}

func TestScoreWithFailures(t *testing.T) {
	s, _ := NewStoreAt(filepath.Join(t.TempDir(), "t"))
	s.RecordCall("x", "y", 100, 10, 0, true, 0)
	s.RecordCall("x", "y", 100, 10, 0, false, 2)
	m, _ := s.MetricFor("x", "y")
	if m.Score() >= 0.99 {
		t.Fatalf("expected lower score with failures, got %f", m.Score())
	}
}

func TestEventTimeStamped(t *testing.T) {
	s, _ := NewStoreAt(filepath.Join(t.TempDir(), "t"))
	before := time.Now()
	s.RecordEvent(Event{Type: "t"})
	after := time.Now()
	evs := s.Events()
	if len(evs) != 1 {
		t.Fatal("event missing")
	}
	if evs[0].Time.Before(before) || evs[0].Time.After(after) {
		t.Fatalf("event time not stamped: %v", evs[0].Time)
	}
}
