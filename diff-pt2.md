# GitHub API Availability for CLI Gap Recommendations

This document cross-references the recommendations from `diff.md` with the actual availability of GitHub REST and GraphQL APIs to determine which features can realistically be implemented in the GitHub CLI.

## Summary

| Category | API Available? | Implementation Feasibility |
|----------|---------------|---------------------------|
| Security (Code Scanning) | ✅ REST API | **High** - Full API support |
| Security (Secret Scanning) | ✅ REST API | **High** - Full API support |
| Security (Dependabot) | ✅ REST API | **High** - Full API support |
| Security Advisories | ✅ REST API | **High** - Full API support |
| Notifications | ✅ REST API | **High** - Full API support |
| Discussions | ✅ GraphQL API | **High** - Full API support |
| Stars | ✅ REST API | **High** - Full API support |
| Direct File Operations | ✅ REST API | **High** - Full API support |
| Teams | ✅ REST API | **High** - Full API support |
| Sub-Issues | ✅ REST + GraphQL | **High** - Full API support |
| PR Update Branch | ✅ REST API | **High** - Already in CLI! |
| Copilot Assignment | ⚠️ REST/GraphQL | **Medium** - API exists |
| Copilot Code Review | ❌ No direct API | **Low** - UI/Ruleset only |

---

## Detailed API Analysis

### 🔒 Security Commands - **FULLY IMPLEMENTABLE**

All security-related features have comprehensive REST API support.

#### Code Scanning Alerts
| Endpoint | Method | Description |
|----------|--------|-------------|
| `/repos/{owner}/{repo}/code-scanning/alerts` | GET | List alerts |
| `/repos/{owner}/{repo}/code-scanning/alerts/{alert_number}` | GET | Get single alert |
| `/repos/{owner}/{repo}/code-scanning/alerts/{alert_number}` | PATCH | Update/dismiss alert |
| `/orgs/{org}/code-scanning/alerts` | GET | List org alerts |

**Scopes Required**: `security_events` or `repo`

#### Secret Scanning Alerts
| Endpoint | Method | Description |
|----------|--------|-------------|
| `/repos/{owner}/{repo}/secret-scanning/alerts` | GET | List alerts |
| `/repos/{owner}/{repo}/secret-scanning/alerts/{alert_number}` | GET | Get single alert |
| `/repos/{owner}/{repo}/secret-scanning/alerts/{alert_number}` | PATCH | Update/resolve alert |
| `/repos/{owner}/{repo}/secret-scanning/alerts/{alert_number}/locations` | GET | Get alert locations |
| `/orgs/{org}/secret-scanning/alerts` | GET | List org alerts |

**Scopes Required**: `security_events` or `repo`

#### Dependabot Alerts
| Endpoint | Method | Description |
|----------|--------|-------------|
| `/repos/{owner}/{repo}/dependabot/alerts` | GET | List alerts |
| `/repos/{owner}/{repo}/dependabot/alerts/{alert_number}` | GET | Get single alert |
| `/repos/{owner}/{repo}/dependabot/alerts/{alert_number}` | PATCH | Update alert |
| `/orgs/{org}/dependabot/alerts` | GET | List org alerts |
| `/enterprises/{enterprise}/dependabot/alerts` | GET | List enterprise alerts |

**Scopes Required**: `security_events` or `repo`

#### Security Advisories
| Endpoint | Method | Description |
|----------|--------|-------------|
| `/advisories` | GET | List global advisories |
| `/advisories/{ghsa_id}` | GET | Get global advisory |
| `/repos/{owner}/{repo}/security-advisories` | GET | List repo advisories |
| `/orgs/{org}/security-advisories` | GET | List org advisories |

**Scopes Required**: Public (global), `repo` (repository-specific)

**Recommendation**: ✅ Create `gh security` command group with subcommands:
- `gh security code-scanning list/view/dismiss`
- `gh security secret-scanning list/view/resolve`
- `gh security dependabot list/view/update`
- `gh security advisory list/view`

---

