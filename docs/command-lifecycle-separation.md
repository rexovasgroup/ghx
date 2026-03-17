# Proposal: Command Lifecycle Separation

## Problem

The `gh` CLI currently uses cobra's `PersistentPreRunE` and `PersistentPostRun` hooks for application-level concerns: auth checks, feature flag fetching, telemetry sending, host resolution. This conflates two responsibilities:

1. **Framework concerns**: flag parsing, help text, shell completion
2. **Application concerns**: auth, host resolution, feature flags, validation, telemetry

This causes several problems:

- **Host guessing**: `guessTargetHost` runs before the command executes, so it has to reverse-engineer which host the command will target by sniffing flags, env vars, and git remotes. New commands or new ways to specify a host silently break it.
- **Fragile lifecycle**: Application logic spread across `PersistentPreRunE`, `RunE`, and `PersistentPostRun` makes the execution order hard to reason about.
- **Flag validation in the wrong place**: Business rules like mutual exclusion (`--org` vs `--env`) are enforced in cobra's `RunE`, mixing parsing concerns with application logic.
- **Testing friction**: Commands are tested by constructing cobra commands and executing them, rather than testing business logic directly.

## Proposed Architecture

### Core Idea

Separate cobra (parsing) from command execution (application logic) using a `Runnable` wrapper and command structs that declare their requirements through interfaces.

```
cobra: parse flags/args → populate options
    ↓
RunE: construct concrete command struct → hand to Runnable
    ↓
Runnable.Execute():
    Pre:  resolve host, check auth, fetch feature flags, validate
    Run:  command business logic
    Post: send telemetry, cleanup
```

### The `Command` Interface and `Runnable` Wrapper

```go
// Every command implements this minimal interface.
type Command interface {
    Run() error
}

// Runnable handles the pre/post lifecycle for any Command.
type Runnable struct {
    cmd     Command
    factory *Factory
}

func NewRunnable(f *Factory, cmd Command) *Runnable {
    return &Runnable{cmd: cmd, factory: f}
}

func (r *Runnable) Execute() error {
    // Pre: inspect cmd via interface assertions
    if hr, ok := r.cmd.(HostResolver); ok {
        host := r.resolveHost(hr.HostStrategy())
        hr.SetHost(host)
    }
    if ar, ok := r.cmd.(AuthRequired); ok {
        if err := r.checkAuth(ar); err != nil {
            return err
        }
    }
    if fc, ok := r.cmd.(FeatureFlagConsumer); ok {
        flags := r.fetchFeatureFlags()
        fc.SetFeatureFlags(flags)
    }
    if v, ok := r.cmd.(Validator); ok {
        if err := v.Validate(); err != nil {
            return err
        }
    }

    // Run
    err := r.cmd.Run()

    // Post
    r.sendTelemetry()
    return err
}
```

### Capability Interfaces

Commands opt in to framework capabilities by satisfying interfaces:

```go
// HostResolver — command declares how its target host is determined.
type HostResolver interface {
    HostStrategy() HostStrategy
    SetHost(string)
}

type HostStrategy int
const (
    FromRepoFlag     HostStrategy = iota // -R / GH_REPO / git remotes
    FromHostnameFlag                     // --hostname
    FromPositionalArg                    // gh repo clone HOST/OWNER/REPO
    FromDefaultHost                      // config default
)

// AuthRequired — command declares its auth requirements.
type AuthRequired interface {
    AuthRequirements() AuthRequirements
}

// FeatureFlagConsumer — command needs feature flags during execution.
type FeatureFlagConsumer interface {
    SetFeatureFlags(FeatureFlagState)
}

// Validator — command has business-rule validation beyond flag parsing.
type Validator interface {
    Validate() error
}
```

### Shared Base Structs via Embedding

Commands embed shared structs to get common dependencies:

```go
// BaseCommand — every command gets these.
type BaseCommand struct {
    IO     *iostreams.IOStreams
    Config gh.Config
}

// RepoCommand — commands that target a specific repository.
type RepoCommand struct {
    BaseCommand
    Repo ghrepo.Interface
    Host string
}

func (c *RepoCommand) HostStrategy() HostStrategy { return FromRepoFlag }
func (c *RepoCommand) SetHost(h string)           { c.Host = h }
```

### Concrete Example: `gh variable set`

Today:

