package discovery

import (
	"context"
	"strings"
	"testing"
)

func TestFindLeadsValidatesAndSelectFocusesNext(t *testing.T) {
	gen := func(_ context.Context, prompt, policy string) (string, error) {
		if strings.Contains(policy, "investigation leads") {
			return `{"leads":[{"id":"lead-1","title":"Database incident","why":"May reveal diagnosis method"},{"id":"lead-2","title":"Customer escalation","why":"May reveal collaboration"},{"id":"lead-3","title":"Automation work","why":"May reveal technical method"}]}`, nil
		}
		if !strings.Contains(prompt, "Database incident") {
			t.Fatal("selected lead focus absent from prompt")
		}
		return `{"question":"What did you personally check first?","proposal":null}`, nil
	}
	leads, err := FindLeads(context.Background(), gen, "Provided database and customer support.")
	if err != nil || len(leads) != 3 {
		t.Fatal(leads, err)
	}
	s, _ := New("Provided database and customer support.")
	s.Leads = leads
	if _, err := SelectLead(s, "lead-9"); err == nil {
		t.Fatal("unknown lead accepted")
	}
	s, err = SelectLead(s, "lead-1")
	if err != nil || s.SelectedLeadID != "lead-1" {
		t.Fatal(s, err)
	}
	s, err = Next(context.Background(), gen, s)
	if err != nil || s.Question == "" {
		t.Fatal(s, err)
	}
}

func TestFindLeadsRejectsTooFewAndBadIDs(t *testing.T) {
	bad := func(_ context.Context, _, _ string) (string, error) {
		return `{"leads":[{"id":"x","title":"One","why":"w"}]}`, nil
	}
	if _, err := FindLeads(context.Background(), bad, "resume text"); err == nil {
		t.Fatal("invalid leads accepted")
	}
}
