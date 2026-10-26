// rolebench runs bounded synthetic Contributor-mode role comparisons.
// Limits: max 6 requests, synthetic data only.
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

func generate(ctx context.Context, prompt, policy string) (string, error) {
	if calls >= 6 {
		return "", fmt.Errorf("request limit reached")
	}
	calls++
	key := os.Getenv("MODEL_API_KEY")
	body, _ := json.Marshal(map[string]any{"model": discovery.DevelopmentModel, "max_tokens": 3072, "reasoning_effort": "low", "response_format": map[string]string{"type": "json_object"}, "stream": false, "messages": []map[string]string{{"role": "system", "content": policy}, {"role": "user", "content": prompt}}})
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "POST", discovery.Endpoint, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)
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
	fmt.Printf("call %d: %.1fs in=%d out=%d finish=%s\n", calls, lat.Seconds(), result.Usage.Prompt, result.Usage.Completion, result.Choices[0].Finish)
	out := result.Choices[0].Message.Content
	if result.Choices[0].Finish != "stop" {
		failures++
		return "", fmt.Errorf("finish=%s", result.Choices[0].Finish)
	}
	return out, nil
}

func supportRole() discovery.RoleProfile {
	return discovery.RoleProfile{Key: "support-engineer", Title: "Technical Support Engineer", Focus: "Linux troubleshooting, MySQL diagnostics, production incidents and escalation.",
		Requirements: []string{"Diagnose production Linux and MySQL issues", "Document incidents and hand off to engineering", "Own customer accounts and renewal targets"}}
}

func accountRole() discovery.RoleProfile {
	return discovery.RoleProfile{Key: "account-manager", Title: "Technical Account Manager", Focus: "Customer-facing technical communication, coordination and escalation.",
		Requirements: []string{"Explain technical issues clearly to customers", "Coordinate across support and engineering teams", "Apply production database changes directly"}}
}

func fullSet() []discovery.RoleEvidence {
	return []discovery.RoleEvidence{
		{ID: 1, Revision: 2, Title: "MySQL slow-query diagnosis", Situation: "Payroll outage with a full table scan on the jobs table", Actions: "Checked the MySQL slow query log, used EXPLAIN to confirm, suggested an index in the ticket; the DBA team applied the production change", OutcomeState: "unknown"},
		{ID: 2, Revision: 1, Title: "Customer billing escalation", Situation: "A customer could not access invoices after a plan change", Actions: "Explained the access cause in plain language, coordinated with billing support, and confirmed the customer could download invoices; did not own the account or renewal", OutcomeState: "recorded"},
	}
}

func weakSet() []discovery.RoleEvidence {
	return []discovery.RoleEvidence{
		{ID: 3, Revision: 1, Title: "Wiki updates", Situation: "Support procedures changed", Actions: "Updated wiki pages when procedures changed", OutcomeState: "not_recorded"},
	}
}

func report(name string, analysis discovery.RoleAnalysis, err error) {
	if err != nil {
		fmt.Printf("%s: FAIL %v\n", name, err)
		return
	}
	fmt.Printf("%s:\n", name)
	for _, item := range analysis.Items {
		fmt.Printf("  ev%d r%d relevant=%v caps=%q gaps=%q\n  why: %s\n", item.EvidenceID, item.EvidenceRevision, item.Relevant, item.Capabilities, item.Gaps, item.Explanation)
	}
	fmt.Printf("  unmet=%q\n", analysis.Unmet)
}

func main() {
	if strings.TrimSpace(os.Getenv("MODEL_API_KEY")) == "" {
		fmt.Fprintln(os.Stderr, "MODEL_API_KEY required")
		os.Exit(1)
	}
	ctx := context.Background()
	a1, e1 := discovery.AnalyzeRole(ctx, generate, supportRole(), fullSet())
	report("support/full", a1, e1)
	a2, e2 := discovery.AnalyzeRole(ctx, generate, accountRole(), fullSet())
	report("account/full", a2, e2)
	a3, e3 := discovery.AnalyzeRole(ctx, generate, supportRole(), weakSet())
	report("support/weak-only", a3, e3)
	// Candidate correction: DBA attribution removed, unknown timing explicit.
	corrected := fullSet()
	corrected[0].Actions = "Checked the MySQL slow query log and suggested the team look at indexing; another team chose and applied the fix"
	a4, e4 := discovery.AnalyzeRole(ctx, generate, supportRole(), corrected)
	report("support/corrected-attribution", a4, e4)
	fmt.Printf("SUMMARY calls=%d failures=%d\n", calls, failures)
}
