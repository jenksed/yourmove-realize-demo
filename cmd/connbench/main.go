// connbench runs bounded synthetic cross-experience synthesis comparisons.
// Histories: A genuinely transferable, B superficially similar, C ambiguous
// ownership/outcomes. Limits: max 6 requests, synthetic data only.
// Backend: Meta Contributor by default; HACKATHON_MUSE_BACKEND=baseten uses
// Baseten Muse Spark Open with BASETEN_API_KEY (synthetic data only).
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	discovery "github.com/jenksed/yourmove-career-discovery"
)

var calls, failures int
var endpoint, model, apiKey string

func generate(ctx context.Context, prompt, policy string) (string, error) {
	if calls >= 14 {
		return "", fmt.Errorf("request limit reached")
	}
	calls++
	body, _ := json.Marshal(map[string]any{"model": model, "max_tokens": 3072, "reasoning_effort": "low", "response_format": map[string]string{"type": "json_object"}, "stream": false, "messages": []map[string]string{{"role": "system", "content": policy}, {"role": "user", "content": prompt}}})
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	start := time.Now()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		failures++
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 128<<10))
	var result struct {
		Choices []struct {
			Finish  string `json:"finish_reason"`
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			Prompt     int `json:"prompt_tokens"`
			Completion int `json:"completion_tokens"`
		} `json:"usage"`
	}
	lat := time.Since(start)
	if resp.StatusCode != 200 || json.Unmarshal(raw, &result) != nil || len(result.Choices) != 1 {
		failures++
		return "", fmt.Errorf("bad response %d", resp.StatusCode)
	}
	fmt.Printf("call %d: %.1fs in=%d out=%d finish=%s backend_model=%s\n", calls, lat.Seconds(), result.Usage.Prompt, result.Usage.Completion, result.Choices[0].Finish, model)
	if result.Choices[0].Finish != "stop" {
		failures++
		return "", fmt.Errorf("finish=%s", result.Choices[0].Finish)
	}
	return result.Choices[0].Message.Content, nil
}

func historyA() []discovery.ConnectionEvidence {
	return []discovery.ConnectionEvidence{
		{ID: 1, Revision: 1, Title: "MySQL performance diagnosis", Situation: "Production payroll system slowed during peak load", Actions: "I diagnosed a full table scan using EXPLAIN, documented my findings in the incident ticket, and handed implementation to the DBA team", OutcomeState: "unknown", SourceTexts: []string{"I diagnosed a full table scan using EXPLAIN and documented my findings"}},
		{ID: 2, Revision: 1, Title: "Billing integration investigation", Situation: "Customers could not download invoices after a billing integration change", Actions: "I investigated the integration logs, communicated status to the customer, and coordinated a technical handoff to billing engineering", OutcomeState: "unknown", SourceTexts: []string{"I investigated the integration logs and coordinated a technical handoff"}},
		{ID: 3, Revision: 1, Title: "Complex support escalations", Situation: "Repeated complex escalations involving customers, developers and operations", Actions: "I handled escalations by clarifying the technical issue for the customer and coordinating developers and operations through handoff", OutcomeState: "not_recorded", SourceTexts: []string{"I coordinated developers and operations through handoff"}},
	}
}

func historyB() []discovery.ConnectionEvidence {
	return []discovery.ConnectionEvidence{
		{ID: 1, Revision: 1, Title: "MySQL report runs", Situation: "Weekly MySQL reports were due", Actions: "I ran the provided MySQL report script each week and emailed the output file to my manager", OutcomeState: "recorded"},
		{ID: 2, Revision: 1, Title: "MySQL password resets", Situation: "Users requested MySQL access changes", Actions: "I followed the runbook to reset MySQL passwords and closed the tickets", OutcomeState: "recorded"},
		{ID: 3, Revision: 1, Title: "Database checklist", Situation: "Monthly database maintenance window", Actions: "I completed the standard database checklist items assigned to me and marked them done", OutcomeState: "recorded"},
	}
}

func historyC() []discovery.ConnectionEvidence {
	return []discovery.ConnectionEvidence{
		{ID: 1, Revision: 1, Title: "Platform outage response", Situation: "A platform outage affected many customers", Actions: "We worked on the outage and things improved afterwards; my individual part is not separated from the team's work", OutcomeState: "unknown"},
		{ID: 2, Revision: 1, Title: "Customer migration", Situation: "Customers were migrated to a new system", Actions: "The team handled the migration together and customers were moved; who owned which part was not recorded", OutcomeState: "unknown"},
		{ID: 3, Revision: 1, Title: "Process improvement", Situation: "Support handling times were a concern", Actions: "Improvements were made to the support process while I was on the team; my specific contribution is unclear", OutcomeState: "unknown"},
	}
}

