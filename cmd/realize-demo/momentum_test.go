package main

import (
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
