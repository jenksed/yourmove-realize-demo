# Experience flow, copy, and video beat sheet

This is the proposed **user-facing story**, not an instruction to generate canned answers. The model must produce real leads/questions when in live mode.

## Screen A — Welcome and upload

**Eyebrow:** YOURMOVE / REALIZE

**Headline:** There's more to your story.

**Subhead:** Your résumé tells people where you've worked. Realize helps you see the strengths, decisions, and experiences hiding between the lines.

**Primary CTA:** Choose a résumé

**Dropzone:** Drop a PDF or Word résumé here, or browse files.

**Support line:** Works with a master résumé or any résumé. PDF or DOCX, up to 10 MB.

**Trust copy:** Your experience stays yours. You decide what to keep.

Small print near submission: `Realize extracts text locally. With your approval, relevant text and your answers are sent to the configured AI service to generate questions and suggestions.`

Do **not** require an account, email or job listing before discovery.

## Screen B — Document confirmation

**Headline:** Let's see what's already here.

Present filename and a short readable preview so the candidate can confirm the parser didn't scramble the document. If extraction was truncated to fit the engine, say so. **Begin discovery** sends the relevant text to the model after disclosure/authorization.

## Screen C — Lead choice and dialogue

**Headline:** Start with something worth exploring.

Show 3–5 model-generated investigable leads, not generic skill tags. The candidate chooses an area; the next question is focused on one incident.

**Dialogue intro:** A few questions can reveal things a bullet point leaves out.

**Submit answer:** Continue

**Waiting:** Thinking through what you shared…

Keep the question prominent. Show earlier turns, but don't surround the person with dashboards. Do not advance past rejected/malformed model output; show a retry and keep the candidate's recorded response.

## Screen D — Realization

**Eyebrow:** YOUR EXPERIENCE, IN FOCUS

**Headline:** You were doing more than the résumé said.

Lead with a **source-backed insight** in human language, written from the real generated proposal. The panel should answer three questions:

- **What you did:** a precise, limited statement of the candidate's action.
- **What it suggests:** a transferable capability or interpretation, clearly presented as an inference.
- **What we don't know yet:** ownership, results and gaps that weren't established.

**Source action:** See where this came from

Opening the evidence panel shows exact answer text, selected quote, and unresolved questions. Do not imply independent employment verification. Give the candidate an action to challenge/correct the interpretation; the standalone demo requires explicit candidate review, keep or reject, and JSON export.

**Closing line:** The experience was already yours. Now you can see it more clearly.

## Three-minute video outline (provisional)

| Time | Story beat | What viewers see |
| --- | --- | --- |
| 0:00–0:20 | Problem | An ordinary résumé. Short bullet points. The question: *How much of your story never makes it onto this page?* |
| 0:20–0:45 | Easy start | Upload the synthetic PDF/DOCX, see confirmed extracted text, begin discovery. |
| 0:45–1:40 | Human conversation | Pick an investigation lead; Muse probes actual judgment and handoff rather than just asking a generic trivia question. |
| 1:40–2:30 | The realization | The screen converts candidate statements into a bounded, useful professional insight and clearly shows unknowns. |
| 2:30–2:50 | Trust | Open source quote and show exactly where the claim originated. |
| 2:50–3:00 | Meaning | Close on the product promise and transparent disclosure of what was built during the event. |

This is a **video plan**, not proof of model consistency. Rehearse a real session and only show actual generated text as live output. If a recording includes edited/cut waiting time, avoid presenting it as an uncut real-time interaction.

## Narrative scenario

Use `fixtures/avery-morgan-resume.pdf` or `.docx`. The résumé deliberately under-describes an incident. The candidate's interview statements in `fixtures/demo-interview-answers.md` may illuminate technical judgment, cross-team handoff and scope boundaries. The model decides which lead/question to present; the operator responds naturally using accurate synthetic facts, rather than reading scripted responses that don't answer the question.
