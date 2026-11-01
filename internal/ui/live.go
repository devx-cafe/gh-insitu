package ui

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// PlannedCheck identifies a check that is about to run in a wave.
type PlannedCheck struct {
	ID          string
	DisplayName string
}

// WavePlanner is implemented by formatters that want to know every check of a
// wave before any of them starts.
type WavePlanner interface {
	WavePlan(id string, checks []PlannedCheck)
}

// LiveEnabled reports whether the live (redrawing) wave display should be used.
// It is disabled when appendOnly is set, inside GitHub Actions, and whenever
// out is not an interactive terminal.
func LiveEnabled(out io.Writer, appendOnly bool) bool {
	if appendOnly || os.Getenv("GITHUB_ACTIONS") == "true" {
		return false
	}
	if t := os.Getenv("TERM"); t == "dumb" {
		return false
	}
	f, ok := out.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// NewFormatterWithMode is like NewFormatter but additionally selects the live
// wave display when LiveEnabled allows it.
func NewFormatterWithMode(out io.Writer, verbose, appendOnly bool) Formatter {
	if LiveEnabled(out, appendOnly) {
		return newLiveFormatter(out, verbose)
	}
	return NewFormatter(out, verbose)
}

type liveEntry struct {
	id, name string
	done     bool
	passed   bool
	duration time.Duration
	output   []byte
}

type liveFormatter struct {
	localFormatter
	mu       sync.Mutex
	planned  []*liveEntry
	finished []*liveEntry
	drawn    int
}

func newLiveFormatter(out io.Writer, verbose bool) *liveFormatter {
	return &liveFormatter{localFormatter: localFormatter{out: out, verbose: verbose}}
}

func (f *liveFormatter) WavePlan(_ string, checks []PlannedCheck) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.planned = make([]*liveEntry, 0, len(checks))
	f.finished = nil
	f.drawn = 0
	for _, c := range checks {
		f.planned = append(f.planned, &liveEntry{id: c.ID, name: c.DisplayName})
	}
	f.render()
}

func (f *liveFormatter) CheckStart(_, _ string) {}

func (f *liveFormatter) CheckEnd(id, displayName string, passed bool, output []byte, duration time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var entry *liveEntry
	for _, e := range f.planned {
		if !e.done && e.id == id {
			entry = e
			break
		}
	}
	if entry == nil { // not announced via WavePlan
		entry = &liveEntry{id: id, name: displayName}
		f.planned = append(f.planned, entry)
	}
	entry.done, entry.passed, entry.duration, entry.output = true, passed, duration, output
	f.finished = append(f.finished, entry)
	f.render()
}

func (f *liveFormatter) WaveEnd(_ string, passed bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, e := range f.finished {
		if (f.verbose || !e.passed) && len(e.output) > 0 {
			ow(fmt.Fprintf(f.out, "\n%s %s output:\n%s\n", icon(e.passed), e.name, strings.TrimRight(string(e.output), "\n")))
		}
	}
	f.planned, f.finished, f.drawn = nil, nil, 0
	if passed {
		ow(fmt.Fprintln(f.out))
	}
}

func icon(passed bool) string {
	if passed {
		return "✅"
	}
	return "❌"
}

// render redraws the status list: finished checks in completion order followed
// by unfinished ones in wave order.  Callers must hold f.mu.
func (f *liveFormatter) render() {
	var b strings.Builder
	if f.drawn > 0 {
		fmt.Fprintf(&b, "\x1b[%dA\r\x1b[J", f.drawn)
	}
	lines := 0
	for _, e := range f.finished {
		fmt.Fprintf(&b, "   %s %s (%.1fs)\n", icon(e.passed), e.name, e.duration.Seconds())
		lines++
	}
	for _, e := range f.planned {
		if !e.done {
			fmt.Fprintf(&b, "   ⏳ %s\n", e.name)
			lines++
		}
	}
	f.drawn = lines
	ow(fmt.Fprint(f.out, b.String()))
}
