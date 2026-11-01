package ui

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestLiveEnabled(t *testing.T) {
	tests := []struct {
		name       string
		ci         string
		appendOnly bool
	}{
		{"non-file writer", "", false},
		{"append-only", "", true},
		{"github actions", "true", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GITHUB_ACTIONS", tt.ci)
			if LiveEnabled(&bytes.Buffer{}, tt.appendOnly) {
				t.Error("expected live display disabled")
			}
		})
	}
}

func TestNewFormatterWithModeFallback(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "")
	if _, ok := NewFormatterWithMode(&bytes.Buffer{}, false, false).(*localFormatter); !ok {
		t.Error("non-TTY should fall back to local append-only formatter")
	}
}

func TestLiveFormatterOrdering(t *testing.T) {
	var buf bytes.Buffer
	f := newLiveFormatter(&buf, false)
	f.WaveStart("w", "Wave")
	f.WavePlan("w", []PlannedCheck{{"a", "A"}, {"b", "B"}, {"c", "C"}})
	if got := buf.String(); !strings.Contains(got, "⏳ A") || !strings.Contains(got, "⏳ C") {
		t.Fatalf("initial display missing hourglasses: %q", got)
	}
	f.CheckEnd("c", "C", true, nil, time.Second)
	f.CheckEnd("a", "A", false, []byte("boom\n"), 2*time.Second)

	f.mu.Lock()
	var out bytes.Buffer
	f.out = &out
	f.drawn = 0
	f.render()
	f.mu.Unlock()
	want := "   ✅ C (1.0s)\n   ❌ A (2.0s)\n   ⏳ B\n"
	if out.String() != want {
		t.Errorf("render = %q, want %q", out.String(), want)
	}
	if !strings.Contains(buf.String(), "\x1b[3A") {
		t.Error("expected cursor-up redraw sequence")
	}

	f.WaveEnd("w", false)
	if strings.Count(out.String(), "boom") != 1 {
		t.Errorf("failure output should appear once: %q", out.String())
	}
}

func TestLiveFormatterVerbose(t *testing.T) {
	tests := []struct {
		verbose bool
		want    int
	}{{false, 0}, {true, 1}}
	for _, tt := range tests {
		var buf bytes.Buffer
		f := newLiveFormatter(&buf, tt.verbose)
		f.WavePlan("w", []PlannedCheck{{"a", "A"}})
		f.CheckEnd("a", "A", true, []byte("hello"), time.Second)
		f.WaveEnd("w", true)
		if got := strings.Count(buf.String(), "hello"); got != tt.want {
			t.Errorf("verbose=%v: output count %d, want %d", tt.verbose, got, tt.want)
		}
	}
}
