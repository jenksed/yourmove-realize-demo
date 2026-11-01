package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	discovery "github.com/jenksed/yourmove-career-discovery"
)

func TestConversationStoppingAndDiscoveryContinuity(t *testing.T) {
	for _, tc := range []struct {
		name, answer, stage string
		calls               int
	}{
		{"cannot remember", "No, I honestly don't remember.", "paused", 0},
		{"change topic", "Can we talk about something else?", "paused", 0},
		{"end interview", "I want to stop the interview.", "paused", 0},
		{"productive partial", "I don't remember the customer's name, but I remember how we resolved the issue.", "conversation", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st, _ := discovery.New("Customer service experience")
			st.Question = "What happened?"
			calls := 0
			v := &session{Stage: "conversation", State: st}
			srv := &server{sessions: map[string]*session{"test": v}, generate: func(context.Context, string, string) (string, error) {
				calls++
				return `{"question":"What did you do to resolve it?","proposal":null}`, nil
			}}
			r := httptest.NewRequest("POST", "/answer", strings.NewReader(url.Values{"answer": {tc.answer}}.Encode()))
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			r.AddCookie(&http.Cookie{Name: "realize_session", Value: "test"})
			srv.serve(httptest.NewRecorder(), r)
			if v.Stage != tc.stage || calls != tc.calls || len(v.State.Responses) != 1 {
				t.Fatalf("stage=%s calls=%d responses=%d", v.Stage, calls, len(v.State.Responses))
			}
		})
	}
	st, _ := discovery.New("Customer service experience")
	st.Question = "What did you do?"
	st, _ = discovery.RecordAnswer(st, "I called the customer and explained the delay.")
	st.Proposal = &discovery.Proposal{Title: "Communication", Experience: "Called the customer", Claim: "Explained the delay", SourceID: "response-1", Quote: "I called the customer and explained the delay."}
	st.Question = "What happened next?"
	v := &session{Stage: "conversation", State: st}
	srv := &server{sessions: map[string]*session{"test": v}, generate: func(context.Context, string, string) (string, error) { t.Fatal("unexpected inference"); return "", nil }}
	post := func(path string, values url.Values) {
		r := httptest.NewRequest("POST", path, strings.NewReader(values.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.AddCookie(&http.Cookie{Name: "realize_session", Value: "test"})
		srv.serve(httptest.NewRecorder(), r)
	}
	post("/answer", url.Values{"answer": {"Can we finish now?"}})
	if v.State.ValidateProposal() != nil || len(v.Discoveries) != 0 || !v.EndRequested {
		t.Fatal("pending proposal lost or accepted")
	}
	post("/explore", url.Values{"focus": {"Another experience"}})
	if v.Stage != "paused" || v.State.ValidateProposal() != nil {
		t.Fatal("pending proposal discarded when changing topic")
	}
	post("/reveal", nil)
	post("/review", nil)
	if v.Stage != "review" || len(v.Discoveries) != 0 {
		t.Fatal("proposal not reviewable")
	}
	post("/continue", nil)
	if v.Stage != "paused" {
		t.Fatal("completion resumed questions")
	}
}

func TestTopicChangeOffersDifferentLead(t *testing.T) {
	st, _ := discovery.New("Customer service and event coordination")
	st.Leads = []discovery.Lead{{ID: "lead-1", Title: "Customer complaint", Why: "Service"}, {ID: "lead-2", Title: "Event setup", Why: "Coordination"}, {ID: "lead-3", Title: "Training", Why: "Coaching"}}
	st.SelectedLeadID = "lead-1"
	st.Question = "What happened with the complaint?"
	calls := 0
	v := &session{Stage: "conversation", State: st, OriginalLeads: st.Leads}
	srv := &server{sessions: map[string]*session{"test": v}, generate: func(_ context.Context, prompt, policy string) (string, error) {
		calls++
		if !strings.Contains(prompt, "Event setup") {
			t.Errorf("wrong selected lead: %s", prompt)
		}
		return `{"question":"How did you prepare the event?","proposal":null}`, nil
	}}
	post := func(path string, values url.Values) string {
		r := httptest.NewRequest("POST", path, strings.NewReader(values.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.AddCookie(&http.Cookie{Name: "realize_session", Value: "test"})
		w := httptest.NewRecorder()
		srv.serve(w, r)
		return w.Body.String()
	}
	post("/answer", url.Values{"answer": {"Let's move on."}})
	if v.Stage != "paused" || calls != 0 {
		t.Fatal("topic boundary not enforced")
	}
	body := post("/explore", url.Values{})
	if v.Stage != "leads" || strings.Contains(body, `value="lead-1"`) {
		t.Fatal("closed lead remained selectable")
	}
	post("/select", url.Values{"lead": {"lead-1"}})
	if calls != 0 {
		t.Fatal("closed lead accepted")
	}
	post("/select", url.Values{"lead": {"lead-2"}})
	if calls != 1 || v.State.Question != "How did you prepare the event?" {
		t.Fatal("different lead was not explored")
	}
}

func pendingBoundarySession() *session {
	st, _ := discovery.New("Customer service experience")
	st.Question = "What did you do?"
	st, _ = discovery.RecordAnswer(st, "I called the customer and explained the delay.")
	st.Proposal = &discovery.Proposal{Title: "Communication", Experience: "Called the customer", Claim: "Explained the delay", SourceID: "response-1", Quote: "I called the customer and explained the delay."}
	st.Question = "What happened next?"
	saved := evidence{Proposal: *st.Proposal, ID: 1, Revision: 1, OriginalMaterial: st.Resume.Text, SourceText: st.Responses[0].Source.Text, ReviewStatus: "candidate_reviewed", Responses: st.Responses}
	return &session{Stage: "paused", State: st, Discoveries: []evidence{saved}, EndRequested: true, BoundaryPaused: true}
}

func postTo(srv *server, path string, values url.Values) {
	r := httptest.NewRequest("POST", path, strings.NewReader(values.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.AddCookie(&http.Cookie{Name: "realize_session", Value: "test"})
	srv.serve(httptest.NewRecorder(), r)
}

func TestPendingProposalSurvivesCollectionAndExplore(t *testing.T) {
	newServer := func(v *session) *server {
		return &server{sessions: map[string]*session{"test": v}, generate: func(context.Context, string, string) (string, error) {
			return `{"question":"What did you do differently?","proposal":null}`, nil
		}}
	}
	t.Run("collection then explore preserves pending", func(t *testing.T) {
		v := pendingBoundarySession()
		srv := newServer(v)
		postTo(srv, "/collection", url.Values{})
		if v.Stage != "saved" || v.State.ValidateProposal() != nil {
			t.Fatalf("collection lost pending: stage=%s", v.Stage)
		}
		postTo(srv, "/explore", url.Values{"focus": {"Another experience"}})
		if v.Stage != "paused" || v.State.ValidateProposal() != nil {
			t.Fatalf("explore discarded pending: stage=%s", v.Stage)
		}
		postTo(srv, "/edit", url.Values{"id": {"1"}})
		if v.Stage != "paused" || v.Editing != 0 || v.State.ValidateProposal() != nil {
			t.Fatal("edit replaced the pending proposal")
		}
		postTo(srv, "/reveal", nil)
		postTo(srv, "/review", nil)
		if v.Stage != "review" {
			t.Fatalf("pending not reviewable after navigation: stage=%s", v.Stage)
		}
		postTo(srv, "/reject", nil)
		if v.State.ValidateProposal() == nil || v.Stage != "paused" || len(v.Discoveries) != 1 {
			t.Fatal("reject did not clear pending normally")
		}
	})
	t.Run("accept consumes pending then explore proceeds", func(t *testing.T) {
		v := pendingBoundarySession()
		srv := newServer(v)
		postTo(srv, "/collection", url.Values{})
		postTo(srv, "/reveal", nil)
		postTo(srv, "/review", nil)
		postTo(srv, "/keep", url.Values{"title": {"Communication"}, "experience": {"Called the customer about a delay"}, "claim": {"Clear communication"}})
		if len(v.Discoveries) != 2 || v.Stage != "saved" || v.State.ValidateProposal() == nil {
			t.Fatal("accept did not save and consume the pending proposal")
		}
		postTo(srv, "/explore", url.Values{"focus": {"Another experience"}})
		if v.Stage != "conversation" || v.State.ValidateProposal() == nil {
			t.Fatal("explore did not proceed after accept")
		}
	})
}

func TestEmptyEndReachesTerminalSummary(t *testing.T) {
	calls := 0
	st, _ := discovery.New("Customer service experience")
	st.Question = "What did you do?"
	st, _ = discovery.RecordAnswer(st, "I handled routine questions.")
	v := &session{Stage: "paused", State: st, EndRequested: true, BoundaryPaused: true}
	srv := &server{sessions: map[string]*session{"test": v}, generate: func(context.Context, string, string) (string, error) {
		calls++
		return `{"question":"What next?","proposal":null}`, nil
	}}
	var buf bytes.Buffer
	if e := page.Execute(&buf, v); e != nil {
		t.Fatal(e)
	}
	paused := buf.String()
	if !strings.Contains(paused, "See your summary") {
		t.Fatal("paused terminal control missing for empty end")
	}
	if strings.Contains(paused, `action="/explore"`) {
		t.Fatal("explore form leaks into the empty end")
	}
	postTo(srv, "/takeaway", url.Values{})
	if v.Stage != "takeaway" || calls != 0 || v.Outcomes != nil {
		t.Fatalf("empty takeaway required inference or missed: stage=%s calls=%d", v.Stage, calls)
	}
	buf.Reset()
	if e := page.Execute(&buf, v); e != nil {
		t.Fatal(e)
	}
	got := buf.String()
	for _, want := range []string{"no summary to show", "Start over", "Explore an experience"} {
		if !strings.Contains(got, want) {
			t.Fatalf("terminal summary missing %q", want)
		}
	}
	for _, bad := range []string{"reviewed discoveries are available below", "Download JSON", "Back to discoveries"} {
		if strings.Contains(got, bad) {
			t.Fatalf("empty terminal fabricates continuity: %q", bad)
		}
	}
}
