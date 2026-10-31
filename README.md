# YourMove Realize

A standalone career discovery demo: upload a résumé, choose an investigation lead, talk with Realize, and review and export candidate-controlled Career Evidence. The existing discovery package generates leads and questions and validates source-linked proposals; the web app handles documents, sessions and review. Candidate review is not independent verification.

## Run locally

Requires Go 1.27.2 or later and Poppler's `pdftotext` on PATH (macOS: `brew install poppler`; Debian/Ubuntu: `sudo apt install poppler-utils`). PDF and DOCX are extracted locally; scanned PDFs need OCR and are not supported. Maximum upload: 10 MiB.

Create `.env` in the project root:

```dotenv
BASETEN_API_KEY=your-key-here
BASETEN_MODEL=meta-models/muse-spark-open
BASETEN_BASE_URL=https://inference.baseten.co/v1
```

Existing environment variables take precedence over `.env`. The model and base URL are optional. Keep the key private; `.env` is ignored by Git. The application uses Baseten's chat completions endpoint with Meta Muse Spark Open; it never falls back to another provider or model. A Baseten key must have access to the configured model. Requests are not retried automatically.

```sh
go test -count=1 ./...
go run ./cmd/realize-demo
```

Open http://localhost:8080. Upload one of the fictional Avery Morgan PDF or DOCX fixtures in `realize-demo-pack/fixtures/`, confirm the extracted text, and begin discovery. Relevant résumé text and answers are transmitted to Baseten after the displayed disclosure. The engine considers at most the first 8,000 UTF-8 bytes; the interface discloses when a document exceeds that limit.

The session is held in server memory and survives browser refreshes while the process runs. It is lost on restart; download the reviewed JSON for a durable copy. Uploaded originals are not retained. An answer is recorded before the next inference request, so a failed request can be retried without re-entering it. Proposal citations are checked against the exact recorded answer, but a matching quote cannot prove that the model's interpretation is correct. Review and correct it before keeping it.

The historical CLI and benchmark commands and `docs/EXECUTION.md` describe earlier experiments, not this demo's configured provider or product boundary. They are not required to run the web application.
