package discovery

import (
	"context"
	"strings"
	"testing"
)

func TestGenerateOutcomesRequiresRecordedQuoteAndRevision(t *testing.T) {
	sources := []OutcomeSource{{ID: 1, Revision: 2, Experience: "Helped customers", Claim: "Communication", Responses: []Response{{Source: Source{ID: "response-1", Kind: "self_report", Text: "I called customers to explain the delay."}}}}}
	good := `{"resume":[{"evidence_id":1,"evidence_revision":2,"source_id":"response-1","quote":"I called customers to explain the delay.","title":"Customer communication","explanation":"This shows how you kept customers informed.","wording":"Called customers to explain delays."}],"strengths":[],"possibilities":[]}`
	for _, tc := range []struct {
		name, raw string
		valid     bool
	}{{"valid", good, true}, {"invented quote", strings.Replace(good, "I called customers to explain the delay.", "I resolved every delay", 1), false}, {"stale revision", strings.Replace(good, `"evidence_revision":2`, `"evidence_revision":1`, 1), false}, {"wrong source", strings.Replace(good, `"source_id":"response-1"`, `"source_id":"response-2"`, 1), false}} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := GenerateOutcomes(context.Background(), func(context.Context, string, string) (string, error) { return tc.raw, nil }, sources)
			if (err == nil) != tc.valid {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestOutcomesDiscardModelWrittenFacts(t *testing.T) {
	sources := []OutcomeSource{{ID: 1, Revision: 1, Responses: []Response{{Source: Source{ID: "response-1", Kind: "self_report", Text: "I called two customers the next day."}}}}}
	raw := `{"resume":[{"evidence_id":1,"evidence_revision":1,"source_id":"response-1","quote":"I called two customers the next day.","title":"Promised updates","explanation":"I provided an update.","wording":"Delivered promised updates."}],"strengths":[{"evidence_id":1,"evidence_revision":1,"source_id":"response-1","quote":"I called two customers the next day.","title":"follow_through","explanation":"You fulfilled a promise.","wording":""},{"evidence_id":1,"evidence_revision":1,"source_id":"response-1","quote":"I called two customers the next day.","title":"You promised to call","explanation":"","wording":""}],"possibilities":[{"evidence_id":1,"evidence_revision":1,"source_id":"response-1","quote":"I called two customers the next day.","title":"service_coordination","explanation":"You already managed service teams.","wording":""}]}`
	out, err := GenerateOutcomes(context.Background(), func(context.Context, string, string) (string, error) { return raw, nil }, sources)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Resume) != 1 || out.Resume[0].Wording != "I called two customers the next day." || out.Resume[0].Title != "Your words" || out.Resume[0].Explanation != "" {
		t.Fatalf("resume leaked model text: %+v", out.Resume)
	}
	if len(out.Strengths) != 1 || out.Strengths[0].Title != "Follow-through" || out.Strengths[0].Explanation != "" {
		t.Fatalf("strength leaked model text: %+v", out.Strengths)
	}
	if len(out.Possibilities) != 1 || out.Possibilities[0].Title != "Service coordination" || out.Possibilities[0].Explanation != "" {
		t.Fatalf("possibility leaked model text: %+v", out.Possibilities)
	}
}
