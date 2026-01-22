# Effort Estimation for CLI Gap Implementation

This document estimates the development effort required to implement the recommendations from `diff.md` and `diff-pt2.md`, based on the repository's current velocity and codebase patterns.

---

## Repository Metrics (2024-2026)

| Metric | Value |
|--------|-------|
| PRs merged in 2024 | ~2,739 |
| PRs merged in 2025 (to date) | ~441 |
| PRs merged per month (avg) | ~35-40 |
| Feature PRs per month | ~2-5 |
| Dependency updates | ~50% of PRs |
| Bug fixes | ~15% of PRs |

---

## Code Size Reference (Existing Commands)

| Command Type | Files | Lines of Code | Example |
|--------------|-------|---------------|---------|
| Simple subcommand | 2 | ~150-200 | `issue/close` |
| List/View subcommand | 3-4 | ~400-600 | `issue/list`, `pr/view` |
| New command group (simple) | ~20 | ~2,000-3,000 | `agent-task` |
| New command group (complex) | ~60 | ~8,000-10,000 | `attestation` |

### Typical Command Structure

Each subcommand follows a consistent pattern:
- **Options struct** - Injectable dependencies + flags
- **NewCmd factory** - Creates Cobra command, registers flags
- **RunE handler** - Validates options, calls main function
- **HTTP layer** - GraphQL/REST queries (separate `http.go` for data-heavy commands)
- **Tests** - Colocated `*_test.go` files with fixtures

---

## Effort Estimates by Feature

### Tier 1: High Priority Features

| Feature | Subcommands | Estimated LOC | Dev Weeks | Notes |
|---------|-------------|---------------|-----------|-------|
| `gh security` | 12 (4 domains × 3 each) | 4,000-5,000 | 6-8 weeks | 4 alert types, similar patterns |
| `gh notification` | 5 | 1,500-2,000 | 2-3 weeks | REST API, straightforward |
| `gh discussion` | 5 | 2,000-2,500 | 3-4 weeks | GraphQL, similar to issues |
| `gh repo star/unstar` | 3 | 400-600 | 1 week | Simple REST endpoints |

**Tier 1 Total: 12-16 developer weeks**

### Tier 2: Medium Priority Features

| Feature | Subcommands | Estimated LOC | Dev Weeks | Notes |
|---------|-------------|---------------|-----------|-------|
| `gh team` | 3 | 800-1,200 | 1-2 weeks | Simple list/view pattern |
| `gh repo file` | 4 | 1,200-1,500 | 2-3 weeks | Base64 encoding complexity |
| `gh issue sub-issue` | 3 | 600-800 | 1 week | Extend existing issue cmd |
| Enhanced `gh pr review` | flags | 400-600 | 1 week | Modify existing command |

**Tier 2 Total: 5-7 developer weeks**

---

## Total Effort Summary

| Category | Dev Weeks | Confidence |
|----------|-----------|------------|
| **Tier 1 (High Priority)** | 12-16 weeks | High |
| **Tier 2 (Medium Priority)** | 5-7 weeks | High |
| **Total** | **17-23 weeks** | Medium |

---

## Velocity-Based Timeline

Based on the repository's current velocity:
- **Feature PRs merged**: ~2-5 per month
- **Core team capacity**: Appears focused on maintenance + selective features
- **External contributions**: ~30% of non-dependency PRs

### Realistic Implementation Scenarios

#### Scenario A: Dedicated Sprint (1 developer)
| Scope | Timeline |
|-------|----------|
| All Tier 1 + Tier 2 | **5-6 months** |
| Just `gh security` | **2 months** |
| Just `gh notification` + `gh discussion` | **2 months** |

#### Scenario B: Parallel Development (2-3 developers)
| Scope | Timeline |
|-------|----------|
| All Tier 1 + Tier 2 | **2-3 months** |
| Tier 1 only | **1.5-2 months** |

#### Scenario C: Community Contributions + Core Team
| Scope | Timeline |
|-------|----------|
| Tier 1 features (core team) | **3-4 months** |
| Tier 2 features (community) | **6-12 months** (variable) |

