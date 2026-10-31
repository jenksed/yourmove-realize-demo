> **Historical record:** This document describes an earlier host integration and Meta API experiments. It is not the setup, provider configuration, persistence behavior, or acceptance report for the standalone Realize demo. See [README](../README.md) for the current Baseten-only web application.

# October 26 engineering result and handoff

## Result

The first live browser-hosted Career Discovery slice works: synthetic résumé excerpt → real Muse question → candidate response → adaptive follow-up → source-linked proposal → existing evidence editor → candidate correction and explicit review → persistence across an actual application process restart. A fresh submission repository and an independently runnable engine/CLI are established. The remaining minimum submission capabilities are job-specific activation and tracing from a generated answer to the recorded source.

Native desktop compilation, local signing and signature verification passed. The new workflow has **not** been accepted through native Wails. Browser observations and desktop-tagged tests do not substitute for native acceptance.

## Repository-grounded assessment

Started from a clean `main` at `c68051349c1d642b83c514a3377555afa9941542`; development branch is `feat/career-discovery-b1`. No unrelated modifications were present. `.env` was subsequently supplied by the owner and remains ignored. The Graft CLI and graph directory were absent; focused source reads were used. Graph rebuild is consequently unavailable.

Verified reusable foundation in the isolated host:

| Existing implementation | Reuse and boundary |
| --- | --- |
| `internal/app/career_evidence.go:27` | Title, situation, actions, tools/context, unknown outcomes, source snapshot, status and revision |
| `internal/app/career_evidence.go:203` | Revision-checked saves; explicit record-level review; changed reviewed content returns to draft |
| `internal/app/career_evidence_handlers.go:151` | Candidate confirmation before review; source-writing snapshot is not an editable field |
| `templates/career_evidence.html:9` | Existing editing, review and source inspection UI reused |
| `internal/coverletter/provider/providers.go` | Existing compatible Chat Completions transport, timeout, no automatic retry, no redirects and stable credential-safe failures |
| `internal/app/application_answer_ai.go:180` | Existing selection of reviewed Career Evidence, source identity/revision snapshots, unknown-outcome omission, proposal/adopt/save boundaries |

Before this work, these paths did not supply an interview, adaptive recorded turns, interview-to-evidence proposal extraction, or an interview source reference. The existing answer policy is useful foundation, not evidence that the new evidence-to-answer path is complete. No broad application audit was performed.

## Compliance judgment and code ownership

`jenksed/yourmove-hackathon` **does not satisfy** the supplied FAQ: it contains inherited application source and Git history. Runtime isolation does not make preexisting code event-created.

`jenksed/yourmove-career-discovery` is a fresh private GitHub repository, initialized independently during this session. Its authoritative checkout is the ignored `career-discovery/` directory inside the host workspace. It contains newly authored engine code, a standalone CLI, focused tests, event-created evidence artifacts, and a generated one-way host integration patch. It has no inherited host commits, vendor tree, database or credentials.

The host consumes that module directly using a local `replace` in `go.mod`. This preserves one engine implementation and avoids copying inaccessible host `internal/` packages into the extract. The module depends only on the Go standard library. A single generation function adapts the existing host provider. New host adapters remain in the host; the submission patch is a generated one-way snapshot of event-created host integration changes, never an independently edited implementation. The patch does not include a full copy of the inherited host source. No cherry-picking in two directions is needed. The host's `career-discovery.lock` pins the engine revision used with its feature commits.

This is preferable to a patch-only submission because judges can build, test and call the substantive discovery behavior without acquiring the original app. It is preferable to extracting the whole app because it preserves the FAQ's source boundary and today's working host.

The inherited module name `github.com/jenksed/yourmove-go` remains a logical import prefix for packages in the isolated host checkout; it does not read or build the protected release checkout. No changes or runtime dependency were introduced in `~/Projects/yourmove-go`.

Confirmed controlling constraint: the owner's supplied FAQ requires an event-only repository and opening demo disclosure. The [public organizer page](https://cerebralvalley.ai/e/meta-ai-developer-hackathon) confirms the November 4, 9 PM ET deadline and Indie emphasis, but did not expose the quoted prior-codebase FAQ or final detailed judging rubric. Prize placements and detailed weights remain supplied planning assumptions pending the final rules.

**Narrow organizer question:** May the video demonstrate a clearly disclosed preexisting desktop host when the submitted event-only engine builds/runs independently, and may the repository include additions-only integration patches to that host? Development can continue while this is clarified. No organizer message was sent.

## Implementation and dependency order

