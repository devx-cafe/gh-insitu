<!-- cspell:ignore gofmt golangci insitu mkdirs -->

# Contributing to gh-insitu

Thank you for your interest in contributing to gh-insitu! This document provides guidelines and information for developers.

## Developer Resources

### Required Reading

For comprehensive development guidelines, please read the following RAG (Retrieval-Augmented Generation) files:

- [`.github/copilot-instructions.md`](.github/copilot-instructions.md) - Project overview and code standards
- [`.github/instructions/go-standards.instructions.md`](.github/instructions/go-standards.instructions.md) - Detailed Go development standards
- [`.github/instructions/insitu-idea.instructions.md`](.github/instructions/insitu-idea.instructions.md) - InSitu design document

These files contain essential information about:

- Project structure and organization
- Coding standards and conventions
- Testing practices
- Build processes
- Security considerations

## Prerequisites

- **Go 1.21 or higher** - This project targets Go 1.21+
- **Make** - For build automation
- **golangci-lint v2** - Can be installed via `make install-lint`
- **Git** - Configured with `.githooks` for pre-commit checks

## Getting Started

1. **Fork and Clone**

   ```bash
   git clone https://github.com/YOUR_USERNAME/gh-insitu.git
   cd gh-insitu
   ```

2. **Configure Git Hooks**

   ```bash
   git config core.hooksPath .githooks
   ```

3. **Install Dependencies**

   ```bash
   make deps
   ```

4. **Verify Setup**

   ```bash
   make build
   make test
   make lint
   ```

## Development Workflow

1. **Create a Feature Branch**

   ```bash
   git checkout -b feature/your-feature-name
   ```

2. **Make Your Changes**
   - Follow the coding standards (see RAG files)
   - Write or update tests for your changes
   - Update documentation as needed

3. **Test Your Changes**

   ```bash
   # Run tests
   make test

   # Run tests with coverage
   make coverage

   # Run linter
   make lint

   # Format code
   make fmt

   # Run static analysis
   make vet
   ```

4. **Build and Verify**

   ```bash
   # Build for current platform
   make build

   # Test the binary
   ./insitu --help

   # Validate your .insitu.yml changes
   ./insitu plan

   # Run all checks (self-hosting)
   ./insitu run
   ```

5. **Commit Your Changes**
   - The pre-commit hook will automatically run checks
   - Ensure all checks pass before pushing

6. **Submit a Pull Request**
   - Push your branch to your fork
   - Create a pull request with a clear description
   - Link any related issues

## Project Structure

```txt
.
├── main.go                    # Application entry point
├── cmd/                       # Command implementations
│   ├── root.go               # Root command definition
│   ├── run.go                # 'run' command – execute waves
│   ├── plan.go               # 'plan' command – dry-run preview
│   ├── init.go               # 'init' command – bootstrap config
│   └── setstatus.go          # 'set-status' command – GitHub commit status
├── internal/
│   ├── config/               # YAML parsing, validation, inheritance
│   │   ├── config.go
│   │   ├── config_test.go
│   │   └── testdata/         # Test YAML fixtures
│   ├── runner/               # Parallel wave executor
│   │   ├── runner.go
│   │   └── runner_test.go
│   ├── ui/                   # Output formatters (local + CI)
│   │   └── formatter.go
│   └── github/               # GitHub API helpers (commit status)
│       └── status.go
├── .insitu.yml               # Self-hosting configuration
├── Makefile                  # Build automation
├── .golangci.yml             # Linter configuration (golangci-lint v2)
├── .githooks/                # Git hooks
│   └── pre-commit            # Pre-commit checks
└── .github/                  # GitHub configuration
    ├── copilot-instructions.md
    └── instructions/
        ├── go-standards.instructions.md
        └── insitu-idea.instructions.md
```

## The insitu.yml Configuration

The core configuration format for `gh insitu`:

