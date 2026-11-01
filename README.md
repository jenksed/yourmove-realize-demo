# YourMove Realize

A standalone career discovery demo: upload a résumé or paste career material, investigate experiences, review discoveries, and examine connections across them. The existing discovery package generates leads and questions and validates source-linked proposals; the web app handles documents, sessions and review. Candidate review is not independent verification.

## Run locally

Requires Go 1.27.2 or later and Poppler's `pdftotext` on PATH (macOS: `brew install poppler`; Debian/Ubuntu: `sudo apt install poppler-utils`). PDF and DOCX are extracted locally; scanned PDFs need OCR and are not supported. Maximum upload: 10 MiB.

Create `.env` in the project root:

```dotenv
BASETEN_API_KEY=your-key-here
BASETEN_MODEL=meta-models/muse-spark-open
BASETEN_BASE_URL=https://inference.baseten.co/v1
DEEPGRAM_API_KEY=your-deepgram-key
FISH_AUDIO_API_KEY=your-fish-audio-key
# Optional voice overrides; defaults are Laura and Adrian.
FISH_AUDIO_VOICE_ID=e3cd384158934cc9a01029cd7d278634
FISH_AUDIO_MASCULINE_VOICE_ID=bf322df2096a46f18c579d0baa36f41d
# Optional public-demo speech controls (defaults shown)
REALIZE_SPEECH_ENABLED=true
REALIZE_STT_SESSION_LIMIT=5
REALIZE_TTS_SESSION_LIMIT=12
REALIZE_STT_SERVER_LIMIT=50
REALIZE_TTS_SERVER_LIMIT=100
```

Existing environment variables take precedence over `.env`. Choose the interviewer voice before starting: Laura is the default, and Adrian is also available. The choice stays with the session. To override the reference IDs, set `FISH_AUDIO_VOICE_ID` for Laura and `FISH_AUDIO_MASCULINE_VOICE_ID` for Adrian and restart the app. The browser’s default voice is used as a fallback if Fish Audio fails. `REALIZE_SPEECH_ENABLED=false` disables both Deepgram and Fish Audio requests while leaving the text interview available. The default allowances are 5 transcriptions and 12 question-speech requests per session, plus 50 transcription and 100 question-speech requests for the whole server process; set any allowance to `0` to disable that request type. These counters are process-local, reset on restart, and are not a durable spending limit. They are intended to bound casual demo use, not guarantee a maximum bill. Deepgram provides project usage and balance information; check these in its [usage documentation](https://developers.deepgram.com/docs/using-logs-usage) and [project balance API](https://developers.deepgram.com/reference/manage/billing/list). Fish Audio currently lists `s2.1-pro-free` at $0 per million UTF-8 input bytes in its [pricing and rate limits](https://docs.fish.audio/developer-guide/models-pricing/pricing-and-rate-limits); monitor remaining API credits and configure any account-level cap or alerts exposed for the account. No hard provider-side spending cap was confirmed for either configured account, so retain the app allowances and the kill switch. Keep keys private; `.env` is ignored by Git. The application uses Baseten's chat completions endpoint with Meta Muse Spark Open; it never falls back to another provider or model. A Baseten key must have access to the configured model. Requests are not retried automatically.

The hackathon Baseten account used by the companion project has access to the restricted model `meta-models/muse-spark-open-1026`. If the default model returns HTTP 403 with that account, explicitly set `BASETEN_MODEL=meta-models/muse-spark-open-1026` in `.env` and restart. The application will use exactly the configured model; it does not switch models automatically. The discovery request uses JSON output, low reasoning effort and a 3,072-token cap, matching the companion project's working Baseten profile.

```sh
go test -count=1 ./...
go run ./cmd/realize-demo
```

Open http://localhost:8080. Upload one of the fictional Avery Morgan PDF or DOCX fixtures in `realize-demo-pack/fixtures/`, or paste career text. Confirm the material and begin discovery. For pasted material, choose a focused conversation or model-generated starting points. Relevant résumé text and answers are transmitted to Baseten after the displayed disclosure. The engine considers at most the first 8,000 UTF-8 bytes; the interface discloses when a document exceeds that limit.

The session is held in server memory and survives browser refreshes while the process runs. It is lost on restart; use the printable takeaway or download the reviewed JSON for a durable copy. Up to four discoveries can be kept in a session. Two or more can be sent to Baseten for connection synthesis; editing a discovery invalidates the prior synthesis. Uploaded originals are not retained. An answer is recorded before the next inference request, so a failed request can be retried without re-entering it. Proposal citations are checked against the exact recorded answer, but a matching quote cannot prove that the model's interpretation is correct. Review and correct it before keeping it.

The historical CLI and benchmark commands and `docs/EXECUTION.md` describe earlier experiments, not this demo's configured provider or product boundary. They are not required to run the web application.
