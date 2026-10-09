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
	r := NewRollout(p, testGate, f)
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
	r = Resume(p, testGate, f, events, 15*time.Minute)
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
	r := Resume(testPlan(t), testGate, NewFleet([]int{1, 1, 1}, 3, 1), events, time.Hour)
	if r.State != RolledBack {
		t.Fatalf("state %v", r.State)
	}
}

func TestCoveredAfterResume(t *testing.T) {
	f := NewFleet([]int{1, 1, 2}, 3, 1)
	events := []Event{{0, 0, "deploy", ""}, {time.Minute, 0, "healthy", ""}, {time.Minute, 1, "deploy", ""}}
	r := Resume(testPlan(t), testGate, f, events, time.Hour)
	c := r.Covered()
	if len(c) != 6 { // stage 2 is all of tier 3: 2 DCs of 3 servers
		t.Fatalf("covered %d servers, want 6", len(c))
	}
	for s, share := range c {
		if s.DC.Tier != 3 || share != 1 {
			t.Fatalf("covered %v at %v", s.DC.Name, share)
		}
	}
	r = Resume(testPlan(t), testGate, f, append(events, Event{2 * time.Minute, 1, "revert", ""}), time.Hour)
	if len(r.Covered()) != 0 {
		t.Fatal("reverted rollout still covers servers")
	}
}

func TestJournalWriteFailureHalts(t *testing.T) {
	j, _, err := OpenJournal(filepath.Join(t.TempDir(), "j"))
	if err != nil {
		t.Fatal(err)
	}
	j.Close() // every write now fails
	r := NewRollout(testPlan(t), testGate, NewFleet([]int{1, 1, 1}, 3, 1))
	r.Journal = j
	paged := false
	r.Page = func(Event) { paged = true }
	if servers, _ := r.Next(0); servers != nil || r.State != Halted || !paged {
		t.Fatalf("deployed %d servers without a journal, state %v", len(servers), r.State)
	}
}

func TestBaselineSurvivesResume(t *testing.T) {
	path := filepath.Join(t.TempDir(), "j")
	j, _, _ := OpenJournal(path)
	r := NewRollout(testPlan(t), testGate, NewFleet([]int{1, 1, 1}, 3, 1))
	r.Journal = j
	r.SetBaseline(0, map[string]Counts{"5xx": {3e6, 900}})
	r.Next(0)
	j.Close()
	_, events, err := OpenJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	r = Resume(testPlan(t), testGate, r.Fleet, events, time.Hour)
	if got := r.Baseline["5xx"]; got != (Counts{3e6, 900}) {
		t.Fatalf("baseline after resume: %+v", got)
	}
}

func TestBaselineJournalFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "j")
	j, _, _ := OpenJournal(path)
	r := NewRollout(testPlan(t), testGate, NewFleet([]int{1}, 1, 1))
	r.Journal = j
	r.SetBaseline(0, map[string]Counts{"5xx": {100, 1}})
	j.Close()
	b, _ := os.ReadFile(path)
	want := `{"at":0,"stage":-1,"kind":"baseline","what":"{\"5xx\":{\"requests\":100,\"failures\":1}}"}` + "\n"
	if string(b) != want {
		t.Fatalf("journal line:\n%s\nwant\n%s", b, want)
	}
}

func TestJournalLineMissingOnlyItsNewline(t *testing.T) {
	path := filepath.Join(t.TempDir(), "j")
	os.WriteFile(path, []byte(`{"at":1,"stage":0,"kind":"deploy","what":"a"}`), 0o644)
	j, events, err := OpenJournal(path)
	if err != nil || len(events) != 1 {
		t.Fatalf("%v %v", events, err)
	}
	if err := j.Append(Event{Kind: "healthy"}); err != nil {
		t.Fatal(err)
	}
	j.Close()
	_, events, err = OpenJournal(path)
	if err != nil || len(events) != 2 {
		t.Fatalf("reopened: %v %v", events, err)
	}
}
