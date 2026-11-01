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