```go
func NewCmdSet(f *cmdutil.Factory, runF func(*SetOptions) error) *cobra.Command {
    opts := &SetOptions{
        IO:         f.IOStreams,
        Config:     f.Config,
        HttpClient: f.HttpClient,
        Prompter:   f.Prompter,
    }
    cmd := &cobra.Command{
        Use:  "set <variable-name>",
        RunE: func(cmd *cobra.Command, args []string) error {
            opts.BaseRepo = f.BaseRepo
            if err := cmdutil.MutuallyExclusive("...", opts.OrgName != "", opts.EnvName != ""); err != nil {
                return err
            }
            // ... more validation ...
            if runF != nil {
                return runF(opts)
            }
            return setRun(opts)
        },
    }
    // ... flag definitions ...
    return cmd
}
```

Proposed:

```go
// Command struct — declares requirements through embedding and interfaces.
type VariableSetCommand struct {
    RepoCommand                  // embeds IO, Config, Repo, Host; satisfies HostResolver
    HttpClient *http.Client
    OrgName    string
    EnvName    string
    Body       string
    // ...
}

// Override host strategy based on scope.
func (c *VariableSetCommand) HostStrategy() HostStrategy {
    if c.OrgName != "" {
        return FromDefaultHost
    }
    return FromRepoFlag
}

// Business-rule validation — not in cobra.
func (c *VariableSetCommand) Validate() error {
    return cmdutil.MutuallyExclusive(
        "specify only one of --org or --env",
        c.OrgName != "", c.EnvName != "",
    )
}

func (c *VariableSetCommand) Run() error {
    // Pure business logic. Host, auth, config already resolved.
    // ...
}

// Cobra side — only flag parsing, then hand off.
func NewCmdSet(f *Factory) *cobra.Command {
    opts := &SetOptions{}
    cmd := &cobra.Command{
        Use: "set <variable-name>",
        RunE: func(cmd *cobra.Command, args []string) error {
            c := &VariableSetCommand{
                OrgName: opts.OrgName,
                EnvName: opts.EnvName,
                Body:    opts.Body,
            }
            return NewRunnable(f, c).Execute()
        },
    }
    cmd.Flags().StringVarP(&opts.OrgName, "org", "o", "", "")
    cmd.Flags().StringVarP(&opts.EnvName, "env", "e", "", "")
    cmd.Flags().StringVarP(&opts.Body, "body", "b", "", "")
    return cmd
}
```

### What Changes, What Stays

| Concern | Today | Proposed |
|---------|-------|----------|
| Flag parsing | Cobra (`cmd.Flags()`) | Cobra — unchanged |
| Help text / completion | Cobra | Cobra — unchanged |
| Flag validation (mutual exclusion) | Cobra `RunE` | Command struct `Validate()` |
| Auth checks | `PersistentPreRunE` | `Runnable.Execute()` pre-phase |
| Host resolution | `guessTargetHost` in `PersistentPreRunE` | `Runnable` reads `HostStrategy()` |
| Feature flags | `PersistentPreRunE` | `Runnable.Execute()` pre-phase |
| Telemetry | `PersistentPostRun` | `Runnable.Execute()` post-phase |
| Business logic | `fooRun(opts)` | `Command.Run()` |
| Test injection | `runF func(*FooOptions) error` | Construct command struct directly |

### Testing

Commands become trivially testable — construct the struct, call `Run()`:

```go
func TestVariableSet(t *testing.T) {
    c := &VariableSetCommand{
        RepoCommand: RepoCommand{
            BaseCommand: BaseCommand{IO: ios, Config: cfg},
            Repo:        ghrepo.New("owner", "repo"),
            Host:        "github.com",
        },
        HttpClient: fakeClient,
        Body:       "my-value",
    }
    err := c.Run()
    require.NoError(t, err)
}
```

No cobra command construction, no `ExecuteC()`, no buffer wiring.

### Migration Path

This can be adopted incrementally:

1. **Introduce `Runnable` and base interfaces** — no commands change yet.
2. **Migrate one command** (e.g., `gh variable set`) as a proof of concept.
3. **Remove `PersistentPreRunE`/`PostRun` logic** once all commands use `Runnable`.
4. **Delete `guessTargetHost`** — each command declares its strategy.

Commands can coexist: old-style commands use `RunE` directly, new-style commands go through `Runnable`. No big-bang rewrite needed.

## Open Questions

- **Should `Runnable` be the only way to run commands?** Or should simple commands (e.g., `gh version`) bypass it?
- **How granular should `HostStrategy` be?** The current set covers most cases, but commands like `gh variable set --org` change strategy based on flags. The `HostStrategy()` method handles this since it can inspect its own fields.
- **Where does the `HttpClient` come from?** Today it's a factory function. In the new model, `Runnable` could construct it after host resolution, ensuring the client targets the right host.
- **Should `Validate()` return user-facing errors?** Or should it return typed errors that `Runnable` formats?
