// Package ui provides output formatting for insitu's local and CI environments.
package ui

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// Formatter writes progress and result messages for waves and checks.
type Formatter interface {
	WaveStart(id, name string)
	WaveEnd(id string, passed bool)
	CheckStart(id, displayName string)
	CheckEnd(id, displayName string, passed bool, output []byte, duration time.Duration)
	PrintSummary(allPassed bool)
}

// NewFormatter returns the appropriate Formatter for the current environment.
// When GITHUB_ACTIONS is set to "true" a CI-optimised formatter is returned;
// otherwise a plain text formatter suitable for local use is returned.
//
// verbose controls whether command output is printed for passing checks as well
// as failing ones.  When false (the default for local runs), output is shown
// only for failed checks.  When true (the default inside GitHub Actions),
// output is always shown.
func NewFormatter(out io.Writer, verbose bool) Formatter {
	if os.Getenv("GITHUB_ACTIONS") == "true" {
		return &ciFormatter{out: out, summaryPath: os.Getenv("GITHUB_STEP_SUMMARY"), verbose: verbose}
	}
	return &localFormatter{out: out, verbose: verbose}
}

// ow is a write helper that silently absorbs the (n, err) pair returned by
// fmt.Fprintf / fmt.Fprintln.  Write errors to stdout/stderr in a CLI are
// unrecoverable (e.g. SIGPIPE), so the caller cannot meaningfully act on them.
func ow(n int, err error) { _, _ = n, err }

// ─── Local formatter ─────────────────────────────────────────────────────────

type localFormatter struct {
	out     io.Writer
	verbose bool
}

func (f *localFormatter) WaveStart(_, name string) {
	if name == "" {
		return
	}
	ow(fmt.Fprintf(f.out, "\n🔍 %s\n", name))
}

func (f *localFormatter) WaveEnd(_ string, passed bool) {
	if passed {
		ow(fmt.Fprintln(f.out))
	}
}

func (f *localFormatter) CheckStart(_, _ string) {
	// Local output is results-only: ⏳ is not printed because in parallel waves
	// the hourglass lines interleave with the ✅/❌ result lines.
}

func (f *localFormatter) CheckEnd(_, displayName string, passed bool, output []byte, duration time.Duration) {
	icon := "✅"
	if !passed {
		icon = "❌"
	}
	ow(fmt.Fprintf(f.out, "   %s %s (%.1fs)\n", icon, displayName, duration.Seconds()))
	if (f.verbose || !passed) && len(output) > 0 {
		ow(fmt.Fprintf(f.out, "\n%s\n", strings.TrimRight(string(output), "\n")))
	}
}

func (f *localFormatter) PrintSummary(allPassed bool) {
	if allPassed {
		ow(fmt.Fprintln(f.out, "\n✅ All checks passed"))
	} else {
		ow(fmt.Fprintln(f.out, "\n❌ Some checks failed"))
		ow(fmt.Fprintln(f.out, "👉 Fix the issues above before committing."))
	}
}

// ─── CI formatter ─────────────────────────────────────────────────────────────

type ciFormatter struct {
	out         io.Writer
	summaryPath string
	verbose     bool
}

func (f *ciFormatter) WaveStart(_, name string) {
	ow(fmt.Fprintf(f.out, "::group::%s\n", name))
}

func (f *ciFormatter) WaveEnd(_ string, _ bool) {
	ow(fmt.Fprintln(f.out, "::endgroup::"))
}

func (f *ciFormatter) CheckStart(_, displayName string) {
	ow(fmt.Fprintf(f.out, "⏳ %s\n", displayName))
}

func (f *ciFormatter) CheckEnd(_, displayName string, passed bool, output []byte, _ time.Duration) {
	icon := "✅"
	if !passed {
		icon = "❌"
	}
	ow(fmt.Fprintf(f.out, "%s %s\n", icon, displayName))

	if (f.verbose || !passed) && len(output) > 0 {
		ow(fmt.Fprintf(f.out, "%s\n", string(output)))
	}

	f.appendSummary(icon, displayName, output, passed)
}

func (f *ciFormatter) PrintSummary(allPassed bool) {
	if allPassed {
		ow(fmt.Fprintln(f.out, "✅ All checks passed"))
	} else {
		ow(fmt.Fprintln(f.out, "❌ Some checks failed"))
	}
}

func (f *ciFormatter) appendSummary(icon, displayName string, output []byte, passed bool) {
	if f.summaryPath == "" {
		return
	}
	fh, err := os.OpenFile(f.summaryPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer func() { _ = fh.Close() }()

	ow(fmt.Fprintf(fh, "%s %s\n", icon, displayName))
	if !passed && len(output) > 0 {
		ow(fmt.Fprintf(fh, "\n```\n%s\n```\n\n", strings.TrimRight(string(output), "\n")))
	}
}
