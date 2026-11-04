package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	discovery "github.com/jenksed/yourmove-career-discovery"
)

func TestFixtures(t *testing.T) {
	for _, ext := range []string{"pdf", "docx"} {
		t.Run(ext, func(t *testing.T) {
			b, e := os.ReadFile("../../realize-demo-pack/fixtures/avery-morgan-resume." + ext)
			if e != nil {
				t.Fatal(e)
			}
			text, e := extract("resume."+ext, b)
			if e != nil {
				t.Fatal(e)
			}
			if !strings.Contains(text, "MySQL") || !strings.Contains(text, "Northline") {
				t.Fatal("missing fixture content")
			}
		})
	}
	if _, e := extract("fake.pdf", []byte("not a PDF")); e == nil {
		t.Fatal("accepted invalid PDF")
	}
}
func TestBoundedUTF8(t *testing.T) {
	s, truncated := bounded(strings.Repeat("a", 7999) + "é" + "rest")
	if !truncated || len(s) != 7999 {
		t.Fatalf("bad boundary %d", len(s))
	}
}
func TestReviewAndFailedInference(t *testing.T) {
	source := "I diagnosed the issue and handed my findings to engineering."
	st, _ := discovery.New("A useful resume excerpt about troubleshooting.")
	st.Question = "What did you do?"
	st, _ = discovery.RecordAnswer(st, source)
	s := &server{sessions: map[string]*session{}, generate: func(context.Context, string, string) (string, error) { return "", errors.New("unavailable") }}
	v := &session{Stage: "conversation", State: st}
	s.sessions["test"] = v
	post := func(path string, values url.Values) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", path, strings.NewReader(values.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.AddCookie(&http.Cookie{Name: "realize_session", Value: "test"})
		w := httptest.NewRecorder()
		s.serve(w, r)
		return w
	}
	post("/retry", url.Values{})
	if len(v.State.Responses) != 1 || v.State.Responses[0].Source.Text != source {
		t.Fatal("answer was lost")
	}
	v.State.Proposal = &discovery.Proposal{Title: "Diagnosis", Experience: "Investigated an issue", Claim: "Diagnostic judgment", SourceID: "response-1", Quote: "diagnosed the issue", Unresolved: []string{"Outcome unknown"}}
	v.State.Question = "What happened next?"
	post("/reveal", url.Values{})
	post("/review", url.Values{})
	post("/keep", url.Values{"title": {"Corrected title"}, "experience": {"Investigated and handed off"}, "claim": {"Diagnostic work"}})
	if v.Saved == nil || v.Saved.Title != "Corrected title" || v.Saved.SourceText != source || v.Saved.ReviewStatus != "candidate_reviewed" {
		t.Fatal("review or source not preserved")
	}
	r := httptest.NewRequest("GET", "/download", nil)
	r.AddCookie(&http.Cookie{Name: "realize_session", Value: "test"})
	w := httptest.NewRecorder()
	s.serve(w, r)
	var got evidence
	if json.Unmarshal(w.Body.Bytes(), &got) != nil || got.Quote != "diagnosed the issue" || got.SourceText != source || got.OriginalProposal == nil || got.OriginalProposal.Title != "Diagnosis" || len(got.ReviewChanges) != 1 || got.ReviewChanges[0].After.Title != "Corrected title" {
		t.Fatal("bad export")
	}
	post("/edit", url.Values{"id": {"1"}})
	post("/keep", url.Values{"title": {"Corrected title"}, "experience": {"Investigated and handed off"}, "claim": {"Diagnostic work refined"}})
	if v.Saved.OriginalProposal == nil || v.Saved.OriginalProposal.Title != "Diagnosis" || len(v.Saved.ReviewChanges) != 2 || v.Saved.ReviewChanges[1].Before.Claim != "Diagnostic work" || v.Saved.ReviewChanges[1].After.Claim != "Diagnostic work refined" || v.Saved.Responses[0].Source.Text != source {
		t.Fatal("re-edit lost provenance or changed source")
	}
}

