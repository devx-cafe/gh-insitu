package ui_test

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/devx-cafe/gh-insitu/internal/ui"
)

// ─── Local formatter ──────────────────────────────────────────────────────────

func TestLocalFormatter_WaveStart_PrintsName(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "")
	var buf bytes.Buffer
	f := ui.NewFormatter(&buf)

	f.WaveStart("w1", "My Wave")

	if !strings.Contains(buf.String(), "My Wave") {
		t.Errorf("WaveStart output %q does not contain wave name", buf.String())
	}
}

func TestLocalFormatter_WaveStart_EmptyName_NoPrint(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "")
	var buf bytes.Buffer
	f := ui.NewFormatter(&buf)

	f.WaveStart("w1", "")

	if buf.Len() != 0 {
		t.Errorf("WaveStart with empty name wrote %q, want nothing", buf.String())
	}
}

func TestLocalFormatter_WaveEnd_Passed_PrintsNewline(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "")
	var buf bytes.Buffer
	f := ui.NewFormatter(&buf)

	f.WaveEnd("w1", true)

	if buf.Len() == 0 {
		t.Error("WaveEnd(passed=true) wrote nothing, want newline")
	}
}

func TestLocalFormatter_WaveEnd_Failed_NoPrint(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "")
	var buf bytes.Buffer
	f := ui.NewFormatter(&buf)

	f.WaveEnd("w1", false)

	if buf.Len() != 0 {
		t.Errorf("WaveEnd(passed=false) wrote %q, want nothing", buf.String())
	}
}

func TestLocalFormatter_CheckStart_IsNoop(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "")
	var buf bytes.Buffer
	f := ui.NewFormatter(&buf)

	f.CheckStart("id1", "My Check")

	if buf.Len() != 0 {
		t.Errorf("CheckStart wrote %q, want nothing (no-op in local mode)", buf.String())
	}
}

func TestLocalFormatter_CheckEnd_Passed(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "")
	var buf bytes.Buffer
	f := ui.NewFormatter(&buf)

	f.CheckEnd("id1", "Build", true, nil, 500*time.Millisecond)
	out := buf.String()

	if !strings.Contains(out, "✅") {
		t.Errorf("CheckEnd passed output %q missing ✅", out)
	}
	if !strings.Contains(out, "Build") {
		t.Errorf("CheckEnd passed output %q missing check name", out)
	}
	if !strings.Contains(out, "0.5s") {
		t.Errorf("CheckEnd passed output %q missing duration", out)
	}
}

func TestLocalFormatter_CheckEnd_Failed_WithOutput(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "")
	var buf bytes.Buffer
	f := ui.NewFormatter(&buf)

	f.CheckEnd("id1", "Lint", false, []byte("error: something wrong"), 200*time.Millisecond)
	out := buf.String()

	if !strings.Contains(out, "❌") {
		t.Errorf("CheckEnd failed output %q missing ❌", out)
	}
	if !strings.Contains(out, "error: something wrong") {
		t.Errorf("CheckEnd failed output %q missing command output", out)
	}
}

func TestLocalFormatter_CheckEnd_Failed_NoOutput(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "")
	var buf bytes.Buffer
	f := ui.NewFormatter(&buf)

	f.CheckEnd("id1", "Lint", false, nil, 100*time.Millisecond)
	out := buf.String()

	if !strings.Contains(out, "❌") {
		t.Errorf("CheckEnd failed output %q missing ❌", out)
	}
}

func TestLocalFormatter_PrintSummary_AllPassed(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "")
	var buf bytes.Buffer
	f := ui.NewFormatter(&buf)

	f.PrintSummary(true)
	out := buf.String()

	if !strings.Contains(out, "✅") {
		t.Errorf("PrintSummary(true) output %q missing ✅", out)
	}
}

func TestLocalFormatter_PrintSummary_SomeFailed(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "")
	var buf bytes.Buffer
	f := ui.NewFormatter(&buf)

	f.PrintSummary(false)
	out := buf.String()

	if !strings.Contains(out, "❌") {
		t.Errorf("PrintSummary(false) output %q missing ❌", out)
	}
	if !strings.Contains(out, "Fix") {
		t.Errorf("PrintSummary(false) output %q missing fix hint", out)
	}
}

// ─── CI formatter ─────────────────────────────────────────────────────────────

