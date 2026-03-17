# **InSitu: Self-Contained Parallel Task Runner for GitHub CLI**

**InSitu** is a Go-based GitHub CLI extension designed to bring parity between local development and CI environments. It replaces complex YAML-based GitHub Action logic with a repository-native "Inventory and Wave" system.

## **1\. Core Philosophy**

- **Shift-Left Excellence:** If it runs in the CI, it runs locally via gh insitu.  
- **In Situ (In Place):** Validating the codebase within its natural environment (the repo) rather than a remote, sterile runner.  
- **Symmetric CI:** Identical check definitions for pre-commit hooks and GitHub Workflows.

## **2\. Configuration Schema (insitu.yml)**

The configuration uses a hierarchical approach: **Global Defaults \-\> Inventory Library \-\> Wave Execution**.

```yaml
\# .insitu.yml

\# 1\. Global Defaults (Inherited by all)  
defaults:  
  die-on-error: true  
  timeout: 5m  
  verbose: false

\# 2\. The Library of Actions (The 'What')  
inventory:  
  \- id: "cspell"  
    name: "Spell Check"  
    command: "cspell"

  \- id: "build"  
    name: "Build (Current OS/Arch)"  
    command: "make build"  
    timeout: 10m

  \- id: "coverage"  
    name: "Unit Test with Coverage"  
    command: "make coverage"

\# 3\. Execution Plan (The 'How' and 'When')  
waves:  
  \- id: "trunk-worthy"  
    name: "Static Analysis"  
    parallel: true  
    checks:  
      \- "cspell"  
      \- "markdownlint"  
      \- "prettier"  
      \- "build"

  \- id: "test"  
    name: "Post-Build Validation"  
    parallel: true  
    checks:  
      \- "coverage"
```

## **3\. Go Implementation Strategy**

### **A. Data Modeling & Overrides**

Use pointers in Go structs to handle the inheritance chain effectively. This allows the application to distinguish between a "zero value" (like an empty string) and a value that was intentionally omitted.

type Config struct {  
    Defaults  Defaults   \`yaml:"defaults"\`  
    Inventory \[\]Check    \`yaml:"inventory"\`  
    Waves     \[\]Wave     \`yaml:"waves"\`  
}

type Defaults struct {  
    DieOnError bool          \`yaml:"die-on-error"\`  
    Timeout    time.Duration \`yaml:"timeout"\`  
    Verbose    bool          \`yaml:"verbose"\`  
}

type Check struct {  
    ID          string         \`yaml:"id"\`  
    Name        string         \`yaml:"name"\`  
    Command     string         \`yaml:"command"\`  
    Timeout     \*time.Duration \`yaml:"timeout"\`      // Optional override  
    DieOnError  \*bool          \`yaml:"die-on-error"\` // Optional override  
}

type Wave struct {  
    ID         string   \`yaml:"id"\`  
    Name       string   \`yaml:"name"\`  
    Parallel   bool     \`yaml:"parallel"\`  
    Checks     \[\]string \`yaml:"checks"\` // IDs referencing the inventory  
}

### **B. Parallelism Engine**

- **Synchronization:** Use sync.WaitGroup to coordinate the completion of all checks within a single wave before proceeding to the next.  
- **Timeout Control:** Use context.WithTimeout per check, initialized by the inheritance-calculated timeout value.  
- **Output Management:** Do not stream directly to os.Stdout during parallel runs. Capture output in a bytes.Buffer for each goroutine and print the block only after the check completes or fails.

### **C. Context Awareness**

The tool detects its environment to adjust behavior:

- **Local Mode:** Uses TUIs (e.g., charmbracelet/bubbletea) for interactive progress bars and live status updates.  
- **CI Mode:** Detects GITHUB\_ACTIONS=true.  
  - Switches to ::group:: and ::endgroup:: logging for cleaner GitHub Action logs.  
  - Automatically populates GITHUB\_STEP\_SUMMARY.  
  - Utilizes the GitHub API (via cli/go-gh) to update Commit Statuses/Checks.

## **4\. Proposed CLI Commands**

- gh insitu init: Bootstrap the configuration file and install .git/hooks/pre-commit pointing to the run command.  
- gh insitu run \[wave-id\]: Execute specific waves. If no ID is provided, it executes all waves sequentially.  
- gh insitu plan: A dry-run mode that validates the YAML schema and lists the resolved execution order and command overrides.

## **5\. Transitioning from Bash to Go**

| Feature | Bash Prototype | Go Implementation |
| :---- | :---- | :---- |
| **Concurrency** | Background jobs & with wait | Goroutines with sync.WaitGroup |
| **Status Mapping** | External gh-set-status extension | Native integration using cli/go-gh |
| **Config** | Hardcoded script arrays/maps | Structured YAML via gopkg.in/yaml.v3 |
| **Parallel UI** | Manual tput cursor manipulation | charmbracelet/bubbletea & lipgloss |
| **Error Handling** | trap and exit codes | Context-driven cancellation & os/exec error wrapping |

## **6\. Project Structure (Go)**

```text
.  
├── cmd/  
│   └── insitu/  
│       └── main.go         \# CLI Entry point (spf13/cobra)  
├── internal/  
│   ├── config/             \# YAML parsing & inheritance logic  
│   ├── runner/             \# The parallel execution engine  
│   └── ui/                 \# TUI and CI-specific output formatters  
├── pkg/  
│   └── github/             \# GitHub API wrappers (Status & Summary)  
├── go.mod  
└── .insitu.yml             \# Self-hosting configuration  
```
