# GitHub MCP Server vs GitHub CLI Capability Comparison

This document identifies capabilities available in the GitHub MCP Server that are currently unavailable in the GitHub CLI codebase.

## Summary

The GitHub MCP Server offers **~90+ tools** across various categories, while the GitHub CLI has **~40 command groups** with their subcommands. Many MCP Server capabilities have CLI equivalents, but several key areas are missing or have limited support in the CLI.

---

## Capabilities MISSING from GitHub CLI

### 🔒 Security & Vulnerability Management (NOT in CLI)

| MCP Server Tool | Description | CLI Status |
|-----------------|-------------|------------|
| `get_code_scanning_alert` | Get code scanning alert details | ❌ Missing |
| `list_code_scanning_alerts` | List code scanning alerts | ❌ Missing |
| `get_secret_scanning_alert` | Get secret scanning alert | ❌ Missing |
| `list_secret_scanning_alerts` | List secret scanning alerts | ❌ Missing |
| `get_dependabot_alert` | Get dependabot alert | ❌ Missing |
| `list_dependabot_alerts` | List dependabot alerts | ❌ Missing |
| `get_global_security_advisory` | Get global security advisory | ❌ Missing |
| `list_global_security_advisories` | List global security advisories | ❌ Missing |
| `list_org_repository_security_advisories` | List org security advisories | ❌ Missing |
| `list_repository_security_advisories` | List repo security advisories | ❌ Missing |

**Impact**: Users cannot manage security alerts via CLI; must use web UI or `gh api`.

---

### 🔔 Notifications (Limited in CLI)

| MCP Server Tool | Description | CLI Status |
|-----------------|-------------|------------|
| `list_notifications` | List notifications | ⚠️ Partial (via `gh status`) |
| `get_notification_details` | Get notification details | ❌ Missing |
| `dismiss_notification` | Mark notification read/done | ❌ Missing |
| `mark_all_notifications_read` | Mark all notifications read | ❌ Missing |
| `manage_notification_subscription` | Manage thread subscription | ❌ Missing |
| `manage_repository_notification_subscription` | Manage repo notification subscription | ❌ Missing |

**Impact**: `gh status` shows notifications but cannot manage them. No dedicated notification commands.

---

### 💬 Discussions (NOT in CLI)

| MCP Server Tool | Description | CLI Status |
|-----------------|-------------|------------|
| `get_discussion` | Get discussion details | ❌ Missing |
| `get_discussion_comments` | Get discussion comments | ❌ Missing |
| `list_discussions` | List discussions | ❌ Missing |
| `list_discussion_categories` | List discussion categories | ❌ Missing |

**Impact**: No CLI support for GitHub Discussions. Users must use web UI or `gh api`.

---

### ⭐ Stargazers (NOT in CLI)

| MCP Server Tool | Description | CLI Status |
|-----------------|-------------|------------|
| `star_repository` | Star a repository | ❌ Missing |
| `unstar_repository` | Unstar a repository | ❌ Missing |
| `list_starred_repositories` | List starred repos | ❌ Missing |

**Impact**: Cannot manage stars via CLI. `gh repo view` shows star count but can't star/unstar.

---

### 📁 Direct File Operations (NOT in CLI)

| MCP Server Tool | Description | CLI Status |
|-----------------|-------------|------------|
| `create_or_update_file` | Create/update file in repo | ❌ Missing |
| `delete_file` | Delete file from repo | ❌ Missing |
| `push_files` | Push multiple files in single commit | ❌ Missing |
| `get_file_contents` | Get file contents | ⚠️ Partial (`gh api` or clone) |
| `get_repository_tree` | Get repository tree | ❌ Missing |

**Impact**: CLI assumes clone/push workflow. No direct file manipulation without local clone.

---

### 👥 Teams & Organization Management (Limited in CLI)

| MCP Server Tool | Description | CLI Status |
|-----------------|-------------|------------|
| `get_team_members` | Get team members | ❌ Missing |
| `get_teams` | Get teams for user | ❌ Missing |
| `search_orgs` | Search organizations | ❌ Missing |

**Current CLI**: Only `gh org list` exists. No team management commands.

---

### 🔄 Pull Request Reviews (Partial in CLI)

| MCP Server Tool | Description | CLI Status |
|-----------------|-------------|------------|
| `create_pending_pull_request_review` | Create pending review | ❌ Missing |
| `add_comment_to_pending_review` | Add comment to pending review | ❌ Missing |
| `submit_pending_pull_request_review` | Submit pending review | ❌ Missing |
| `delete_pending_pull_request_review` | Delete pending review | ❌ Missing |
| `request_copilot_review` | Request Copilot review | ❌ Missing |
| `update_pull_request_branch` | Update PR branch from base | ❌ Missing |