func historyD() []discovery.ConnectionEvidence {
	return []discovery.ConnectionEvidence{
		{ID: 1, Revision: 1, Title: "Incident status updates", Situation: "Production incident affecting internal users", Actions: "I sent status updates to stakeholders during the incident; I did not lead the incident or manage responders", OutcomeState: "unknown"},
		{ID: 2, Revision: 1, Title: "Vendor call coordination", Situation: "Vendor troubleshooting call for a network fault", Actions: "I scheduled the vendor call and relayed questions between the vendor and our engineers; decisions stayed with the engineers", OutcomeState: "unknown"},
		{ID: 3, Revision: 1, Title: "Change window notices", Situation: "Planned maintenance window", Actions: "I drafted the maintenance notice and sent reminders; I did not approve or manage the change", OutcomeState: "recorded"},
	}
}

func historyE() []discovery.ConnectionEvidence {
	return []discovery.ConnectionEvidence{
		{ID: 1, Revision: 1, Title: "Database diagnosis", Situation: "Slow reporting database", Actions: "I diagnosed a missing index with EXPLAIN, documented it, and handed implementation to the DBA team", OutcomeState: "unknown"},
		{ID: 2, Revision: 1, Title: "Office move help", Situation: "Team moved offices", Actions: "I carried equipment to the new office and reconnected workstations following a checklist", OutcomeState: "recorded"},
		{ID: 3, Revision: 1, Title: "Training attendance", Situation: "Annual compliance training", Actions: "I attended the required training sessions and completed the quizzes", OutcomeState: "recorded"},
	}
}

func historyF() []discovery.ConnectionEvidence {
	return []discovery.ConnectionEvidence{
		{ID: 1, Revision: 1, Title: "API outage diagnosis", Situation: "Partner API returning errors", Actions: "I traced the failing requests in the logs, documented the pattern, and handed the evidence to the platform team", OutcomeState: "unknown", SourceTexts: []string{"I traced the failing requests in the logs and documented the pattern"}},
		{ID: 2, Revision: 1, Title: "Payment webhook investigation", Situation: "Payment confirmations not arriving", Actions: "I traced webhook deliveries in the logs, documented which events were missing, and handed the evidence to the payments team", OutcomeState: "unknown", SourceTexts: []string{"I traced webhook deliveries in the logs and documented which events were missing"}},
	}
}

func report(name, expect string, s discovery.ConnectionSynthesis, err error) {
	if err != nil {
		failures++
		fmt.Printf("%s: FAIL(validity) %v | expectation: %s\n", name, err, expect)
		return
	}
	fmt.Printf("%s: connections=%d | expectation: %s\n", name, len(s.Connections), expect)
	for _, c := range s.Connections {
		fmt.Printf("  %s capability=%q supports=%d limitations=%q candidate_use=%q follow_up=%q\n    interpretation: %s\n    relationship: %s\n", c.ID, c.Capability, len(c.Supports), c.Limitations, c.CandidateUse, c.FollowUp, c.Interpretation, c.Relationship)
		for _, sup := range c.Supports {
			fmt.Printf("    ev%d r%d action=%q quote=%q\n", sup.EvidenceID, sup.EvidenceRevision, sup.Action, sup.Quote)
		}
	}
}

func main() {
	endpoint, model, apiKey = discovery.Endpoint, discovery.DevelopmentModel, os.Getenv("MODEL_API_KEY")
	if m := os.Getenv("CONNBENCH_MODEL"); m != "" {
		model = m
	}
	if strings.EqualFold(os.Getenv("HACKATHON_MUSE_BACKEND"), "baseten") {
		endpoint, model, apiKey = "https://inference.baseten.co/v1/chat/completions", "meta-models/muse-spark-open-1026", os.Getenv("BASETEN_API_KEY")
	}
	if strings.TrimSpace(apiKey) == "" {
		fmt.Fprintln(os.Stderr, "inference API key required for selected backend")
		os.Exit(1)
	}
	ctx := context.Background()
	runs := []struct {
		name, expect string
		hist         func() []discovery.ConnectionEvidence
	}{
		{"A-transferable", ">=1 connection spanning >=2 records, limitations name missing ownership/outcomes", historyA},
		{"B-superficial", "0 connections (shared MySQL keywords only)", historyB},
		{"C-ambiguous", "0 connections, or limitations state personal ownership/outcomes not established", historyC},
		{"D-coordination-not-leadership", "any connection must limit itself to communication/coordination and must NOT claim people management or incident leadership", historyD},
		{"E-one-strong-two-weak", "0 connections (only one record shows diagnosis/handoff)", historyE},
		{"F-two-record-transfer", ">=1 connection across the 2 records (log-tracing plus documented handoff)", historyF},
	}
	// Two rounds over identical inputs: round-to-round stability is itself a
	// reported comparison. Total bounded calls: 12.
	for round := 1; round <= 2; round++ {
		for _, r := range runs {
			s, err := discovery.SynthesizeConnections(ctx, generate, r.hist())
			report(fmt.Sprintf("round%d/%s", round, r.name), r.expect, s, err)
		}
	}
	fmt.Printf("SUMMARY calls=%d transport_failures=%d\n", calls, failures)
}
