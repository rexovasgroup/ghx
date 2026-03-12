# AGENTS.md

This is the GitHub CLI (`gh`), a command-line tool for interacting with GitHub. The module path is `github.com/cli/cli/v2`.

## Build, Test, and Lint

```bash
# Build
make              # Unix — outputs bin/gh
go run script/build.go  # Windows — outputs bin\gh

# Test
go test ./...                              # All unit tests
go test ./pkg/cmd/issue/list/... -run TestIssueList_nontty  # Single test
go test -tags acceptance ./acceptance      # Acceptance tests (see acceptance/README.md)

# Lint
make lint         # Runs golangci-lint (same config as CI)
```

## Architecture

Entry point is `cmd/gh/main.go` → `internal/ghcmd.Main()` → `pkg/cmd/root.NewCmdRoot()`.

Top-level packages:
- `pkg/cmd/<command>/<subcommand>/` — All CLI command implementations
- `pkg/cmdutil/` — Factory (dependency injection), error types, flag helpers
- `pkg/iostreams/` — I/O abstraction with TTY detection, color, pager support
- `pkg/httpmock/` — HTTP mocking for tests
- `api/` — GitHub API client (GraphQL + REST) and pre-built query functions
- `internal/featuredetection/` — Runtime feature detection for GitHub.com vs GHES compatibility
- `internal/prompter/` — User prompting abstraction (survey/huh-based)
- `internal/config/` — Configuration management
- `internal/ghrepo/` — Repository name/owner representation
- `git/` — Local git operations
- `context/` — Git remote resolution (legacy; only for referencing remotes)

## Command Structure

Every command follows the same pattern. A command `gh foo bar` lives in `pkg/cmd/foo/bar/` with these files:

- `bar.go` — Command implementation
- `bar_test.go` — Tests
- `http.go` / `http_test.go` — API call logic (when non-trivial)

### The Options + Factory Pattern

```go
// 1. Options struct holds all command inputs and injected dependencies
type BarOptions struct {
    IO         *iostreams.IOStreams
    HttpClient func() (*http.Client, error)
    Config     func() (gh.Config, error)
    BaseRepo   func() (ghrepo.Interface, error)
    // ... flags and args
}

// 2. NewCmdBar creates the cobra.Command, wiring up the Factory
func NewCmdBar(f *cmdutil.Factory, runF func(*BarOptions) error) *cobra.Command {
    opts := &BarOptions{
        IO:         f.IOStreams,
        HttpClient: f.HttpClient,
    }
    cmd := &cobra.Command{
        Use:   "bar",
        Short: "Do the bar thing",
        Args:  cmdutil.ExactArgs(1, "cannot bar: argument required"),
        RunE: func(cmd *cobra.Command, args []string) error {
            opts.BaseRepo = f.BaseRepo  // lazy-init inside RunE
            // populate opts from args/flags...
            if runF != nil {
                return runF(opts)  // test injection point
            }
            return barRun(opts)
        },
    }
    cmd.Flags().StringVarP(&opts.SomeFlag, "flag", "f", "", "Description")
    return cmd
}

// 3. barRun contains the actual logic
func barRun(opts *BarOptions) error {
    // implementation
}
```

Key details:
- `runF` parameter allows test injection — tests pass a function that calls the real `barRun` after overriding options
- Lazy-init fields like `BaseRepo`, `Remotes`, `Branch` are set inside `RunE`, not in the constructor
- Commands are registered in `pkg/cmd/root/root.go` via `NewCmdRoot()`
- Parent commands (e.g., `pkg/cmd/pr/pr.go`) group subcommands using `cmdutil.AddGroup()`

### Command Examples and Help Text

Use `heredoc.Doc` for examples with `#` comment lines and `$ ` command prefixes:
```go
Example: heredoc.Doc(`
    # Do the thing
    $ gh foo bar --flag value
`),
```

## Testing

### HTTP Mocking

Tests use `pkg/httpmock.Registry` which implements `http.RoundTripper`:

```go
reg := &httpmock.Registry{}
defer reg.Verify(t)  // ensures all stubs were called

// Register stubs
reg.Register(
    httpmock.REST("GET", "repos/OWNER/REPO"),
    httpmock.JSONResponse(someData),
)
reg.Register(
    httpmock.GraphQL(`query PullRequestList\b`),
    httpmock.FileResponse("./fixtures/prList.json"),
)

// Use as HTTP transport
client := &http.Client{Transport: reg}
```

Available matchers: `REST(method, path)`, `GraphQL(queryPattern)`, `QueryMatcher(method, path, query)`, `GraphQLMutationMatcher(name, callback)`.

Available responders: `JSONResponse(body)`, `FileResponse(path)`, `StringResponse(body)`, `StatusStringResponse(status, body)`, `GraphQLQuery(body, callback)`.

### IOStreams in Tests

```go
ios, stdin, stdout, stderr := iostreams.Test()
ios.SetStdoutTTY(true)  // simulate terminal
ios.SetStdinTTY(true)
ios.SetStderrTTY(true)
```

### Assertions

Use `testify` for assertions. The `assert` package is fine for general checks, but always use `require` (not `assert`) for error checks — `require.NoError` and `require.Error` — so that the test halts immediately on failure rather than continuing with a nil/invalid value:

```go
require.NoError(t, err)
require.Error(t, err)
assert.Equal(t, "expected", actual)
```

### Common Test Pattern

```go
func TestBarRun(t *testing.T) {
    tests := []struct {
        name       string
        opts       *BarOptions
        httpStubs  func(*httpmock.Registry)
        wantOut    string
        wantErrOut string
        wantErr    string
    }{
        // table-driven test cases...
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            reg := &httpmock.Registry{}
            defer reg.Verify(t)
            if tt.httpStubs != nil {
                tt.httpStubs(reg)
            }
            // set up ios, factory, run command, assert
        })
    }
}
```

### Generated Mocks

Interfaces use `moq` for mock generation:
```go
//go:generate moq -rm -out prompter_mock.go . Prompter
```

Run `go generate ./...` to regenerate mocks after interface changes.

## Error Handling

Error types in `pkg/cmdutil/errors.go`:
- `FlagErrorf("msg", args...)` — flag/argument validation errors (prints usage)
- `cmdutil.SilentError` — exit code 1 with no message
- `cmdutil.CancelError` — user cancelled (e.g., Ctrl+C or prompt dismissal)
- `cmdutil.PendingError` — nothing failed but outcome is pending
- `cmdutil.NoResultsError` — query returned no results

Use `cmdutil.MutuallyExclusive("message", cond1, cond2)` for mutually exclusive flags.

## Feature Detection

`internal/featuredetection/` detects capabilities of the connected GitHub host (GitHub.com vs GHES) using GraphQL introspection. Commands that use feature detection must include a `// TODO <cleanupIdentifier>` comment directly above the if-statement for linter compliance:

```go
// TODO someFeatureCleanup
if features.SomeCapability {
    // use new API
} else {
    // fallback for older GHES
}
```

## API Patterns

The `api.Client` wraps HTTP for GitHub API calls:
```go
client := api.NewClientFromHTTP(httpClient)
client.GraphQL(hostname, query, variables, &data)  // GraphQL
client.REST(hostname, "GET", "repos/owner/repo", nil, &data)  // REST
client.Mutate(hostname, "MutationName", &mutation, variables)  // GraphQL mutation
```

All REST requests include `X-GitHub-Api-Version: 2022-11-28`.

For host resolution, use `cfg.Authentication().DefaultHost()` — not `ghinstance.Default()` which always returns `github.com`.
