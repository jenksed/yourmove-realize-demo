package discovery

import (
	"errors"
	"strings"
	"testing"
)

func validAnswerTrace() (AnswerTrace, []TraceSource) {
	answer := "I used EXPLAIN to find a full table scan, then sent the logs and diagnostic notes to database engineering."
	source := TraceSource{EvidenceID: 42, EvidenceRevision: 3, ResponseID: "response-2", ResponseText: "EXPLAIN showed a full table scan for a frequently used query. I sent the slow query log entries, EXPLAIN output, and my diagnostic notes to the database engineering team. They owned selecting and implementing the fix. I do not know the measured outcome."}
	trace := AnswerTrace{Answer: answer, Claims: []AnswerClaim{{Claim: "I used EXPLAIN to find a full table scan", EvidenceID: 42, EvidenceRevision: 3, ResponseID: "response-2", Quote: "EXPLAIN showed a full table scan for a frequently used query"}}}
	return trace, []TraceSource{source}
}

func TestValidateAnswerTraceResolvesReviewedResponse(t *testing.T) {
	trace, sources := validAnswerTrace()
	if err := ValidateAnswerTrace(trace, sources); err != nil {
		t.Fatalf("valid trace rejected: %v", err)
	}
}

func TestValidateAnswerTraceRejectsUnresolvableReferences(t *testing.T) {
	tests := []struct {
		name string
		edit func(*AnswerTrace, *[]TraceSource)
	}{
		{"stale evidence revision", func(trace *AnswerTrace, _ *[]TraceSource) { trace.Claims[0].EvidenceRevision++ }},
		{"missing response", func(trace *AnswerTrace, _ *[]TraceSource) { trace.Claims[0].ResponseID = "response-9" }},
		{"wrong evidence binding", func(trace *AnswerTrace, _ *[]TraceSource) { trace.Claims[0].EvidenceID++ }},
		{"claim not in answer", func(trace *AnswerTrace, _ *[]TraceSource) { trace.Claims[0].Claim = "I implemented the fix" }},
		{"quote not in recorded response", func(trace *AnswerTrace, _ *[]TraceSource) { trace.Claims[0].Quote = "I implemented the fix" }},
		{"empty claim", func(trace *AnswerTrace, _ *[]TraceSource) { trace.Claims[0].Claim = " " }},
		{"oversized claim", func(trace *AnswerTrace, _ *[]TraceSource) {
			trace.Claims[0].Claim = strings.Repeat("x", maxAnswerClaimBytes+1)
		}},
		{"control character", func(trace *AnswerTrace, _ *[]TraceSource) { trace.Claims[0].Quote = "bad\x01quote" }},
		{"duplicate reference", func(trace *AnswerTrace, _ *[]TraceSource) { trace.Claims = append(trace.Claims, trace.Claims[0]) }},
		{"duplicate trusted binding", func(_ *AnswerTrace, sources *[]TraceSource) { *sources = append(*sources, (*sources)[0]) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			trace, sources := validAnswerTrace()
			test.edit(&trace, &sources)
			if err := ValidateAnswerTrace(trace, sources); !errors.Is(err, ErrInvalid) {
				t.Fatalf("expected ErrInvalid, got %v", err)
			}
		})
	}
}