```yaml
defaults:
  die-on-error: true # stop after a failing wave
  timeout: 5m # default per-check timeout
  verbose: false

inventory:
  - id: "build"
    name: "Build"
    command: "make build"
    timeout: 10m # optional per-check override

waves:
  - id: "static"
    name: "Static Analysis"
    parallel: true # run checks in this wave concurrently
    checks:
      - "build"
```

## Testing

### Test Organization

Tests follow the **Go community standard**: test files are placed alongside the code they test.

- Unit tests: `*_test.go` files in the same directory as the implementation
- Test fixtures: `testdata/` directories alongside test files
- Example: `internal/config/config.go` has tests in `internal/config/config_test.go`

### Testing Philosophy (Detroit School)

Tests use **stateful fixtures** rather than mocks:

- YAML test files in `testdata/` represent real configuration states
- Runner tests execute actual shell commands (`echo`, `sh -c 'exit 1'`, etc.)
- No interface mocking; use real objects with controlled inputs

### Writing Tests

- Use **table-driven tests** for multiple test cases
- Test both success and error paths
- Include edge cases
- Aim for high coverage on business logic

## Building

### Local Development Build

```bash
make build
```

This creates the `insitu` binary in the repository root.

### Multi-Platform Builds

```bash
make build-all
```

Builds for:

- Linux: amd64, arm64
- Darwin (macOS): amd64, arm64
- Windows: amd64, arm64

Binaries are placed in the `dist/` directory.

### Clean Build Artifacts

```bash
make clean
```

## Pre-commit Hooks

The repository uses pre-commit hooks to maintain code quality. When you commit, the following checks run automatically:

1. **Spell checking** (cspell)
2. **Markdown linting**
3. **Code formatting** (prettier, gofmt)
4. **Go vet** - Static analysis
5. **golangci-lint** - Comprehensive linting
6. **Go tests** - All unit tests
7. **Build verification** - Ensures code compiles

If any check fails, the commit is rejected. Fix the issues and try again.

## Coding Standards

### Go Programming

- **Follow Go idioms**: Use standard Go patterns and conventions
- **Format with gofmt**: Code must be formatted with `gofmt`
- **Use golangci-lint v2**: Address all linter warnings
- **Document exports**: All exported functions, types, and constants need documentation
- **Error handling**: Always check and wrap errors with context

### CLI Design

- **Named flags**: Use `--flag value` instead of positional arguments
- **Short and long forms**: Provide both (e.g., `-f` and `--file`)
- **Clear help text**: Every command and flag should have descriptive help
- **Cobra framework**: Use Cobra for all CLI commands

### Package Organization

- **`cmd/`**: CLI command definitions
- **`internal/config/`**: YAML config parsing and validation
- **`internal/runner/`**: Parallel wave execution engine
- **`internal/ui/`**: Output formatters (local and CI modes)
- **`internal/github/`**: GitHub API helpers
- **One purpose per package**: Keep packages focused and cohesive

## CI/CD

GitHub Actions workflows automatically:

- Run tests with race detector
- Check code formatting
- Run all linters
- Build binaries
- Verify pre-commit hooks pass

See `.github/workflows/` for workflow definitions.

## Common Tasks

### Add a New Check to .insitu.yml

1. Add an entry to `inventory:` with a unique `id` and `command`
2. Reference the `id` in the appropriate `waves[].checks[]` list
3. Run `insitu plan` to verify the configuration
4. Run `insitu run` to execute

### Add a New CLI Command

1. Create a command file in `cmd/` (e.g., `cmd/newcommand.go`)
2. Create an implementation package if needed (e.g., `internal/newpkg/`)
3. Add tests alongside the implementation
4. Register the command in the file's `init()` function (it auto-registers via `rootCmd.AddCommand`)
5. Update this CONTRIBUTING.md with usage documentation

### Update Dependencies

```bash
# Update go.mod and go.sum
go get -u ./...
go mod tidy

# Verify everything still works
make test
make build
```

## Getting Help

- Check the [RAG files](.github/copilot-instructions.md) first
- Look at existing code for examples
- Review closed pull requests for similar changes
- Open an issue if you have questions

## License

By contributing, you agree that your contributions will be licensed under the same terms as the project.
