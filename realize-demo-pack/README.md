# YourMove Realize — standalone demo handoff pack

This folder is **planning material and reference assets**, not an implemented application.
Place it at the root of the freshly cloned `yourmove-realize-demo` repository, next to its existing `go.mod`, `discovery.go`, and `cmd/`. Do not replace or modify those files while installing the pack.

## The intended result

One polished, browser-based Go demo showing **résumé upload (.pdf or .docx) → candidate-led investigation → meaningful, source-backed realization**. Use the existing career-discovery engine. Make the UI exceptional; keep the implementation small.

## Files

- `BUILD_BRIEF.md` — functional scope, reuse strategy, non-goals, minimal acceptance.
- `EXPERIENCE_AND_COPY.md` — screen flow, exact starting copy and demo-video direction.
- `DESIGN_DIRECTION.md` — Quiet Ink light/dark visual system and design constraints.
- `design/quiet-ink-tokens.css` — CSS variables and theme selector for implementation.
- `design/visual-reference.html` — **static style reference**, not the functioning product. Open locally in a browser. Theme toggle is functional; demonstration content is illustrative.
- `fixtures/avery-morgan-resume.docx` and `.pdf` — same intentionally modest, synthetic résumé, used to verify both upload paths.
- `fixtures/demo-interview-answers.md` — example candidate responses, not predetermined or verified model output.
- `ACCEPTANCE.md` — focused checks for the real user path and video readiness.

## Ownership and provenance

- Existing source: `jenksed/yourmove-career-discovery` remains authoritative for the Go discovery engine, and should remain untouched.
- Target: a **new, separate** `jenksed/yourmove-realize-demo` repo that retains the event-created source/history from the clone and adds only demo-specific integration.
- Never reimplement `FindLeads`, `Next`, `RecordAnswer`, `ValidateProposal`, `SynthesizeConnections`, or role logic in frontend JS or a second Go package.
- This pack is **new planning/demo material**; do not present it as a completed app or a recorded model result.

## After placement

1. Read `BUILD_BRIEF.md`, `EXPERIENCE_AND_COPY.md`, and `ACCEPTANCE.md`.
2. Open `design/visual-reference.html` and switch themes to evaluate direction.
3. We will write the coding-agent one-shot prompt **after** this pack is in the project.

There is no install step and no generated code in this pack that mutates the repository.
