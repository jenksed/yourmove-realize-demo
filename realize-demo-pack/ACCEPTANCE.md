# Minimal acceptance — what must work before recording

Focus on observable behavior through the browser. These are not a request for a new testing framework.

## Functional path

- [ ] From the welcome screen, upload `fixtures/avery-morgan-resume.pdf`, receive a truthful extracted text preview and proceed to a **real** model-generated lead/question.
- [ ] Repeat the same path using `fixtures/avery-morgan-resume.docx`. The relevant experience text survives both parsers; a different layout is acceptable.
- [ ] Choose a lead and complete an adaptive exchange. Candidate answers, not model guesses, supply the source for any proposed career claim.
- [ ] View a meaningful generated insight and open the exact recorded supporting response/quote. Show named unresolved aspects instead of invented outcomes.
- [ ] Candidate corrections/challenges never silently become established facts. If a claim is not supportable, show a graceful absence/uncertainty state rather than a fabricated result.

## Reliability and privacy

- [ ] Corrupt/empty/password-protected PDFs, scanned/image-only PDFs, DOCX extraction failures, and oversized inputs get clear user-facing feedback; no silent fallback to imaginary résumé data.
- [ ] Model unavailable, malformed response, or timeout keeps recorded candidate answers. Show an honest retry path; never say a result completed when it did not.
- [ ] The upload notice is visible before model transmission. API keys never enter client HTML, browser JS or screenshots. Filenames/uploads are not retained as permanent artifacts without a separate reason.
- [ ] Live vs sample/walkthrough status is not ambiguous. If no sample mode is built, that's fine.

## Presentation

- [ ] Light and dark Quiet Ink theme both readable, with working toggle and no flash of unreadable UI.
- [ ] Desktop (~1440px) and narrow (~390px) screen checks: no clipped content or hidden controls.
- [ ] Keyboard navigation and visible focus on upload, lead selection, answer and evidence reveal.
- [ ] No 'powered by Meta', no unsupported endorsement or fictional results shown as observed.
- [ ] One complete, rehearsable ~3-minute video story. Judges should understand the difference between the prior résumé and the insight it supported.

## Explicitly deferred

OCR, bulk upload, accounts, persistence across server restarts, résumé editing, job application workflows, complex auth, deployment orchestration, generic provider configuration, analytics and independent factual verification are **out of scope** unless an observed failure blocks the core demo.
