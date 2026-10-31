# One-shot build brief — YourMove Realize

**Purpose:** A focused, independently runnable hackathon browser demo. The emotional payoff is rediscovering substantive abilities obscured by terse résumé bullets, without inventing accomplishments.

## Hard requirement: real résumé file input

The main entry point accepts a **master résumé or any ordinary résumé in `.pdf` or `.docx`**. Do not make users paste a résumé excerpt. There must be a drag-and-drop area and an accessible file-picker alternative.

- After upload: extract human-readable text server-side, show a minimal preview/confirmation, then let the candidate start discovery. Do not require elaborate profile setup or a résumé editor.
- Limit uploads to **10 MiB** for demo scope. Check size and actual format, not filename alone. Handle corrupt, empty, password-protected or scanned/image-only PDFs with a direct explanation. **No OCR in this slice.**
- DOCX is an OOXML ZIP document; extract visible document paragraphs in sensible reading order. Do not accept `.doc` or `.docm` as if they were `.docx`.
- PDF extraction must actually recover text from the supplied test PDF. A PDF is not plaintext. A single modest extraction dependency is acceptable if it is the smallest reliable path. Document the dependency and make running it reproducible for the demo machine.
- Preserve line and section structure where practical, strip obvious extraction noise, avoid making unsupported assumptions about experience. The current `discovery.New` accepts **at most 8,000 UTF-8 bytes**. Many master résumés are longer: extract the document completely, then use an intentionally bounded portion (e.g. first 8,000 valid UTF-8 bytes, preserving a character boundary) and show an **honest notice** about what will be considered. Do not describe a partial analysis as whole-document understanding. Do not build a second summarization pipeline just for this demo. Never silently pass a huge document to `discovery.New`.
- Process files locally for text extraction; do not write uploaded originals to permanent storage. Show clear notice before sending extracted text to a model service. Never print résumé content or API keys in server logs.
- Use **standard/non-Contributor** model mode for real candidate data. Synthetic developer fixtures may use Contributor only with explicit awareness that inputs may be used for training.

## Reuse the repository, do not recreate it

The existing Go module already owns lead generation (`FindLeads`), focused lead choice (`SelectLead`), interview state (`New`, `Next`, `RecordAnswer`), bounded/source-linked proposals (`ValidateProposal`), connection synthesis and role comparison. The standalone web app uses a narrowly scoped Baseten chat-completions transport rather than a generalized provider layer.

Add one minimal `net/http` server entry point (e.g. `cmd/realize-demo/`), small handlers and templates/static files. Server-rendered HTML, plain CSS and minimal vanilla JS are the default. No React bundler, database, user accounts or API framework.

For local demo state, an in-process per-session state store is sufficient. It should preserve recorded answers across model failures during the same server process, prevent overlapping requests for the same session and avoid exposing another session's information. Do not add a DB just to improve hypothetical persistence.

## Product experience

1. Welcome and upload (one cohesive entry screen, not a marketing site).
2. Upload success: concise extracted résumé preview and explicit **Begin discovery** action.
3. Model-generated investigation leads; user chooses one.
4. Adaptive interview using the existing recorded state. Show one question at a time, human-paced transitions, a clear waiting state and recoverable inference errors.
5. A substantial **Realization** view: proposal/transferable strength, plain-language interpretation, exact source quote and named unknowns. Nothing is automatically approved. Candidate can review/challenge the interpretation. A source panel reveals the original recorded answer.
6. Optional: if already supported and easy to wire, show one cross-experience connection. Never manufacture a connection just to populate the result.

No job search, application automation, ATS integration, résumé rewriting service, cover letter writer, auth, multi-provider selector or unrelated YourMove UI.

## Truthfulness and demo recording

- **Live mode:** real Muse requests through the existing Go engine, truthful waiting/errors, evidence tied to actual responses.
- A **sample walkthrough** is optional and must be clearly labeled as pre-recorded or synthetic. It must not imply that a prewritten result was generated live. The provided documents are source fixtures, **not** model output.
- Never reveal a deterministic result before the candidate has supplied the supporting response. Do not invent numerical scores, badges, percentages, verified skill certifications or unsupported qualifications.
- Don't display 'powered by Meta' or co-brand the UI. Product name: **YourMove Realize**. The README/submission can accurately disclose the Meta Model API and Muse model.
- Keep the original engine repo untouched and do not add this pack to that repo. Any demo-only implementation belongs in the new standalone repository.

## Minimal complete vertical slice

The win condition is **a real DOCX upload and a real PDF upload, each feeding real model questions and a reviewable source-backed realization**. Polish the transitions and result, but do not broaden scope to make the demo feel more complete.

Record from a stable, rehearsed synthetic scenario. The UI should work at 1440px and approximately 390px widths, with keyboard focus and legible contrast in both themes.