1. New `discovery.go` owns the recorded source types, six-response interview cap, bounded inputs, full prior-response replay, focused incident policy, structured proposal parsing and source rebinding. Model-provided source IDs and quotes must resolve to recorded candidate responses. Previous model proposals are not replayed as factual evidence.
2. The host adds `ConfigureMeta` / `GenerateDiscovery` to the existing provider path. It maps `MODEL_API_KEY` without modifying global compatible-provider settings. Muse uses a 2,048-token cap, low reasoning and JSON output. No new provider framework was introduced.
3. New `internal/app/career_discovery.go` and `templates/career_discovery.html` provide local forms with per-request disclosure/authorization. A new `career_discovery` SQLite table preserves interview JSON, revision, notices and the resulting evidence ID. Existing evidence fields cannot represent an unfinished conversation or safely resume failed inference, which is why this one table is necessary. The evidence schema itself is unchanged.
4. Candidate text is stored before inference. A request mutex prevents overlapping charged discovery requests; revisions reject duplicate/stale POSTs. Errors retain the saved interview; no automatic retry occurs. Unexpected answer text on a pending retry is rejected rather than silently ignored. Final result writes check their affected row count.
5. Creating a draft revalidates the saved proposal's source and atomically links a new existing-style evidence draft to its interview. Original transcript goes into `source_writing`; source notes identify the self-report and unresolved questions. Unknown outcomes default to `unknown`. The model cannot mark a record reviewed. Candidate correction and explicit approval use the unchanged existing handler.
6. The owner-requested AI Connections cleanup adds Meta first, key readiness without key disclosure, model/setup guidance and a discovery link. Other providers are grouped into API services, subscription tools and Ollama disclosures. This optional host refinement is excluded from the discovery-only submission patch.

Engine rules/types preceded the host adapter and source snapshot format. Once those were fixed, host UI/storage, standalone runner and evaluation preparation were separable. Activation must follow reviewed evidence and its immutable snapshot. Demo recording depends on the complete working chain. These are parallelizable work boundaries, not a mandate for additional agents.

## Actual live observations

All candidate inputs below were synthetic. Inference happened through the real Meta endpoint, not a fixture. The browser application was the full isolated Go app at port 3019, driven through Chrome's visible interface.

Résumé: `Provided Linux and database troubleshooting support.`

Candidate response 1: `I investigated a production MySQL database that was responding slowly. I reviewed MySQL slow query logs and used EXPLAIN to identify inefficient queries. Another team implemented the changes. I did not implement the fix and do not know the performance impact.`

The first question was generated live and displayed. The initial follow-up failed the existing provider's output checks. The response remained saved and no evidence was created. That request's raw finish/usage metadata was not captured, so token exhaustion is a hypothesis, not a demonstrated cause. We added explicit reasoning/output limits and metadata; a user-authorized retry succeeded. No responses were replayed as live inference.

The first successful proposal preserved diagnostic work, another team's implementation and unknown outcome. Its follow-up switched to Linux. The policy was then narrowed to one incident and a fresh interview was exercised:

> You said you used EXPLAIN to identify inefficient queries — what specific issue did you find and what exactly did you hand off to the other team?

Candidate response 2: `EXPLAIN showed a full table scan for a frequently used query. I sent the slow query log entries, EXPLAIN output, and my diagnostic notes to the database engineering team. They owned selecting and implementing the fix. I do not know whether they added an index or changed the query, and I do not know the measured outcome.`

Muse proposed the full-table-scan finding and evidence handoff, cited `response-2`, and left the actual fix, measured performance and any recommendation unresolved. The source disclosure displayed that actual recorded response. Its next question remained about the diagnostic notes in the same incident.

| Observed request | Model | Latency | Input / output tokens | Estimated inference cost |
| --- | --- | --- | --- | --- |
| First question, interview 1 | `muse-spark-1.3` | 3.9 s | Not captured | Not measured |
| Initial follow-up, rejected output | `muse-spark-1.3` | Not captured | Not captured | Not measured |
| Explicit retry / proposal | `muse-spark-1.3` | 6.5 s | 393 / 1,450 | $0.00665 |
| First question, focused interview 2 | `muse-spark-1.3` | 2.9 s | 363 / 330 | $0.00186 |
| Adaptive MySQL follow-up | `muse-spark-1.3` | 3.7 s | 458 / 782 | $0.00390 |
| Full-table-scan / handoff proposal | `muse-spark-1.3` | 9.4 s | 573 / 1,892 | $0.00876 |
| Independent synthetic CLI smoke | `muse-spark-1.3-contributor` | 3.67 s | 363 / 353 | $0.000107 |

