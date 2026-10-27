package discovery

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestRecordedSourceAndAdaptiveContext(t *testing.T) {
	s, _ := New("Provided Linux and database troubleshooting support.")
	s.Question = "What did you personally do?"
	answer := "I reviewed MySQL slow query logs and used EXPLAIN. Another team implemented the changes. I do not know the performance impact."
	s, err := RecordAnswer(s, answer)
	if err != nil {
		t.Fatal(err)
	}
	if s.Question != "" || !strings.Contains(s.Transcript(), answer) {
		t.Fatal("answer was not retained before inference")
	}
	p := Proposal{Title: "MySQL investigation", Experience: "Investigated slow queries", Claim: "Reviewed MySQL slow query logs and used EXPLAIN", SourceID: "response-1", Quote: answer, Unresolved: []string{"Performance impact unknown; another team implemented changes."}}
	raw, _ := json.Marshal(map[string]any{"question": "How did you share the diagnosis?", "proposal": p})
	next, err := Next(context.Background(), func(_ context.Context, prompt, policy string) (string, error) {
		if !strings.Contains(prompt, answer) || !strings.Contains(prompt, "What did you personally do?") || !strings.Contains(policy, "diagnosis from implementation") {
			t.Fatal("missing adaptive context or ownership boundary")
		}
		return string(raw), nil
	}, s)
	if err != nil || next.Proposal == nil || next.Proposal.SourceID != "response-1" {
		t.Fatalf("%+v %v", next, err)
	}
	if next.Responses[0].Source.Text != answer {
		t.Fatal("model replaced source")
	}
}

func TestRejectInventedSourcesAndMalformedProposals(t *testing.T) {
	s, _ := New("Provided troubleshooting support.")
	s.Question = "What happened?"
	s, _ = RecordAnswer(s, "I diagnosed slow queries. Another team implemented the fix.")
	for _, raw := range []string{
		`{"question":"What next?","proposal":{"title":"X","experience":"Y","claim":"Z","source_id":"response-99","quote":"I diagnosed slow queries.","unresolved":[]}}`,
		`{"question":"What next?","proposal":{"title":"X","experience":"Y","claim":"Z","source_id":"response-1","quote":"I improved performance by 90%","unresolved":[]}}`,
		`{"question":"What next?","proposal":{"title":"X","experience":"Y","claim":"Z","source_id":"resume","quote":"Provided troubleshooting support.","unresolved":[]}}`,
		`{"question":"What next?","proposal":null,"approved":true}`,
		`{"question":"What next?","proposal":null} {"approved":true}`,
	} {
		next, err := Next(context.Background(), func(context.Context, string, string) (string, error) { return raw, nil }, s)
		if err == nil || next.Proposal != nil || next.Responses[0].Source.Text != s.Responses[0].Source.Text {
			t.Fatalf("accepted invalid output: %s", raw)
		}
	}
}

func TestInvestigativeFollowupPrefersJudgmentOverTechnicalDetail(t *testing.T) {
	for _, want := range []string{
		"distinguish diagnosis from implementation",
		"why they chose that approach",
		"how they knew their diagnosis or recommendation was useful",
		"what happened where their responsibility ended",
		"confirmation, feedback, or follow-through",
		"Respect an explicit boundary or unknown",
		"over further technical specifics",
	} {
		if !strings.Contains(Policy, want) {
			t.Fatalf("interview policy lost investigative direction %q", want)
		}
	}
}

func TestInputBudgetsAndResponseLimit(t *testing.T) {
	if _, err := New(strings.Repeat("x", 8001)); err == nil {
		t.Fatal("oversized resume accepted")
	}
	s, _ := New("Support engineer")
	for i := 0; i < MaxResponses; i++ {
		s.Question = "What did you do?"
		var err error
		s, err = RecordAnswer(s, "Investigated an incident")
		if err != nil {
			t.Fatal(err)
		}
	}
	s.Question = "Anything else?"
	if _, err := RecordAnswer(s, "More"); err == nil {
		t.Fatal("unbounded interview")
	}
}
