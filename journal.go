package colorollout

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// Journal is an append-only file of rollout events, one JSON object per
// line, synced to disk before the rollout acts on them. If the process
// running a rollout dies, the journal says which stage it reached and
// whether it had already reverted, so a new process can pick up from there
// instead of starting over or leaving a half-done release behind.
type Journal struct {
	f *os.File
}

// OpenJournal opens or creates a journal and returns the events already in
// it. A last line cut short by a crash is dropped; anything else that
// doesn't parse is an error, because a journal we can't trust shouldn't be
// guessed at.
func OpenJournal(path string) (*Journal, []Event, error) {
	b, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, nil, err
	}
	var events []Event
	lines := bytes.Split(b, []byte("\n"))
	for i, line := range lines {
		if len(line) == 0 {
			continue
		}
		var e Event
		if err := json.Unmarshal(line, &e); err != nil {
			if i == len(lines)-1 { // no newline after it: torn write
				b = b[:len(b)-len(line)]
				break
			}
			return nil, nil, fmt.Errorf("journal %s line %d: %w", path, i+1, err)
		}
		events = append(events, e)
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return nil, nil, err
	}
	if err := f.Truncate(int64(len(b))); err != nil {
		f.Close()
		return nil, nil, err
	}
	if _, err := f.Seek(0, 2); err != nil {
		f.Close()
		return nil, nil, err
	}
	return &Journal{f}, events, nil
}

// Append writes one event and syncs it.
func (j *Journal) Append(e Event) error {
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	w := bufio.NewWriter(j.f)
	w.Write(b)
	w.WriteByte('\n')
	if err := w.Flush(); err != nil {
		return err
	}
	return j.f.Sync()
}

// Close closes the file.
func (j *Journal) Close() error { return j.f.Close() }

// Resume rebuilds a rollout from its journal. A rollout that was mid-stage
// restarts that stage's soak at now: the counts behind it were in memory and
// are gone, so the stage has to earn its pass again. A crash costs time,
// never a check.
func Resume(p Plan, g Gate, f *Fleet, events []Event, now time.Duration) *Rollout {
	r := NewRollout(p, g, f)
	r.Events = append(r.Events, events...)
	for _, e := range events {
		switch e.Kind {
		case "baseline":
			if err := json.Unmarshal([]byte(e.What), &r.Baseline); err != nil {
				r.Baseline = nil // compare with the control only
			}
		case "deploy":
			r.Stage = e.Stage
		case "revert":
			r.State = RolledBack
		case "halt":
			r.State = Halted
		case "done":
			r.State = Done
		}
	}
	if r.Stage >= 0 {
		r.StageStart = now
		r.Canary, r.Control = map[string]Counts{}, map[string]Counts{}
		if r.State == Running {
			r.log(now, "resume", "picked up from the journal; soak starts again")
		}
	}
	return r
}