func TestKeepDiscoveryKeepsUnchangedProposalGroundedAndEditsCandidateAuthored(t *testing.T) {
	tests := []struct {
		name, source, proposal, expected string
	}{
		{"Priya read-aloud purpose", "I posted the wrong room number once because I relied on memory. Since then I read the layout sheet aloud with a second staff member before doors open.", "I read the layout sheet aloud with a second staff member to get event rooms ready.", "I posted the wrong room number once because I relied on memory. Since then I read the layout sheet aloud with a second staff member before doors open."},
		{"Jordan customer callback", "I called a customer back after I found the account mismatch.", "I promised the customer I would call again after correcting the account.", "I called a customer back after I found the account mismatch."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st, _ := discovery.New("Resume excerpt")
			st.Question = "What happened?"
			st, _ = discovery.RecordAnswer(st, tt.source)
			st.Proposal = &discovery.Proposal{Title: "Discovery", Experience: tt.proposal, Claim: "A possible interpretation", SourceID: "response-1", Quote: tt.source}
			v := &session{Stage: "review", State: st}
			s := &server{sessions: map[string]*session{"test": v}}
			r := httptest.NewRequest("POST", "/keep", strings.NewReader(url.Values{"title": {"Discovery"}, "experience": {tt.proposal}, "claim": {"A possible interpretation"}}.Encode()))
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			r.AddCookie(&http.Cookie{Name: "realize_session", Value: "test"})
			s.serve(httptest.NewRecorder(), r)
			if v.Saved == nil || v.Saved.Experience != tt.expected || v.Saved.SourceText != tt.source {
				t.Fatalf("archive experience = %q, want source-grounded %q", v.Saved.Experience, tt.expected)
			}
			// A changed review field is candidate-authored and must survive.
			v.Stage = "review"
			v.State.Proposal = &discovery.Proposal{Title: "Discovery", Experience: tt.proposal, Claim: "A possible interpretation", SourceID: "response-1", Quote: tt.source}
			edit := "I read the layout sheet aloud with a colleague before opening, after I once used the wrong room number."
			r = httptest.NewRequest("POST", "/keep", strings.NewReader(url.Values{"title": {"Discovery"}, "experience": {edit}, "claim": {"A possible interpretation"}, "correction": {"I now verify the room number with a second staff member."}}.Encode()))
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			r.AddCookie(&http.Cookie{Name: "realize_session", Value: "test"})
			s.serve(httptest.NewRecorder(), r)
			if v.Saved.Experience != edit || len(v.Saved.Responses) != 2 || v.Saved.Responses[1].Source.Text != "I now verify the room number with a second staff member." {
				t.Fatal("candidate-authored wording or correction was not preserved")
			}
		})
	}
}
func TestBasetenRequest(t *testing.T) {
	var body map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" || r.Header.Get("Authorization") != "Bearer test-key" {
			t.Error("bad request")
		}
		b, _ := io.ReadAll(r.Body)
		json.Unmarshal(b, &body)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"content":"{}"}}]}`))
	}))
	defer ts.Close()
	t.Setenv("BASETEN_API_KEY", "test-key")
	t.Setenv("BASETEN_BASE_URL", ts.URL)
	t.Setenv("BASETEN_MODEL", "test-model")
	got, e := generate(context.Background(), "input", "policy")
	if e != nil || got != "{}" || body["model"] != "test-model" {
		t.Fatal("bad transport", e)
	}
	if body["max_tokens"] != float64(3072) || body["reasoning_effort"] != "low" {
		t.Fatal("incorrect discovery generation profile")
	}
	format, ok := body["response_format"].(map[string]any)
	if !ok || format["type"] != "json_object" {
		t.Fatal("missing JSON output profile")
	}
	if bytes.Contains([]byte(got), []byte("test-key")) {
		t.Fatal("key leaked")
	}
}

func TestPastedIntakeAndMultipleDiscoveries(t *testing.T) {
	s := &server{sessions: map[string]*session{}, generate: func(_ context.Context, _, _ string) (string, error) {
		return `{"question":"Tell me about a specific incident.","proposal":null}`, nil
	}}
	v := &session{Stage: "welcome"}
	s.sessions["test"] = v
	post := func(path string, values url.Values) {
		r := httptest.NewRequest("POST", path, strings.NewReader(values.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.AddCookie(&http.Cookie{Name: "realize_session", Value: "test"})
		s.serve(httptest.NewRecorder(), r)
	}
	post("/paste", url.Values{"career_text": {"Skills: troubleshooting and customer communication"}})
	if v.Stage != "preview" || v.State.Resume.Kind != "pasted_text" {
		t.Fatal("pasted provenance lost")
	}
	post("/begin", url.Values{"approach": {"focused"}})
	if v.Stage != "conversation" || v.State.Question == "" {
		t.Fatal("focused interview did not start")
	}
	save := func(statement, title string) {
		v.State.Responses = nil
		v.State.Question = "What did you do?"
		var err error
		v.State, err = discovery.RecordAnswer(v.State, statement)
		if err != nil {
			t.Fatal(err)
		}
		v.State.Proposal = &discovery.Proposal{Title: title, Experience: statement, Claim: statement, SourceID: "response-1", Quote: statement, Unresolved: []string{"Outcome unknown"}}
		v.Stage = "review"
		post("/keep", url.Values{"title": {title}, "experience": {statement}, "claim": {statement}})
	}
	save("I investigated a login problem.", "Investigation")
	post("/explore", url.Values{"focus": {"Customer communication"}})
	save("I explained the findings to the customer.", "Communication")
	if len(v.Discoveries) != 2 || v.Discoveries[0].ID == v.Discoveries[1].ID || v.Discoveries[0].SourceText != "I investigated a login problem." {
		t.Fatal("discoveries not preserved")
	}
	v.Synthesis = &discovery.ConnectionSynthesis{}
	post("/edit", url.Values{"id": {"1"}})
	post("/keep", url.Values{"title": {"Corrected"}, "experience": {"I investigated a login problem."}, "claim": {"I investigated a login problem."}})
	if v.Synthesis != nil || v.Discoveries[0].Revision != 2 || len(v.Discoveries) != 2 {
		t.Fatal("edit did not invalidate synthesis")
	}
}

func TestChallengeWithdrawsAndCorrectionRevises(t *testing.T) {
	st, _ := discovery.New("Worked on incidents and onboarding.")
	st.Question = "What did you do?"
	st, _ = discovery.RecordAnswer(st, "I investigated and handed the issue to engineering.")
	p := discovery.Proposal{Title: "Incident", Experience: "Investigated", Claim: "Investigation", SourceID: "response-1", Quote: "I investigated"}
	st.Proposal = &p
	v := &session{Stage: "connections", State: st, Discoveries: []evidence{{Proposal: p, ID: 1, Revision: 1, OriginalMaterial: st.Resume.Text, Responses: st.Responses}}, Synthesis: &discovery.ConnectionSynthesis{Connections: []discovery.Connection{{ID: "conn-1", Capability: "Leadership", Interpretation: "Led the resolution"}}}}
	s := &server{sessions: map[string]*session{"test": v}}
	post := func(path string, values url.Values) {
		r := httptest.NewRequest("POST", path, strings.NewReader(values.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.AddCookie(&http.Cookie{Name: "realize_session", Value: "test"})
		s.serve(httptest.NewRecorder(), r)
	}
	post("/challenge", url.Values{"connection_id": {"conn-1"}, "challenge": {"I did not lead resolution"}})
	if len(v.Synthesis.Connections) != 0 || v.Challenge == "" || v.PreviousInterpretation == "" {
		t.Fatal("challenge failed to withdraw")
	}
	post("/edit", url.Values{"id": {"1"}})
	post("/keep", url.Values{"title": {"Incident"}, "experience": {"Investigated and handed off"}, "claim": {"Investigation"}, "correction": {"Engineering resolved it; I did not."}})
	if v.Discoveries[0].Revision != 2 || v.Synthesis != nil || len(v.Discoveries[0].Responses) != 2 || v.Discoveries[0].Responses[0].Source.Text != st.Responses[0].Source.Text {
		t.Fatal("correction lost sources or failed to invalidate")
	}
}

func TestFragmentAnswerPreservesRecordedResponseOnInferenceFailure(t *testing.T) {
	st, err := discovery.New("I helped customers investigate recurring login problems.")
	if err != nil {
		t.Fatal(err)
	}
	st.Question = "What did you do?"
	v := &session{Stage: "conversation", State: st}
	s := &server{sessions: map[string]*session{"test": v}, generate: func(context.Context, string, string) (string, error) { return "", errors.New("inference unavailable") }}
	r := httptest.NewRequest("POST", "/answer", strings.NewReader(url.Values{"answer": {"I gathered examples and shared them with engineering."}}.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("X-Realize-Fragment", "1")
	r.AddCookie(&http.Cookie{Name: "realize_session", Value: "test"})
	w := httptest.NewRecorder()
	s.serve(w, r)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "I gathered examples and shared them with engineering.") || !strings.Contains(w.Body.String(), "inference unavailable") || !strings.Contains(w.Body.String(), `action="/retry"`) {
		t.Fatalf("fragment did not preserve answer and retry: %d", w.Code)
	}
	if len(v.State.Responses) != 1 || v.State.Question != "" {
		t.Fatal("answer was not recorded exactly once")
	}
}

func TestWorkstreamCPresentation(t *testing.T) {
	mkProposal := func() discovery.Proposal {
		return discovery.Proposal{Title: "Diagnosis", Headline: "Steady triage", Experience: "I investigated the outage and handed findings to engineering.", Claim: "Careful diagnostic work under pressure.", SourceID: "response-1", Quote: "I diagnosed the issue", Supports: []discovery.ProposalSupport{{SourceID: "response-2", Quote: "I handed my findings to engineering"}}, Unresolved: []string{"Outcome unknown"}}
	}
	mkState := func(p discovery.Proposal) discovery.State {
		st, _ := discovery.New("A useful resume excerpt about troubleshooting.")
		st.Question = "What did you do?"
		st, _ = discovery.RecordAnswer(st, "I diagnosed the issue and kept notes.")
		st.Question = "What happened next?"
		st, _ = discovery.RecordAnswer(st, "I handed my findings to engineering for the fix.")
		st.Proposal = &p
		return st
	}
	render := func(v *session) string {
		var buf bytes.Buffer
		if e := page.Execute(&buf, v); e != nil {
			t.Fatal(e)
		}
		return buf.String()
	}
	p := mkProposal()
	st := mkState(p)
	got := render(&session{Stage: "reveal", State: st})
	for _, want := range []string{"YOU SAID", "inspect the evidence", "Why Realize thinks so", "What remains open", "your exact words", "Your words support a possibility, not a proven conclusion.", "response-2", "exact supporting statement", "YOU WOULD KEEP", "Careful diagnostic work under pressure."} {
		if !strings.Contains(got, want) {
			t.Fatalf("reveal missing %q", want)
		}
	}
	got = render(&session{Stage: "review", State: st})
	for _, want := range []string{"where your responsibility ended", "cited source", "Exact cited passage"} {
		if !strings.Contains(got, want) {
			t.Fatalf("review missing %q", want)
		}
	}
	if !strings.Contains(got, "Your original statements stay unchanged.") {
		t.Fatal("review must duplicate edits notice above Keep button")
	}
	item := evidence{Proposal: p, OriginalMaterial: st.Resume.Text, SourceText: st.Responses[0].Source.Text, ReviewStatus: "candidate_reviewed", ID: 1, Revision: 1, Responses: st.Responses}
	got = render(&session{Stage: "saved", State: st, Discoveries: []evidence{item}})
	for _, want := range []string{"Exact cited passage", "response-1", "cited source", "Supporting statements", "response-2"} {
		if !strings.Contains(got, want) {
			t.Fatalf("saved missing %q", want)
		}
	}
	open := render(&session{Stage: "takeaway", State: st, Discoveries: []evidence{item}, Synthesis: &discovery.ConnectionSynthesis{Connections: []discovery.Connection{{ID: "conn-1", Capability: "Judgment", Interpretation: "Steady triage", Relationship: "Across both", CandidateUse: "Steady triage", Limitations: []string{"Outcome unknown"}, FollowUp: "What would you revisit next time?"}}}})
	for _, want := range []string{"Open questions remain", "cited passage supporting this claim", "A question to consider: What would you revisit next time?", "Next actions", "Download JSON", "Back to discoveries"} {
		if !strings.Contains(open, want) {
			t.Fatalf("takeaway (open) missing %q", want)
		}
	}
	closed := p
	closed.Unresolved = nil
	closedItem := evidence{Proposal: closed, OriginalMaterial: st.Resume.Text, SourceText: "x", ID: 1, Revision: 1, Responses: st.Responses}
	got = render(&session{Stage: "takeaway", State: st, Discoveries: []evidence{closedItem}})
	if !strings.Contains(got, "No open questions identified") {
		t.Fatal("takeaway (resolved) missing Ready to consider")
	}
	if !strings.Contains(pageHTML, "44px") || !strings.Contains(pageHTML, "overflow-wrap:anywhere") || !strings.Contains(pageHTML, "700px") {
		t.Fatal("missing presentation CSS guards")
	}
}

func TestExperimentCookieIsolation(t *testing.T) {
	t.Setenv("REALIZE_COOKIE_NAME", "realize_astra_session")
	s := &server{sessions: map[string]*session{"baseline": {Stage: "saved"}}}
	r := httptest.NewRequest("GET", "/", nil)
	r.AddCookie(&http.Cookie{Name: "realize_session", Value: "baseline"})
	w := httptest.NewRecorder()
	first := s.get(w, r)
	if first.Stage != "welcome" {
		t.Fatal("experiment reused baseline cookie")
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != "realize_astra_session" {
		t.Fatal("missing isolated cookie")
	}
	again := httptest.NewRequest("GET", "/", nil)
	again.AddCookie(cookies[0])
	if s.get(httptest.NewRecorder(), again) != first {
		t.Fatal("experiment session did not survive refresh")
	}
}

func TestQuoteExcerptPreservesPrefix(t *testing.T) {
	short := "My work matters."
	if quoteExcerpt(short) != short {
		t.Fatal("short quote changed")
	}
	long := strings.Repeat("évidence ", 30)
	got := quoteExcerpt(long)
	if len([]rune(got)) > 160 || !strings.HasPrefix(long, got) {
		t.Fatal("excerpt must be an exact bounded prefix")
	}
}

func TestOutcomesFollowReviewAndInvalidateOnCorrection(t *testing.T) {
	st, _ := discovery.New("Customer service experience")
	st.Question = "What did you do?"
	st, _ = discovery.RecordAnswer(st, "I called customers to explain the delay.")
	st.Proposal = &discovery.Proposal{Title: "Communication", Experience: "Called customers about a delay", Claim: "Clear communication", SourceID: "response-1", Quote: "I called customers to explain the delay."}
	v := &session{Stage: "review", State: st}
	s := &server{sessions: map[string]*session{"test": v}, generate: func(context.Context, string, string) (string, error) {
		return `{"resume":[{"evidence_id":1,"evidence_revision":1,"source_id":"response-1","quote":"I called customers to explain the delay.","title":"Customer updates","explanation":"Shows how you kept customers informed.","wording":"Called customers to explain a delay."}],"strengths":[],"possibilities":[]}`, nil
	}}
	post := func(path string, vals url.Values) {
		r := httptest.NewRequest("POST", path, strings.NewReader(vals.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.AddCookie(&http.Cookie{Name: "realize_session", Value: "test"})
		s.serve(httptest.NewRecorder(), r)
	}
	post("/keep", url.Values{"title": {"Communication"}, "experience": {"Called customers about a delay"}, "claim": {"Clear communication"}})
	post("/takeaway", url.Values{})
	if v.Outcomes == nil || len(v.Outcomes.Resume) != 1 || v.Outcomes.Resume[0].Quote != st.Proposal.Quote {
		t.Fatal("outcome or citation missing")
	}
	post("/edit", url.Values{"id": {"1"}})
	post("/keep", url.Values{"title": {"Communication"}, "experience": {"Explained a delay"}, "claim": {"Clear communication"}})
	if v.Outcomes != nil {
		t.Fatal("correction retained stale outcomes")
	}
}

func TestProgressiveDiscoveryConversation(t *testing.T) {
	const statement = "I posted the wrong room number once because I relied on memory. Since then I read the layout sheet aloud with a second staff member before doors open."
	calls := 0
	s := &server{sessions: map[string]*session{}, generate: func(_ context.Context, _, policy string) (string, error) {
		if policy == discovery.OutcomePolicy {
			return `{"resume":[],"strengths":[{"evidence_id":1,"evidence_revision":1,"source_id":"response-2","quote":"Since then I read the layout sheet aloud with a second staff member before doors open.","title":"attention_to_detail","explanation":"","wording":""}],"possibilities":[]}`, nil
		}
		calls++
		switch calls {
		case 1:
			return `{"question":"What happened?","proposal":null}`, nil
		case 2:
			return `{"question":"Could you share an example?","proposal":null}`, nil
		case 3:
			return `{"question":"What happened next?","proposal":{"title":"Checking details","strength":"attention_to_detail","experience":"Invented factual description","claim":"Invented factual claim","source_id":"response-2","quote":"Since then I read the layout sheet aloud with a second staff member before doors open.","unresolved":[]}}`, nil
		default:
			return `{"question":"Why did you choose to check together?","proposal":{"title":"Checking details","strength":"attention_to_detail","experience":"Invented factual description","claim":"Invented factual claim","source_id":"response-2","quote":"Since then I read the layout sheet aloud with a second staff member before doors open.","unresolved":[]}}`, nil
		}
	}}
	v := &session{Stage: "welcome"}
	s.sessions["test"] = v
	post := func(path string, values url.Values) string {
		r := httptest.NewRequest("POST", path, strings.NewReader(values.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.Header.Set("X-Realize-Fragment", "1")
		r.AddCookie(&http.Cookie{Name: "realize_session", Value: "test"})
		w := httptest.NewRecorder()
		s.serve(w, r)
		return w.Body.String()
	}
	post("/paste", url.Values{"career_text": {"I help prepare rooms for events."}})
	post("/begin", url.Values{"approach": {"focused"}})
	body := post("/answer", url.Values{"answer": {"I help prepare rooms for events."}})
	if strings.Contains(body, "SOMETHING WORTH RECOGNIZING") {
		t.Fatal("revealed without a proposal")
	}
	body = post("/answer", url.Values{"answer": {statement}})
	for _, want := range []string{"SOMETHING WORTH RECOGNIZING", "Potential strength: Attention to detail", "Since then I read the layout sheet aloud with a second staff member before doors open.", "What happened next?", `action="/answer"`} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q", want)
		}
	}
	if strings.Contains(body, "Invented factual") || len(v.Discoveries) != 0 {
		t.Fatal("unreviewed factual claim or acceptance leaked")
	}
	body = post("/answer", url.Values{"answer": {"We checked the sheet together."}})
	if strings.Contains(body, "SOMETHING WORTH RECOGNIZING") || !strings.Contains(body, "Why did you choose to check together?") {
		t.Fatal("repeat reveal or conversation stopped")
	}
	post("/reveal", nil)
	post("/review", nil)
	post("/keep", url.Values{"title": {"Checking details"}, "experience": {"Invented factual description"}, "claim": {"Invented factual claim"}})
	if len(v.Discoveries) != 1 || v.Discoveries[0].Experience != statement {
		t.Fatalf("reviewed grounding lost: %+v", v.Discoveries)
	}
	body = post("/takeaway", nil)
	if !strings.Contains(body, "Attention to detail") || !strings.Contains(body, "Since then I read the layout sheet aloud") {
		t.Fatal("takeaway missing grounded strength")
	}
}

func TestProgressiveRequiresKnownStrengthAndAvoidsRepeats(t *testing.T) {
	for _, tc := range []struct {
		name, strength string
		seen           bool
	}{{"unknown", "invented_qualification", false}, {"already shown", "coaching", true}} {
		t.Run(tc.name, func(t *testing.T) {
			st, _ := discovery.New("Event work")
			st.Question = "What did you do?"
			st, _ = discovery.RecordAnswer(st, "I helped new coworkers learn their tasks.")
			v := &session{State: st, RevealedStrengths: map[string]bool{"coaching": tc.seen}}
			s := &server{generate: func(context.Context, string, string) (string, error) {
				return `{"question":"What next?","proposal":{"title":"Coaching","strength":"` + tc.strength + `","experience":"helped coworkers","claim":"may show coaching","source_id":"response-1","quote":"I helped new coworkers learn their tasks.","unresolved":[]}}`, nil
			}}
			s.next(httptest.NewRequest("POST", "/answer", nil), v)
			if v.Progressive != nil || v.State.Question != "What next?" {
				t.Fatal("unexpected reveal or interruption")
			}
		})
	}
}