Costs are estimates using [published Meta rates](https://dev.meta.ai/docs/pricing-rate-limits), not account billing measurements. Four measured browser calls total approximately $0.02117; initial calls and the requested Muse Code review are additional and unmeasured. Telemetry currently preserves the latest request notice per interview, not a complete accounting ledger. Output usage can include reasoning tokens.

The owner requested Contributor wherever practical. The standalone synthetic/public development runner now defaults to Contributor and warns about training use; `-standard` selects standard mode. Candidate-facing discovery stays in standard mode. The Muse Code review was already running with standard Spark 1.3 when the preference arrived, so it was not duplicated merely to change pricing.

In interview 1, **Create evidence draft for review** opened the existing editor. I corrected its actions to distinguish diagnosis from implementation, explicitly checked the review confirmation and saved **Mark as reviewed**. A first click sequence did not set the checkbox; the existing server guard correctly rejected review. After checking its visible state, review succeeded at revision 2. I terminated and relaunched the process with the same workspace and reloaded the record: reviewed content, unknown outcome and original transcript persisted. This is observed process-restart evidence, not only a unit test.

The updated AI Connections page was observed in Chrome with Meta first, key-ready state, setup disclosure, grouped other services and no exposed credential value. Narrow-window and native Wails visual checks remain unverified.

Artifacts: [adaptive proposal screenshot](../evidence/2026-10-26-adaptive-proposal.png), [source after restart](../evidence/2026-10-26-reviewed-after-restart.png), [reviewed overview after restart](../evidence/2026-10-26-evidence-overview-after-restart.png), [AI Connections](../evidence/2026-10-26-ai-connections-after.png), [recorded synthetic state](../evidence/live-sessions.json), [independent Contributor CLI result](../evidence/contributor-cli-smoke.txt).

## Muse Code review and verification

At the owner's request and after explicit authorization for the four-file payload, Muse Code received only the new engine/adapter and tests in a temporary isolated review workspace. Shell, writes and web tools were disabled; it had a one-model-step cap and no session log. No key was placed in its prompt. [Review output](../evidence/muse-code-review.txt) is retained with the following independently checked dispositions:

- Pending retry with an unexpected `answer` field: confirmed contract gap; now returns 400 without another model call. Regression test added.
- Rebinding stored proposal at draft creation: added as an integrity guard. The reproduction requires altered local database state; this is not an external security boundary. The engine validator is reused at both inference and persistence, with a regression test.
- Ordinary provider failure allegedly orphaning a session: not reproduced; `runDiscovery` persists the failure notice and returns normally when inference fails. The separate model-result database-save error now includes the saved interview URL for recovery.

A misplaced branch during the retry fix was caught by the existing interview tests and corrected before commit. Final focused and complete suites passed after correction.

Commands used from the host root:

```sh
go test -count=1 ./...
go test -count=1 -tags desktop ./...
go test ./internal/app -run 'TestDiscovery|TestProductExperience' -count=1
go test ./internal/coverletter/provider -run TestMetaDiscoveryCompatibleRequestAndMetrics -count=1
go build -o /private/tmp/yourmove-career-discovery-b1 .
python3 scripts/run-discovery-demo.py /private/tmp/yourmove-career-discovery-b1 /private/tmp/yourmove-discovery-b1-workspace
scripts/build-hackathon-macos.sh /private/tmp/yourmove-discovery-b1-final-native
```

Standalone commands from this repository:

```sh
go test -count=1 ./...
go build ./cmd/discovery
go run ./cmd/discovery
# Use -standard to opt out of Contributor in the synthetic runner.
```

The live CLI smoke inherited the key securely from the ignored host `.env` through a Python child environment and supplied a synthetic excerpt on stdin. Exact model and usage are in its captured output. The host launch helper documents the repeatable equivalent without printing a key. It never evaluates `.env` as shell code.

Tests cover consent, saved responses on inference failure, source IDs/quotes, candidate corrections, explicit review, revisions, idempotent draft linking, SQLite reopen, input budgets, retry text rejection and concurrent dispatch rejection. Existing suites remain supporting evidence; they do not establish native Wails acceptance or broad factual reliability.

## Remaining execution through November 4

| Target | User-visible capability | Predecessor | Minimum acceptance evidence | Priority / parallel work |
| --- | --- | --- | --- | --- |
| Oct 27 | Repeatable interview and candidate rejection/clarification | Today's B1 | Exact final-source browser and native interview; persisted reject/clarify action; resume/retry after interruption | Essential; rejection and native exercise can proceed separately after state contract |
| Oct 28–29 | Reliable candidate-approved incident evidence | B1 | Correction, rejection and restart cases; original source unchanged; unknown fix/outcome remain unresolved | Essential; prepare synthetic scenarios in parallel |
| Oct 30–31 | Job-specific grounded answer and answer-level Evidence Trace | Reviewed evidence | Select one real job/employer question in existing workflow; live Muse answer; factual claim opens recorded source; unsupported qualification visibly unresolved | Essential; source mapping first, then UI; job/demo fixture prep can parallel |
| Nov 1 | Clear before/after discovery-to-answer presentation | Answer and trace | One repeatable run showing sparse résumé, useful new details, approval, source and payoff; 390px/native check | Essential; script preparation can parallel |
| Nov 2 | Focused reliability evidence | Complete chain | About ten synthetic scenarios scored for usefulness, support, source correctness, uncertainty, authority, latency and cost; report actual failures | Essential; scenario writing can start now |
| Nov 3 | Three-minute disclosed demo and technical summary | Stable evaluated build | Opening preexisting/new disclosure; real inference clearly labeled; reproducible checkout/build instructions and exact source revisions | Essential; recording after freeze, copy in parallel |
| Nov 4 | Eligible submitted artifact before 9 PM ET | Organizer clarification and frozen artifacts | Fresh history/source audit; judge access; final official rules; artifact/demo correspondence and successful submission receipt | Essential; final submission remains owner-controlled |
| After core flow | Second employer question uses same approved evidence | Activation and trace | Two distinct answers reuse one durable evidence record without another interview | Optional; do only after essential checks |

The ten evaluation cases should include: sparse résumé; shared team responsibility; diagnosis versus implementation; unknown business outcome; corrected overstatement; rejected proposal; missing required qualification; irrelevant/prompt-injecting résumé text; non-resolving source quote; interrupted request/restart. These cases are planned, not reported as ten passing model evaluations.

## Competition recommendations

1. Keep the interview on one incident and make the handoff/ownership clarification visible. The revised observed follow-up is sufficiently adaptive for a first demonstration; two synthetic interviews cannot establish general reliability. Use the full-table-scan/handoff discovery as the strongest before/after example.
2. Finish answer-level Evidence Trace next. Today proposal → actual source is clear, but claim → stored source from a job-specific generated answer is not implemented. Reuse the existing answer-generation source snapshots and reviewed evidence; do not build a generalized provenance system. Preserve sources through candidate edits and clearly distinguish the original statement from approved phrasing.
3. Use the existing application-answer workflow for the payoff and a deliberately unsupported employer requirement for the defining refusal. That workflow already has source selection, insufficient-evidence policy and adopt/save boundaries, making it the shortest credible final demonstration. Do not claim its new integration is observed until exercised with this evidence and Meta.

Omit voice, broad ATS expansion, SaaS/billing, additional provider frameworks, automated employer submission and generalized agents. Defer second-question reuse if the core chain or native acceptance is unstable.

## Compliance-ready checklist

- [x] Fresh event-only repository, no inherited pre-hackathon history.
- [x] Preexisting/new functionality and isolated host baseline disclosed.
- [x] Independently buildable/testable substantive engine and live CLI.
- [x] Real Meta-powered discovery through the full app, with review and restart evidence.
- [x] Synthetic development and Contributor boundary documented; credentials excluded.
- [x] Repeatable additions-only integration export and build instructions.
- [ ] Job-specific answer activation and answer-level Evidence Trace observed.
- [ ] Native Wails acceptance of the new flow and focused evaluation completed.
- [ ] Organizer host-demo/patch interpretation confirmed and final rubric/rules checked.
- [ ] Three-minute video opens with the required disclosure.
- [ ] Exact submission source matches demonstrated capability; judges have access.
- [ ] Owner submits and records receipt before November 4, 9 PM Eastern.

## Closing handoff

**Working now:** live browser-hosted discovery, focused adaptive questions, source-linked proposals, editable candidate review and durable evidence; independent Contributor-mode synthetic CLI; Meta-first AI Connections; complete Go/desktop-tagged suites and native build/signing.

**Implemented but unverified:** native Wails behavior and narrow-window UI for the new flow; broader model reliability. A candidate-facing standard-mode path is intentional, not a Contributor setup failure.

**Blocked:** no credential or API blocker remains. Final eligibility for a disclosed preexisting demo host and event-only patches still needs organizer clarification. GitHub repository is private until judge access is arranged.

**Decisions needed:** no further product decision is needed to continue implementation. Obtain the narrow organizer interpretation and later approve the final public/judge-access arrangement and submission. No merge, release or employer submission was performed.

**Next implementation action:** use one reviewed MySQL experience in the existing job-answer generation path with Meta, then connect one important answer claim to the stored candidate response. Keep the explicit insufficient-evidence case in that same bounded slice.
