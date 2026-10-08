package colorollout

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestJournalResume(t *testing.T) {
	path := filepath.Join(t.TempDir(), "edge.journal")
	j, events, err := OpenJournal(path)
	if err != nil || len(events) != 0 {
		t.Fatalf("new journal: %v, %d events", err, len(events))
	}
	p := testPlan(t)
	f := NewFleet([]int{1, 1, 1}, 3, 1)
	r := NewRollout(p, DefaultGate, f)
	r.Journal = j
	ok := Sample{New: map[string]Counts{"5xx": {20000, 10}}, Old: map[string]Counts{"5xx": {20000, 10}}}
	r.Next(0)
	r.Observe(10*time.Minute, ok)
	r.Next(10 * time.Minute)
	j.Close()

	// The process dies here, halfway through writing a line.
	fh, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	fh.WriteString(`{"at":7200000000000,"stage":1,"ki`)
	fh.Close()

	j, events, err = OpenJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	if len(events) != 3 {
		t.Fatalf("read %d events, want 3", len(events))
	}
	r = Resume(p, DefaultGate, f, events, 15*time.Minute)
	r.Journal = j
	if r.Stage != 1 || r.State != Running || r.StageStart != 15*time.Minute {
		t.Fatalf("resumed at stage %d, %v, start %v", r.Stage, r.State, r.StageStart)
	}
	if d := r.Observe(20*time.Minute, ok); d != Wait {
		t.Fatalf("passed a stage on soak from before the crash: %v", d)
	}
	if d := r.Observe(25*time.Minute, ok); d != Continue {
		t.Fatalf("after a full soak: %v", d)
	}
	_, events, _ = OpenJournal(path)
	if last := events[len(events)-1]; last.Kind != "healthy" {
		t.Fatalf("last journal entry %+v", last)
	}
}

func TestJournalRejectsCorruptMiddle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "j")
	os.WriteFile(path, []byte("{\"kind\":\"deploy\"}\nnot json\n{\"kind\":\"revert\"}\n"), 0o644)
	if _, _, err := OpenJournal(path); err == nil {
		t.Fatal("corrupt line in the middle accepted")
	}
}

func TestResumeAfterRevertStaysReverted(t *testing.T) {
	events := []Event{{0, 0, "deploy", ""}, {time.Minute, 0, "revert", "5xx"}}
	r := Resume(testPlan(t), DefaultGate, NewFleet([]int{1, 1, 1}, 3, 1), events, time.Hour)
	if r.State != RolledBack {
		t.Fatalf("state %v", r.State)
	}
}
