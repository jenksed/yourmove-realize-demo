package discovery

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	maxAnswerTraceBytes = 6000
	maxAnswerClaims     = 8
	maxAnswerClaimBytes = 1500
	maxSourceQuoteBytes = 1500
	maxResponseIDBytes  = 64
)

// AnswerTrace is a generated answer with inspectable claims. The host supplies
// trusted bindings from a reviewed Career Evidence revision to recorded
// interview responses; model-provided identity fields are never trusted alone.
type AnswerTrace struct {
	Answer     string        `json:"answer"`
	Claims     []AnswerClaim `json:"claims"`
	Unresolved []string      `json:"unresolved"`
}

// AnswerClaim identifies exact answer text and the original response that
// supports it. This is a resolvable citation, not a semantic truth judgment.
type AnswerClaim struct {
	Claim            string `json:"claim"`
	EvidenceID       int64  `json:"evidence_id"`
	EvidenceRevision int64  `json:"evidence_revision"`
	ResponseID       string `json:"response_id"`
	Quote            string `json:"quote"`
}

// TraceSource is an adapter-created binding to an actual recorded response.
// It must be rebuilt from the reviewed evidence's persisted interview state.
type TraceSource struct {
	EvidenceID       int64  `json:"evidence_id"`
	EvidenceRevision int64  `json:"evidence_revision"`
	ResponseID       string `json:"response_id"`
	ResponseText     string `json:"response_text"`
}

// ValidateAnswerTrace rejects references that do not resolve to the exact
// reviewed evidence revision and recorded candidate response supplied by the
// host. It does not assess whether the quoted words semantically support the
// generated claim; the candidate must inspect the trace and answer.
func ValidateAnswerTrace(trace AnswerTrace, sources []TraceSource) error {
	if !traceText(trace.Answer, maxAnswerTraceBytes) || len(trace.Claims) == 0 || len(trace.Claims) > maxAnswerClaims || len(trace.Unresolved) > 6 || len(sources) == 0 {
		return ErrInvalid
	}
	for _, unknown := range trace.Unresolved {
		if !traceText(unknown, 300) {
			return ErrInvalid
		}
	}
	byIdentity := make(map[string]TraceSource, len(sources))
	for _, source := range sources {
		if source.EvidenceID < 1 || source.EvidenceRevision < 1 || !traceText(source.ResponseID, maxResponseIDBytes) || !traceText(source.ResponseText, 2000) {
			return ErrInvalid
		}
		identity := traceIdentity(source.EvidenceID, source.EvidenceRevision, source.ResponseID)
		if _, exists := byIdentity[identity]; exists {
			return ErrInvalid
		}
		byIdentity[identity] = source
	}
	seen := make(map[string]bool, len(trace.Claims))
	for _, claim := range trace.Claims {
		if !traceText(claim.Claim, maxAnswerClaimBytes) || !strings.Contains(trace.Answer, claim.Claim) ||
			claim.EvidenceID < 1 || claim.EvidenceRevision < 1 || !traceText(claim.ResponseID, maxResponseIDBytes) || !traceText(claim.Quote, maxSourceQuoteBytes) {
			return ErrInvalid
		}
		identity := traceIdentity(claim.EvidenceID, claim.EvidenceRevision, claim.ResponseID)
		source, exists := byIdentity[identity]
		if !exists || !strings.Contains(source.ResponseText, claim.Quote) {
			return ErrInvalid
		}
		reference := identity + "\x00" + claim.Claim + "\x00" + claim.Quote
		if seen[reference] {
			return ErrInvalid
		}
		seen[reference] = true
	}
	return nil
}

func traceIdentity(evidenceID, revision int64, responseID string) string {
	return strings.Join([]string{strconv.FormatInt(evidenceID, 10), strconv.FormatInt(revision, 10), responseID}, "\x00")
}

func traceText(value string, limit int) bool {
	if strings.TrimSpace(value) == "" || len(value) > limit || !utf8.ValidString(value) || strings.ContainsRune(value, 0) {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' {
			return false
		}
	}
	return true
}