### 🔔 Notifications - **FULLY IMPLEMENTABLE**

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/notifications` | GET | List notifications |
| `/notifications` | PUT | Mark all as read |
| `/notifications/threads/{thread_id}` | GET | Get thread details |
| `/notifications/threads/{thread_id}` | PATCH | Mark thread as read |
| `/notifications/threads/{thread_id}/done` | PATCH | Mark thread as done |
| `/notifications/threads/{thread_id}/subscription` | GET/PUT/DELETE | Manage subscription |
| `/repos/{owner}/{repo}/notifications` | GET | List repo notifications |
| `/repos/{owner}/{repo}/notifications` | PUT | Mark repo notifications read |

**Scopes Required**: `notifications`

**Recommendation**: ✅ Create `gh notification` command:
- `gh notification list`
- `gh notification view <id>`
- `gh notification mark-read [--all] [<id>...]`
- `gh notification mark-done <id>`
- `gh notification subscribe/unsubscribe`

---

### 💬 Discussions - **FULLY IMPLEMENTABLE**

GraphQL API provides full support for Discussions.

| Mutation/Query | Description |
|----------------|-------------|
| `repository.discussions` | List discussions |
| `repository.discussion(number)` | Get single discussion |
| `repository.discussionCategories` | List categories |
| `createDiscussion` | Create discussion |
| `addDiscussionComment` | Add comment |
| `updateDiscussion` | Update discussion |
| `deleteDiscussion` | Delete discussion |

**Scopes Required**: `repo` or `public_repo`

**Recommendation**: ✅ Create `gh discussion` command:
- `gh discussion list`
- `gh discussion view <number>`
- `gh discussion create --title --body --category`
- `gh discussion comment <number>`
- `gh discussion close/reopen <number>`

---

### ⭐ Stars - **FULLY IMPLEMENTABLE**

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/user/starred/{owner}/{repo}` | PUT | Star repository |
| `/user/starred/{owner}/{repo}` | DELETE | Unstar repository |
| `/user/starred/{owner}/{repo}` | GET | Check if starred (204/404) |
| `/user/starred` | GET | List starred repos |
| `/users/{username}/starred` | GET | List user's starred repos |
| `/repos/{owner}/{repo}/stargazers` | GET | List stargazers |

**Scopes Required**: None for reading, `public_repo` or `repo` for starring

**Recommendation**: ✅ Add to `gh repo`:
- `gh repo star [<repo>]`
- `gh repo unstar [<repo>]`
- `gh starred list [--user <username>]`

---

### 📁 Direct File Operations - **FULLY IMPLEMENTABLE**

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/repos/{owner}/{repo}/contents/{path}` | GET | Get file contents |
| `/repos/{owner}/{repo}/contents/{path}` | PUT | Create or update file |
| `/repos/{owner}/{repo}/contents/{path}` | DELETE | Delete file |

**Notes**:
- Content must be Base64 encoded
- Updates require current file SHA
- Single file per request (no batch)

**Scopes Required**: `repo` or `public_repo`

**Recommendation**: ✅ Add to `gh repo`:
- `gh repo file view <path>`
- `gh repo file create <path> --content/--file`
- `gh repo file edit <path> --content/--file`
- `gh repo file delete <path>`

---

### 👥 Teams - **FULLY IMPLEMENTABLE**

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/orgs/{org}/teams` | GET | List org teams |
| `/orgs/{org}/teams/{team_slug}` | GET | Get team |
| `/orgs/{org}/teams/{team_slug}/members` | GET | List team members |
| `/user/teams` | GET | List user's teams |

**Scopes Required**: `read:org`

**Recommendation**: ✅ Create `gh team` command:
- `gh team list [--org <org>]`
- `gh team view <team>`
- `gh team members <team>`

---

### 🏷️ Sub-Issues - **FULLY IMPLEMENTABLE**

Both REST and GraphQL APIs support sub-issues.

#### REST API
| Endpoint | Method | Description |
|----------|--------|-------------|
| `/repos/{owner}/{repo}/issues/{issue_number}/sub_issues` | GET | List sub-issues |
| `/repos/{owner}/{repo}/issues/{issue_number}/sub_issues` | POST | Add sub-issue |
| `/repos/{owner}/{repo}/issues/{issue_number}/sub_issues/{sub_issue_id}` | DELETE | Remove sub-issue |
| `/repos/{owner}/{repo}/issues/{issue_number}/sub_issues/priority` | PATCH | Reprioritize |

#### GraphQL API
| Operation | Description |
|-----------|-------------|
| `issue.subIssues` | Query sub-issues |
| `issue.parentIssue` | Get parent issue |
| `addSubIssue` | Add sub-issue |
| `removeSubIssue` | Remove sub-issue |

