# Full YourMove host integration

`yourmove.patch` is a one-way export of event-created host additions against isolated baseline `c68051349c1d642b83c514a3377555afa9941542`. All application framework, review, SQLite, answer-generation, consent, and employer-submission behavior at that baseline is preexisting. It is not included as submission source.

The owner's subsequent AI Connections cleanup is a host-only refinement and is deliberately excluded from this discovery-only patch. It is present on the host feature branch. That screen is not needed to build or exercise the submitted engine.

Optional full-host setup (the standalone package and CLI do not require this):

```sh
git clone https://github.com/jenksed/yourmove-hackathon.git host
cd host
git checkout c68051349c1d642b83c514a3377555afa9941542
git apply --unidiff-zero /absolute/path/to/yourmove-career-discovery/integration/yourmove.patch
git clone https://github.com/jenksed/yourmove-career-discovery.git career-discovery
go test -count=1 ./...
go build -o /private/tmp/yourmove-career-discovery-b1 .
python3 scripts/run-discovery-demo.py /private/tmp/yourmove-career-discovery-b1 /private/tmp/yourmove-discovery-b1-workspace
```

Use `career-discovery.lock` from the host feature branch to check out its tested engine commit. The baseline-plus-patch setup above is for the extraction snapshot; the maintained host feature branch already includes the patch, so do not apply it again there. GitHub access is currently private; ensure judges get permitted access before submission.

The launch helper reads only `MODEL_API_KEY` from the host's ignored `.env`, without evaluating shell code. An existing environment key takes precedence. It sets an explicit isolated workspace, defaults to port 3019, and maps the same key to the existing compatible-provider settings for later application-answer work. It does not modify YourMove release files or configuration. It does not launch the native bundle automatically.

Visit `http://127.0.0.1:3019/career-discovery`, paste synthetic résumé text, authorize the external request, answer, and create a draft when Muse has enough information. Correct it in the existing editor, check the review declaration, and choose **Mark as reviewed**. Keep outcome unknown unless the candidate actually establishes it. Source inspection retains the original transcript independently of editable notes.

One new SQLite table stores serialized interview state, revision, notices and a draft link. The existing `career_evidence` schema is unchanged. It stores the resulting draft, approval status and immutable transcript snapshot. No second evidence database exists. Review is record-level. Explicit persisted rejection and claim-level answer tracing remain scheduled work.

To refresh the patch after host integration edits, first stage only the intended host code and run this explicit allowlist from the host root:

```sh
git diff --cached --unified=0 c68051349c1d642b83c514a3377555afa9941542 -- \
  go.mod internal/app/app.go internal/app/app_http.go \
  internal/app/career_discovery.go internal/app/career_discovery_test.go \
  internal/coverletter/provider/providers.go internal/coverletter/provider/meta_discovery_test.go \
  templates/career_evidence.html templates/career_discovery.html \
  scripts/run-discovery-demo.py > career-discovery/integration/yourmove.patch
```

Check for deleted/context lines before publishing. Do not export the original app, its Git history, `.env`, local databases, unrelated backlog, or old source snapshots. Commit the generated patch in the engine repository; never edit it as a second implementation. Host application adapters remain authoritative in the host. The package remains authoritative in this repository.