**Current CLI**: `gh pr review` submits reviews immediately. No pending review workflow.

---

### 🤖 Copilot Integration (NOT in CLI)

| MCP Server Tool | Description | CLI Status |
|-----------------|-------------|------------|
| `assign_copilot_to_issue` | Assign Copilot coding agent to issue | ❌ Missing |
| `request_copilot_review` | Request Copilot code review | ❌ Missing |

**Impact**: Copilot coding agent features not accessible via CLI.

---

### 🏷️ Issue Sub-Issues & Types (NOT in CLI)

| MCP Server Tool | Description | CLI Status |
|-----------------|-------------|------------|
| `sub_issue_write` | Add/remove/reprioritize sub-issues | ❌ Missing |
| `list_issue_types` | List available issue types | ❌ Missing |
| Issue types in `issue_write` | Create issues with types | ❌ Missing |

**Impact**: Issue hierarchy and typing features not in CLI.

---

### 📊 Projects v2 (Partial in CLI)

| MCP Server Tool | Description | CLI Status |
|-----------------|-------------|------------|
| `get_project_field` | Get single project field | ⚠️ Via `gh project field-list` |
| `get_project_item` | Get single project item | ⚠️ Via `gh project item-list` |
| `add_project_item` | Add item to project | ✅ `gh project item-add` |
| `delete_project_item` | Delete project item | ✅ `gh project item-delete` |
| `update_project_item` | Update project item field | ✅ `gh project item-edit` |

**CLI Coverage**: Good but individual item/field getters are batch operations.

---

### 🔖 Releases (Partial in CLI)

| MCP Server Tool | Description | CLI Status |
|-----------------|-------------|------------|
| `get_latest_release` | Get latest release | ⚠️ Via `gh release list --limit 1` |
| `get_release_by_tag` | Get release by tag | ✅ `gh release view <tag>` |
| `list_releases` | List releases | ✅ `gh release list` |

---

## Capabilities with CLI Equivalents ✅

| Category | MCP Tools | CLI Commands |
|----------|-----------|--------------|
| **Workflows/Actions** | `list_workflows`, `run_workflow`, `list_workflow_runs`, `get_workflow_run`, `cancel_workflow_run`, `rerun_workflow_run`, `rerun_failed_jobs`, `get_job_logs` | `gh workflow list/run/view`, `gh run list/view/watch/cancel/rerun/download` |
| **Issues** | `list_issues`, `issue_read`, `issue_write`, `add_issue_comment`, `search_issues` | `gh issue list/view/create/edit/close/comment` |
| **Pull Requests** | `list_pull_requests`, `pull_request_read`, `create_pull_request`, `update_pull_request`, `merge_pull_request`, `search_pull_requests` | `gh pr list/view/create/edit/merge/close/review/checkout` |
| **Repositories** | `create_repository`, `fork_repository`, `search_repositories`, `list_branches`, `create_branch`, `list_commits`, `get_commit`, `list_tags`, `get_tag` | `gh repo create/fork/clone/view/list`, `gh search repos` |
| **Gists** | `create_gist`, `get_gist`, `list_gists`, `update_gist` | `gh gist create/view/list/edit/delete` |
| **Labels** | `list_label`, `get_label`, `label_write` | `gh label list/create/edit/delete` |
| **Search** | `search_code`, `search_issues`, `search_repositories`, `search_users` | `gh search code/issues/prs/repos/commits` |
| **User Context** | `get_me` | `gh auth status` |

---

## Recommendations for CLI Parity

### High Priority (Frequently Used)
1. **Security Commands** - Add `gh security` command group for code scanning, secret scanning, dependabot, and advisory management
2. **Notifications** - Add `gh notification list/view/dismiss/mark-read` commands
3. **Discussions** - Add `gh discussion list/view/create/comment` commands
4. **Stars** - Add `gh repo star/unstar` and `gh starred list` commands

### Medium Priority
5. **Direct File Ops** - Add `gh repo file create/update/delete` for simple file changes without clone
6. **Teams** - Add `gh team list/view/members` commands
7. **PR Reviews** - Support pending review workflow in `gh pr review`
8. **Sub-Issues** - Add `gh issue sub-issue add/remove/list` commands

### Lower Priority (Specialized)
9. **Copilot Agent** - Add `gh copilot assign/review` commands
10. **Issue Types** - Support issue types in `gh issue create`

---

## Notes

- Users can access missing features via `gh api` for direct REST/GraphQL calls
- Some gaps may be intentional (CLI favors local git workflow over direct file ops)
- MCP Server designed for AI agents; CLI designed for human developers
- This analysis based on GitHub MCP Server (Jan 2026) and GitHub CLI main branch