**Note**: Requires `GraphQL-Features: sub_issues` header (preview)

**Recommendation**: ✅ Add to `gh issue`:
- `gh issue sub-issue list <parent>`
- `gh issue sub-issue add <parent> <child>`
- `gh issue sub-issue remove <parent> <child>`

---

### 🔄 PR Update Branch - **ALREADY EXISTS IN CLI!**

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/repos/{owner}/{repo}/pulls/{pull_number}/update-branch` | PUT | Update PR branch |

**CLI Command**: `gh pr update-branch <number>` ✅

**Note**: This was listed as missing in diff.md but actually exists in the CLI. The MCP tool `update_pull_request_branch` maps to `gh pr update-branch`.

---

### 🔄 PR Pending Reviews - **PARTIALLY IMPLEMENTABLE**

The REST API supports creating reviews with comments:

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/repos/{owner}/{repo}/pulls/{pull_number}/reviews` | POST | Create review |
| `/repos/{owner}/{repo}/pulls/{pull_number}/reviews/{review_id}` | PUT | Update review |
| `/repos/{owner}/{repo}/pulls/{pull_number}/reviews/{review_id}/events` | POST | Submit review |
| `/repos/{owner}/{repo}/pulls/{pull_number}/reviews/{review_id}` | DELETE | Delete pending review |
| `/repos/{owner}/{repo}/pulls/{pull_number}/reviews/{review_id}/comments` | GET | List review comments |

**Current CLI**: `gh pr review` creates and submits in one step.

**Recommendation**: ⚠️ Could enhance `gh pr review` with:
- `gh pr review <number> --pending` (create pending)
- `gh pr review <number> --comment-file <file>` (add comments)
- `gh pr review <number> --submit` (submit pending)
- `gh pr review <number> --delete-pending` (delete pending)

---

### 🤖 Copilot Integration - **PARTIAL API SUPPORT**

#### Assign Copilot to Issue
You can assign Copilot via the standard assignees endpoint:

```
POST /repos/{owner}/{repo}/issues/{issue_number}/assignees
{ "assignees": ["copilot"] }
```

**GraphQL**: Requires `GraphQL-Features: issues_copilot_assignment_api_support` header

**Recommendation**: ⚠️ Could add:
- `gh issue assign-copilot <number>`
- OR enhance `gh issue edit <number> --add-assignee copilot`

#### Request Copilot Code Review
❌ **No direct API endpoint** - Copilot code reviews can only be triggered via:
- GitHub.com UI ("Request Copilot Review" button)
- Repository rulesets (auto-trigger on PR)
- `.github/copilot-instructions.md` configuration

**Recommendation**: ❌ Cannot implement `gh pr request-copilot-review` until GitHub exposes an API

---

## Implementation Priority Matrix

### Tier 1: High Value, Full API Support ✅
| Feature | Effort | User Impact |
|---------|--------|-------------|
| `gh security` command group | Medium | High |
| `gh notification` commands | Medium | High |
| `gh discussion` commands | Medium | Medium |
| `gh repo star/unstar` | Low | Medium |

### Tier 2: Medium Value, Full API Support ✅
| Feature | Effort | User Impact |
|---------|--------|-------------|
| `gh team` commands | Low | Medium |
| `gh repo file` operations | Medium | Medium |
| `gh issue sub-issue` commands | Low | Low |
| Enhanced `gh pr review` (pending) | Medium | Medium |

### Tier 3: Limited API Support ⚠️
| Feature | Effort | Blocker |
|---------|--------|---------|
| Copilot issue assignment | Low | Works but non-standard |
| Copilot PR review request | N/A | No API available |

---

## Corrections to diff.md

1. **PR Update Branch**: Listed as missing but `gh pr update-branch` exists ✅
2. **Copilot Review**: Cannot be implemented - no API available
3. **Issue Types**: API exists via `list_issue_types` in GraphQL for orgs with issue types enabled

---

## Conclusion

**90%+ of the recommendations from diff.md are fully implementable** with existing GitHub REST/GraphQL APIs. The main gaps are:

1. **Copilot Code Review** - No API, requires UI or rulesets
2. **Some Copilot features** - Limited/preview API support

All security, notification, discussion, starring, team, file operation, and sub-issue features have complete API coverage and can be added to the GitHub CLI.
