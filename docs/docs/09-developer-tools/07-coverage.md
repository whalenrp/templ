# Template Coverage

Template coverage tracks which parts of your templ templates execute during tests, helping identify untested code paths.

## Quick Start

### 1. Generate coverage-instrumented templates

```bash
templ generate --coverage
```

### 2. Add TestMain to test packages

Add this to any `_test.go` file in packages that render templ templates:

```go
import (
    "os"
    "testing"

    templruntime "github.com/a-h/templ/runtime"
)

func TestMain(m *testing.M) {
    os.Exit(templruntime.RunWithCoverage(m))
}
```

`RunWithCoverage` is a no-op when `TEMPLCOVERDIR` is not set, so it's safe to leave in permanently. Without this, coverage data will be silently lost when tests exit.

### 3. Run tests with coverage collection

```bash
TEMPLCOVERDIR=coverage/unit go test ./...
```

### 4. Merge coverage profiles

```bash
templ coverage merge -i=coverage/unit/*.json -o=coverage.json
```

## How It Works

When you generate templates with the `--coverage` flag, templ adds tracking calls before each coverage point (expressions, branches, loops, etc.). During test execution, these calls record which code executed.

Coverage is only active when the `TEMPLCOVERDIR` environment variable is set. This ensures zero overhead in production.

Test packages must also use `RunWithCoverage` in a `TestMain` function to ensure coverage data is flushed when tests complete. Setting `TEMPLCOVERDIR` alone is not sufficient — without `TestMain`, the coverage data accumulated during tests will be lost when the test binary exits.

## Coverage Points

Templ tracks execution at the expression and element level:

- String expressions: `{ name }`
- If/else branches: condition, then-branch, else-branch
- Switch cases: expression, each case, default
- For loops: statement, body entry
- Template calls: `@component()`
- Static HTML elements: opening and closing tags
- Text literals
- Children expressions: `{ children... }`

## Workflow

### Collect Coverage

```bash
# Generate instrumented templates
templ generate --coverage

# Ensure test packages have TestMain (one-time setup)
# In each _test.go that renders templates:
#   func TestMain(m *testing.M) {
#       os.Exit(templruntime.RunWithCoverage(m))
#   }

# Run unit tests
TEMPLCOVERDIR=coverage/unit go test ./...

# Run integration tests
TEMPLCOVERDIR=coverage/integration go test ./integration/...
```

### Merge Profiles

Combine coverage from multiple test runs:

```bash
templ coverage merge -i=coverage/unit,coverage/integration -o=coverage.json
```

Supports glob patterns:

```bash
templ coverage merge -i=coverage/**/*.json -o=coverage.json
```

### Generate Reports

Generate a coverage manifest alongside instrumented templates:

```bash
templ generate --coverage --coverage-manifest=coverage-manifest.json
```

View coverage in the terminal:

```bash
templ coverage report -i=coverage/*.json -m=coverage-manifest.json
```

Generate an HTML report with annotated source:

```bash
templ coverage report -i=coverage/*.json -m=coverage-manifest.json -html -o=coverage.html
```

Generate a JSON report for CI integration:

```bash
templ coverage report -i=coverage/*.json -m=coverage-manifest.json -json
```

## Profile Format

Coverage profiles are JSON files:

```json
{
  "version": "1",
  "mode": "count",
  "files": {
    "templates/user/profile.templ": [
      {
        "line": 5,
        "col": 3,
        "hits": 12,
        "type": "expression"
      }
    ]
  }
}
```

- `hits`: Number of times the coverage point executed
- `type`: Kind of coverage point (expression, element, branch, etc.)

## CI/CD Integration

Add to your CI workflow:

```bash
# Generate with coverage
templ generate --coverage

# Run tests (assumes TestMain with RunWithCoverage is already set up)
TEMPLCOVERDIR=coverage go test ./...

# Merge profiles
templ coverage merge -i=coverage/*.json -o=coverage.json
```

## Server Coverage

For non-test binaries (e.g., development servers), use `EnableCoverage()` and `FlushCoverage()` directly:

```go
import templruntime "github.com/a-h/templ/runtime"

func main() {
    templruntime.EnableCoverage()
    defer templruntime.FlushCoverage()

    // start server...
}
```

## Tips

- Use separate coverage directories for different test types (unit, integration)
- Coverage profiles are unique per process (PID + timestamp) to avoid conflicts
- Coverage instrumentation only runs when `TEMPLCOVERDIR` is set
- Regenerate templates without `--coverage` for production builds

## Limitations

- CSS and script templates not yet supported (future enhancement)
- Requires regenerating templates with `--coverage` flag
- Profile format is v1 (may evolve in future releases)
