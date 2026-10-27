package discovery

import (
	"context"
	"testing"
)

func testConnEvidences() []ConnectionEvidence {
	return []ConnectionEvidence{
		{ID: 1, Revision: 2, Title: "MySQL diagnosis", Situation: "Payroll outage, slow queries", Actions: "I diagnosed the slow query with EXPLAIN and documented findings; the DBA team applied the index", OutcomeState: "unknown", SourceTexts: []string{"I diagnosed the slow query with EXPLAIN and documented findings"}},
		{ID: 2, Revision: 1, Title: "Billing escalation", Situation: "Customer could not access invoices", Actions: "I explained the billing cause to the customer and coordinated the handoff to billing engineering", OutcomeState: "recorded", SourceTexts: []string{"I explained the billing cause to the customer"}},
		{ID: 3, Revision: 1, Title: "Support escalation", Situation: "Complex customer escalation across teams", Actions: "I coordinated support, developers and operations and communicated status until handoff", OutcomeState: "unknown"},
	}
}

const validSynthesisJSON = `{"connections":[{"id":"conn-1","capability":"Cross-functional technical problem-solving","interpretation":"Across three contexts the candidate diagnosed issues and moved them across team boundaries.","relationship":"Each record shows diagnosis plus a documented handoff to another team.","supports":[{"evidence_id":1,"evidence_revision":2,"quote":"I diagnosed the slow query with EXPLAIN","action":"Diagnosed and documented"},{"evidence_id":2,"evidence_revision":1,"quote":"coordinated the handoff to billing engineering","action":"Explained and handed off"}],"limitations":["Implementation ownership stayed with other teams","Outcomes not recorded for two records"],"candidate_use":"I can explain how I diagnose and hand off across contexts","follow_up":"Did any handoff recipient confirm your diagnosis was acted on?"}]}`

func TestSynthesizeConnectionsValid(t *testing.T) {
	gen := func(_ context.Context, payload, policy string) (string, error) {
		if payload == "" || policy == "" {
			t.Fatal("empty contract")
		}
		return validSynthesisJSON, nil
	}
	got, err := SynthesizeConnections(context.Background(), gen, testConnEvidences())
	if err != nil || len(got.Connections) != 1 || len(got.Connections[0].Supports) != 2 {
		t.Fatal(got, err)
	}
}

func TestSynthesizeConnectionsRejectsFabricatedQuote(t *testing.T) {
	gen := func(context.Context, string, string) (string, error) {
		return `{"connections":[{"id":"conn-1","capability":"Leadership","interpretation":"x","relationship":"y","supports":[{"evidence_id":1,"evidence_revision":2,"quote":"I led a team of twelve engineers","action":"Led"},{"evidence_id":2,"evidence_revision":1,"quote":"I explained the billing cause","action":"Explained"}],"limitations":["none established"],"candidate_use":"I lead teams"}]}`, nil
	}
	if _, err := SynthesizeConnections(context.Background(), gen, testConnEvidences()); err == nil {
		t.Fatal("fabricated quote accepted")
	}
}

func TestSynthesizeConnectionsRejectsSingleRecordAndWrongRevision(t *testing.T) {
	single := func(context.Context, string, string) (string, error) {
		return `{"connections":[{"id":"conn-1","capability":"Diagnosis","interpretation":"x","relationship":"y","supports":[{"evidence_id":1,"evidence_revision":2,"quote":"I diagnosed the slow query with EXPLAIN","action":"Diagnosed"},{"evidence_id":1,"evidence_revision":2,"quote":"documented findings; the DBA team applied the index","action":"Documented"}],"limitations":["Ownership unclear"],"candidate_use":"I diagnose"}]}`, nil
	}
	if _, err := SynthesizeConnections(context.Background(), single, testConnEvidences()); err == nil {
		t.Fatal("single-record connection accepted")
	}
	stale := func(context.Context, string, string) (string, error) {
		return `{"connections":[{"id":"conn-1","capability":"Diagnosis","interpretation":"x","relationship":"y","supports":[{"evidence_id":1,"evidence_revision":9,"quote":"I diagnosed the slow query with EXPLAIN","action":"Diagnosed"},{"evidence_id":2,"evidence_revision":1,"quote":"I explained the billing cause","action":"Explained"}],"limitations":["Ownership unclear"],"candidate_use":"I diagnose"}]}`, nil
	}
	if _, err := SynthesizeConnections(context.Background(), stale, testConnEvidences()); err == nil {
		t.Fatal("wrong revision accepted")
	}
}

func TestSynthesizeConnectionsAllowsEmptyAndRequiresTwoRecords(t *testing.T) {
	empty := func(context.Context, string, string) (string, error) { return `{"connections":[]}`, nil }
	got, err := SynthesizeConnections(context.Background(), empty, testConnEvidences())
	if err != nil || len(got.Connections) != 0 {
		t.Fatal(got, err)
	}
	if _, err := SynthesizeConnections(context.Background(), empty, testConnEvidences()[:1]); err == nil {
		t.Fatal("one record accepted as synthesis input")
	}
	if err := ValidateSynthesis(ConnectionSynthesis{}, testConnEvidences()); err != nil {
		t.Fatal("persisted empty synthesis should revalidate", err)
	}
}