func TestCIFormatter_WaveStart_UsesGroup(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "true")
	var buf bytes.Buffer
	f := ui.NewFormatter(&buf)

	f.WaveStart("w1", "CI Wave")
	out := buf.String()

	if !strings.Contains(out, "::group::") {
		t.Errorf("WaveStart CI output %q missing ::group::", out)
	}
	if !strings.Contains(out, "CI Wave") {
		t.Errorf("WaveStart CI output %q missing wave name", out)
	}
}

func TestCIFormatter_WaveEnd_UsesEndgroup(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "true")
	var buf bytes.Buffer
	f := ui.NewFormatter(&buf)

	f.WaveEnd("w1", true)
	f.WaveEnd("w1", false)
	out := buf.String()

	if count := strings.Count(out, "::endgroup::"); count != 2 {
		t.Errorf("WaveEnd CI output %q: ::endgroup:: count = %d, want 2", out, count)
	}
}

func TestCIFormatter_CheckStart_PrintsHourglass(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "true")
	var buf bytes.Buffer
	f := ui.NewFormatter(&buf)

	f.CheckStart("id1", "Build")
	out := buf.String()

	if !strings.Contains(out, "⏳") {
		t.Errorf("CI CheckStart output %q missing ⏳", out)
	}
	if !strings.Contains(out, "Build") {
		t.Errorf("CI CheckStart output %q missing check name", out)
	}
}

func TestCIFormatter_CheckEnd_Passed(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "true")
	var buf bytes.Buffer
	f := ui.NewFormatter(&buf)

	f.CheckEnd("id1", "Build", true, []byte("build ok"), time.Second)
	out := buf.String()

	if !strings.Contains(out, "✅") {
		t.Errorf("CI CheckEnd passed output %q missing ✅", out)
	}
	if !strings.Contains(out, "build ok") {
		t.Errorf("CI CheckEnd passed output %q missing command output", out)
	}
}

func TestCIFormatter_CheckEnd_Failed(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "true")
	var buf bytes.Buffer
	f := ui.NewFormatter(&buf)

	f.CheckEnd("id1", "Lint", false, []byte("lint failed"), time.Second)
	out := buf.String()

	if !strings.Contains(out, "❌") {
		t.Errorf("CI CheckEnd failed output %q missing ❌", out)
	}
}

func TestCIFormatter_PrintSummary_AllPassed(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "true")
	var buf bytes.Buffer
	f := ui.NewFormatter(&buf)

	f.PrintSummary(true)
	out := buf.String()

	if !strings.Contains(out, "✅") {
		t.Errorf("CI PrintSummary(true) output %q missing ✅", out)
	}
}

func TestCIFormatter_PrintSummary_SomeFailed(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "true")
	var buf bytes.Buffer
	f := ui.NewFormatter(&buf)

	f.PrintSummary(false)
	out := buf.String()

	if !strings.Contains(out, "❌") {
		t.Errorf("CI PrintSummary(false) output %q missing ❌", out)
	}
}

func TestCIFormatter_AppendsSummary(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "true")

	tmp, err := os.CreateTemp(t.TempDir(), "summary*.md")
	if err != nil {
		t.Fatalf("failed to create temp summary file: %v", err)
	}
	_ = tmp.Close()
	t.Setenv("GITHUB_STEP_SUMMARY", tmp.Name())

	var buf bytes.Buffer
	f := ui.NewFormatter(&buf)

	f.CheckEnd("id1", "Build", true, nil, time.Second)
	f.CheckEnd("id2", "Lint", false, []byte("bad code"), time.Second)

	data, err := os.ReadFile(tmp.Name())
	if err != nil {
		t.Fatalf("failed to read summary file: %v", err)
	}
	summary := string(data)

	if !strings.Contains(summary, "✅") {
		t.Errorf("summary %q missing ✅ for passed check", summary)
	}
	if !strings.Contains(summary, "❌") {
		t.Errorf("summary %q missing ❌ for failed check", summary)
	}
	if !strings.Contains(summary, "bad code") {
		t.Errorf("summary %q missing failure output", summary)
	}
}

func TestCIFormatter_Summary_SkippedWhenNoPath(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "true")
	t.Setenv("GITHUB_STEP_SUMMARY", "") // no summary file

	var buf bytes.Buffer
	f := ui.NewFormatter(&buf)

	// Should not panic even with no summary path
	f.CheckEnd("id1", "Build", true, nil, time.Second)
}
