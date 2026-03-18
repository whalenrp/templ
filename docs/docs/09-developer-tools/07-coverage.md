# Template Coverage

Template coverage tracks which parts of your templ templates execute during tests, helping identify untested code paths.

## Quick Start

### 1. Generate coverage-instrumented templates

```bash
templ generate --coverage
```

### 2. Run tests with coverage collection

```bash
TEMPLCOVERDIR=coverage/unit go test ./...
```

### 3. Merge coverage profiles

```bash
templ coverage merge -i=coverage/unit/*.json -o=coverage.json
```

## How It Works

When you generate templates with the `--coverage` flag, templ adds tracking calls before each coverage point (expressions, branches, loops, etc.). During test execution, these calls record which code executed.

Coverage is only active when the `TEMPLCOVERDIR` environment variable is set. This ensures zero overhead in production.

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

# Run tests
TEMPLCOVERDIR=coverage go test ./...

# Merge profiles
templ coverage merge -i=coverage/*.json -o=coverage.json
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
