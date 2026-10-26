# YourMove Career Discovery

Event-created Go implementation for the Meta Global AI Developer Hackathon, begun October 26, 2026. This repository has fresh history and contains no inherited YourMove application source. The full YourMove application predates this work.

**Working today:** real Muse interview questions, adaptive follow-ups, structured proposals with exact recorded-source resolution, unknowns, and integration into YourMove's existing editable evidence/review/storage workflow. See [observed results and remaining plan](docs/EXECUTION.md) and [synthetic live-session snapshots](evidence/live-sessions.json).

The package does substantive discovery work: it constructs bounded interviews, preserves candidate statements independently of model output, replays recorded context, excludes previous proposals from factual context, resolves exact source IDs and quotes, and rejects malformed or fabricated references. Schema validity and exact quotation do not establish factual correctness. The candidate must review every proposed claim.

## Standalone execution

Requires Go 1.27.2 or later and a Meta Model API key. No YourMove checkout, database, third-party Go packages, or paid service beyond the authorized Meta credits is needed.

```sh
go test -count=1 ./...
go build ./cmd/discovery
go run ./cmd/discovery
```

Set `MODEL_API_KEY` securely in the process environment first. This command transmits supplied input to Meta. Use synthetic information. Paste a one-line résumé excerpt, then answer Muse's questions. A blank response exits. The CLI displays pending proposals and reported usage; it does not approve evidence or persist to YourMove. The desktop host supplies those capabilities.

Example résumé: `Provided Linux and database troubleshooting support.`

The synthetic development runner defaults to `muse-spark-1.3-contributor`, per the owner's preference. Contributor content may be used for Meta model training: use only synthetic/public development inputs. Pass `-standard` to use `muse-spark-1.3`. The candidate-facing host always uses standard mode.

The runner calls `https://api.meta.ai/v1/chat/completions` with low reasoning, JSON output, a 2,048-token output cap, a 120-second timeout, no redirects and no automatic retries. It is a small standalone demonstration adapter; the full app reuses its preexisting Go provider transport. Both call the same authoritative discovery package.

## One implementation, two repositories

```text
yourmove-hackathon/                 Full app / development host, inherited history
  go.mod                           replace discovery module => ./career-discovery
  internal/app/career_discovery.go  New host storage / HTTP adapter
  templates/career_discovery.html  New candidate interface
  career-discovery/                This independent Git checkout (host ignores it)
    discovery.go                  Authoritative engine
    cmd/discovery/                Standalone live runner
    integration/yourmove.patch    Event-only host additions, exported one way
    evidence/                     Synthetic observations / screenshots
```

Write engine changes here, commit here, and build the full application against this same directory. Do not copy the engine into the host or cherry-pick it into another implementation. The engine imports only the Go standard library. Existing `internal/` packages stay in the host; a single generation function connects the engine to its existing provider. The host pins the tested engine commit in `career-discovery.lock`.

The [integration patch](integration/yourmove.patch) contains new event-authored files and zero-context additions to the isolated host baseline `c68051349c1d642b83c514a3377555afa9941542`. It contains no deleted lines or context copied from preexisting source. It is a generated integration artifact, not a second engine implementation. The inherited runtime-isolation commit is identified but not copied here.

For an optional full-host build, obtain the separately disclosed `jenksed/yourmove-hackathon` baseline, apply the patch with `git apply --unidiff-zero`, and clone this repository as its `career-discovery/` directory. See [integration instructions](integration/README.md). Standalone testing and live CLI execution require no host source.

## Competition provenance and remaining eligibility checks

Preexisting: YourMove desktop shell, résumé/profile systems, SQLite, Career Evidence editing/review, answer generation and job workflows. Event work: this discovery engine, standalone adapter, interview storage/HTTP/UI integration, Meta configuration and metrics, source-linked proposals, tests and evidence artifacts. No work was made in the protected `yourmove-go` release checkout.

The supplied FAQ requires a fresh repository containing event work and a demo opening disclosure. This repository follows that source/history separation. Organizer confirmation is still needed that a disclosed preexisting desktop host may appear in the demo while the submitted engine runs independently; confirm acceptance of event-only integration patches as well. Do not claim blanket submission eligibility before the final official rules check.

Suggested opening: “YourMove was already a working Go desktop application for managing job applications. For Meta's hackathon, I built Career Discovery to uncover experience, preserve its sources, and propose candidate-reviewed evidence.” Expand the disclosure to answer activation once that path is actually verified.
