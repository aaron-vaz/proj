# AGENTS.md

Guidelines for agentic coding agents working in this repository.

## Project Overview

`skelly` is a Go CLI tool for scaffolding new projects from templates. It uses go-getter to fetch templates and Go's text/template engine for templating.

- **Language**: Go 1.24
- **Binary**: `skelly`
- **Module**: `github.com/aaron-vaz/skelly`

## Build/Lint/Test Commands

### All-in-one
```bash
make all        # Runs lint, test, and build
```

### Building
```bash
make build      # Build binary for current platform -> build/skelly
make release    # Build for all platforms (linux/darwin, amd64/arm64)
make clean      # Remove build artifacts
```

### Testing
```bash
make test       # Run all tests with coverage -> build/coverage.out
go test -v -race ./...                    # Run all tests directly
go test -v -race -run TestName ./pkg/path # Run single test
go test -v -race -run TestName ./...      # Run single test (search all packages)
```

### Linting
```bash
make lint       # Run golangci-lint (requires installation)
golangci-lint run                        # Run directly
```

### Dependencies
```bash
make deps       # Install dependencies and golangci-lint
```

## Code Style Guidelines

### Package Structure

```
cmd/skelly/        # Main entry point
internal/          # Private packages (not importable externally)
  cli/             # CLI parsing and command invocation
  commands/        # Command implementations
  download/        # Template download functionality
  templates/       # Template processing and rendering
  view/            # User interface (I/O)
```

### Imports

Group imports with blank lines between:
1. Standard library
2. External/third-party packages
3. Local packages (github.com/aaron-vaz/skelly/...)

```go
import (
    "context"
    "errors"
    "fmt"

    "gopkg.in/yaml.v2"

    "github.com/aaron-vaz/skelly/internal/templates"
)
```

### Naming Conventions

- **Interfaces**: Single-word nouns (`Downloader`, `UI`, `Renderer`, `Command`)
- **Structs**: PascalCase (`InitCommand`, `Processor`, `StdUI`)
- **Methods**: PascalCase (exported), camelCase (private)
- **Private struct fields**: camelCase (`options`, `renderer`)
- **Constants**: PascalCase or UPPER_SNAKE_CASE (`TemplateConfigName`)
- **Acronyms**: Keep consistent casing (`StdUI`, not `StdUi`)

### Constructor Functions

Use `NewXxx()` pattern. Return interface types when appropriate:

```go
func NewInitCommand(
    processor *templates.Processor,
    downloader download.Downloader,
    ui view.UI,
) Command[*flag.FlagSet] {
    return &InitCommand{
        processor:  processor,
        downloader: downloader,
        ui:         ui,
        options:    InitOptions{},
    }
}
```

### Error Handling

- Return errors, don't panic (except in main)
- Wrap errors with context using `fmt.Errorf` and `%w`:
  ```go
  return fmt.Errorf("failed to download template: %w", err)
  ```
- Use `errors.Is()` for error comparison:
  ```go
  if errors.Is(err, os.ErrNotExist) {
      // handle not found
  }
  ```
- Main function handles error output to stderr

### Interfaces

- Keep interfaces small and focused
- Define interfaces where they are used
- Document all interface methods:

```go
type Downloader interface {
    // Get downloads a file from source to destination
    Get(ctx context.Context, source string, destination string) error
}
```

### Generics

The codebase uses Go generics. Example:

```go
type Command[T any] interface {
    Name() string
    Init(flags T)
    Run(args []string) error
}
```

### File Organization

Within a file:
1. Package declaration
2. Imports
3. Package-level variables/constants
4. Types (interfaces before structs)
5. Struct types
6. Methods on structs
7. Helper functions
8. Constructor function at the bottom

### Comments

- Add package-level comments for documentation
- Document all exported types, functions, and interface methods
- Use minimal inline comments; prefer self-documenting code
- Comment "why" not "what"

### Testing Conventions

When writing tests:
- Place test files in the same package (`package foo` not `package foo_test`)
- Name test functions `TestFunctionName` or `TestFunctionName_Scenario`
- Use table-driven tests for multiple cases
- Run tests with `-race` flag

### Context Usage

Use context for operations that may timeout or be cancelled:

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
defer cancel()
```

### Code Formatting

- Use `gofmt` or `go fmt` for formatting
- Run `golangci-lint` before committing
- No unused imports or variables
- Prefer explicit error returns over ignoring them
