---
name: telemetry-review
description: Validates that docs/TELEMETRY.md accurately reflects the telemetry implementation. Use after any change touching internal/telemetry/, pkg/cmd/send-telemetry/, pkg/cmd/root/root.go, or docs/TELEMETRY.md.
---

# Telemetry Documentation Review

You are reviewing changes that touch telemetry code. Your job is to verify that `docs/TELEMETRY.md` is accurate, complete, and appropriate for its audience.

## Audience

`docs/TELEMETRY.md` is a **public, user-facing document**. Its audience is end users of the GitHub CLI who want to understand what usage data is collected and why. It should:

- Explain *why* usage data is collected and how it helps improve the product
- Describe *exactly* what data is sent (and what is not)
- Be transparent about how users can control their data (e.g. deleting device ID)
- Link to source code so users can verify claims themselves
- **Not** contain internal infrastructure details (service names, database queries, internal URLs, internal team names, etc.)

## Steps

1. Read `docs/TELEMETRY.md` in full.
2. Read the telemetry implementation:
   - `internal/telemetry/telemetry.go` — event structs, dimensions, device ID, subprocess spawning
   - `pkg/cmd/send-telemetry/send_telemetry.go` — the hidden command that sends usage data
   - `pkg/cmd/root/root.go` — the hook that triggers telemetry
   - Any other file changes in your session that might have modified telemetry
3. Verify the following:

### Payload fields
- Does the example JSON in TELEMETRY.md match the `Event` and `Dimensions` structs exactly (field names, JSON tags, nesting)?
- Are all fields in the struct documented? Are there any documented fields that don't exist in code?

### What is NOT collected
- Verify each "not collected" claim is true by checking the code. For example, confirm that arguments are genuinely not included in the payload.

### How it works
- Does the description of how telemetry works match the actual code flow?
- Are the conditions under which telemetry is not sent accurate?

### Device ID
- Does the description of where the device ID is stored match `getOrCreateDeviceID()`?
- Are the listed default paths correct for each OS?

### No internal details
- Verify the document does **not** contain references to internal infrastructure, services, databases, query languages, internal URLs, or team names. These are implementation details that don't belong in a user-facing document.

### Tone and clarity
- Is the document written for end users, not developers?
- Does it explain *why* data is collected, not just *what*?
- Is it clear how users can delete their device ID?

## Output

If everything is consistent, approve with a short confirmation.

If there are discrepancies, list each one with:
- What the doc says
- What the code actually does (or what the audience concern is)
- A suggested fix for the doc