---

## Recommended Implementation Order

Based on the codebase patterns, API complexity, and user value:

### Phase 1: Quick Wins (2-3 weeks)
1. **`gh repo star/unstar`** (1 week)
   - Simple REST API (PUT/DELETE)
   - High visibility, low effort
   - Good warmup for contributors

2. **`gh issue sub-issue`** (1 week)  
   - Extends existing `gh issue` command
   - REST + GraphQL support
   - Growing user demand

### Phase 2: High-Value Additions (5-7 weeks)
3. **`gh notification`** (2-3 weeks)
   - High user demand (complements `gh status`)
   - Simple REST API patterns
   - Good template for other commands
   - Subcommands: `list`, `view`, `mark-read`, `mark-done`, `subscribe`

4. **`gh discussion`** (3-4 weeks)
   - Fills major gap vs MCP Server
   - GraphQL patterns already established in codebase
   - Similar structure to `gh issue`
   - Subcommands: `list`, `view`, `create`, `comment`, `close`

### Phase 3: Enterprise Features (8-10 weeks)
5. **`gh security`** (6-8 weeks)
   - High enterprise value
   - Four domains share similar patterns:
     - `code-scanning` (list/view/dismiss)
     - `secret-scanning` (list/view/resolve)
     - `dependabot` (list/view/update)
     - `advisory` (list/view)
   - Consider shipping incrementally (one domain at a time)

6. **`gh team`** (1-2 weeks)
   - Complements `gh org`
   - Simple list/view/members pattern
   - Enterprise-focused

### Phase 4: Advanced Features (3-4 weeks)
7. **`gh repo file`** (2-3 weeks)
   - Direct file operations without clone
   - Base64 encoding adds complexity
   - Useful for automation/scripting

8. **Enhanced `gh pr review`** (1 week)
   - Add pending review workflow
   - Flags: `--pending`, `--submit`, `--delete-pending`

---

## Risk Factors

| Risk | Impact | Likelihood | Mitigation |
|------|--------|------------|------------|
| API changes during development | Medium | Low | Pin to stable API versions |
| GraphQL schema complexity | Low | Low | Follow existing patterns |
| Enterprise-only features | Medium | Medium | Feature flags, graceful degradation |
| Testing infrastructure | Low | Low | Existing patterns well-established |
| Scope creep | High | Medium | Ship MVP, iterate based on feedback |

---

## Resource Requirements

### Per Feature Group

| Feature | Skills Required | Testing Effort |
|---------|-----------------|----------------|
| `gh security` | REST API, JSON output | High (4 domains) |
| `gh notification` | REST API | Medium |
| `gh discussion` | GraphQL | Medium |
| `gh repo star` | REST API | Low |
| `gh team` | REST API | Low |
| `gh repo file` | REST API, Base64 | Medium |
| `gh issue sub-issue` | REST + GraphQL | Low |
| `gh pr review` enhancements | REST API | Low |

### Infrastructure Needs
- No new dependencies required
- Existing test patterns sufficient
- CI/CD already supports new commands

---

## Bottom Line

### Total Effort
**17-23 developer weeks** for full parity with MCP Server capabilities (excluding Copilot features with no API).

### Timeline Projections

| Approach | Timeline | Confidence |
|----------|----------|------------|
| **Best case** (dedicated effort) | 3-4 months | Medium |
| **Realistic case** (normal prioritization) | 6-9 months | High |
| **Conservative case** (community-driven) | 12+ months | High |

### Recommended Strategy

1. **Start small**: Ship `gh repo star` and `gh issue sub-issue` first (2 weeks)
2. **Build momentum**: Add `gh notification` (high demand, moderate effort)
3. **Major feature**: Tackle `gh security` incrementally (one domain per release)
4. **Fill gaps**: Complete with `gh discussion`, `gh team`, file operations

This approach allows for:
- Early user feedback
- Incremental value delivery
- Parallel community contributions on simpler features
- Core team focus on complex features (`gh security`)
