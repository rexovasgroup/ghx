# Usage Data in GitHub CLI

GitHub CLI sends usage data to help us understand how the tool is being used and to prioritize improvements. We want to be transparent about what we collect and why. This document serves as a point in time reference with the latest code on `trunk`, not a commitment for the future. However, since the first release of `gh`, we have treated the privacy of our users with respect, and we intend to treat usage data in the same manner.

## Why do we collect usage data?

Our team uses usage data to prioritize our work and evaluate whether we are successfully solving real users' problems. For example, if we release an improvement to `gh pr create`, we want to understand whether the change is actually helping people create pull requests more easily — or whether we need to try a different approach.

The more we understand about which commands are being used and how often, the better we can focus on the areas that matter most to our users.

## What is collected

Each time certain commands complete successfully, a single event is sent containing the following information and **nothing else**:

```json
{
  "eventType": "usage",
  "dimensions": {
    "command": "<command (no arguments), e.g. gh pr create>",
    "device_id": "<random UUID, generated once per user/device combination>",
    "flags": "<comma-separated flag names that were explicitly provided, e.g. draft,title>",
    "os": "<operating system>",
    "architecture": "<CPU architecture>",
    "version": "<gh version string>"
  }
}
```

| Field | Description |
|-------|-------------|
| `command` | The command that was invoked (e.g. `gh pr create`, `gh issue list`). Does **not** include flags, arguments, or any user-provided input. |
| `device_id` | A random identifier generated once and stored locally per user/device combination. This is **not** a machine ID — different users on the same machine get different identifiers. See [Deleting your device ID](#deleting-your-device-id) below. |
| `flags` | A comma-separated sorted list of flag names that were explicitly provided (e.g. `draft,limit,state`). Only flag **names** are recorded, never flag **values**. If no flags were provided, this is an empty string. |
| `os` | The operating system (`darwin`, `linux`, `windows`). |
| `architecture` | The CPU architecture (`amd64`, `arm64`, etc.). |
| `version` | The `gh` version string. |

### What is NOT collected

- **No flag values** — we record that `--title` was used, not what the title was
- **No arguments** — we record `gh pr create`, not `gh pr create my-branch`
- **No repository names or URLs**
- **No usernames or authentication tokens**
- **No file contents or paths**
- **No environment variables**
- **No IP-based geolocation**

The telemetry request is **unauthenticated** — no GitHub tokens or credentials are included in the request.

## How it works

After a command completes successfully, a background process sends the usage event independently so it doesn't slow down the CLI. If the command fails, is interrupted (e.g. Ctrl+C), or the device ID cannot be created, no data is sent.

During the initial rollout, only a subset of commands send usage data. Over time, we plan to expand coverage.

The full implementation can be found in [`internal/telemetry/`](https://github.com/cli/cli/tree/trunk/internal/telemetry/) and [`pkg/cmd/send-telemetry/`](https://github.com/cli/cli/tree/trunk/pkg/cmd/send-telemetry/).

## Opting out

You can opt out of telemetry in two ways:

- **Environment variable**: Set `GH_NO_TELEMETRY` to any truthy value (e.g. `true`, `1`, `yes`) to disable telemetry for that invocation. This is useful for scripting or CI environments.

  ```sh
  GH_NO_TELEMETRY=true gh pr list
  ```

- **Configuration**: Set `no_telemetry` to a truthy value in your gh configuration file to permanently disable telemetry.

  ```sh
  gh config set no_telemetry true
  ```

The environment variable takes precedence over the configuration setting.

## Privacy

GitHub CLI's telemetry practices are governed by [GitHub's General Privacy Statement](https://docs.github.com/en/site-policy/privacy-policies/github-general-privacy-statement).

## Deleting your device ID

Your device ID is stored at `<state-dir>/device-id` where `<state-dir>` is typically:

- **macOS/Linux**: `~/.local/state/gh/`
- **Windows**: `%LocalAppData%\gh\`

You can delete this file at any time. A new identifier will be generated on the next command invocation.
