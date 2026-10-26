// benchmark runs a bounded synthetic Contributor-mode evaluation.
// Limits: max 8 requests, synthetic data only. Usage: MODEL_API_KEY=... go run ./cmd/benchmark
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
var totalLatency time.Duration

func generate(ctx context.Context, prompt, policy string) (string, error) {
	if calls >= 8 {
		return "", fmt.Errorf("request limit reached")
	}
	calls++
	key := os.Getenv("MODEL_API_KEY")
	body, _ := json.Marshal(map[string]any{"model": discovery.DevelopmentModel, "max_tokens": 2048, "reasoning_effort": "low", "response_format": map[string]string{"type": "json_object"}, "stream": false, "messages": []map[string]string{{"role": "system", "content": policy}, {"role": "user", "content": prompt}}})
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
	totalLatency += lat
	if resp.StatusCode != 200 || json.Unmarshal(raw, &result) != nil || len(result.Choices) != 1 {
		failures++
		return "", fmt.Errorf("bad response %d", resp.StatusCode)
	}
	fmt.Printf("call %d: %.1fs in=%d out=%d finish=%s\n", calls, lat.Seconds(), result.Usage.Prompt, result.Usage.Completion, result.Choices[0].Finish)
	return result.Choices[0].Message.Content, nil
}

func main() {
	if strings.TrimSpace(os.Getenv("MODEL_API_KEY")) == "" {
		fmt.Fprintln(os.Stderr, "MODEL_API_KEY required")
		os.Exit(1)
	}
	resumeBytes, _ := os.ReadFile("benchmarks/synthetic-resume.txt")
	resume := strings.TrimSpace(string(resumeBytes))
	ctx := context.Background()
	leads, err := discovery.FindLeads(ctx, generate, resume)
	if err != nil {
		fmt.Println("LEADS FAIL:", err)
		os.Exit(1)
	}
	fmt.Printf("LEADS (%d):\n", len(leads))
	for _, l := range leads {
		fmt.Printf("- %s | %s\n", l.Title, l.Why)
	}
	s, _ := discovery.New(resume)
	s.Leads = leads
	s, err = discovery.SelectLead(s, leads[0].ID)
	if err != nil {
		fmt.Println("SELECT FAIL:", err)
		os.Exit(1)
	}
	answers := []string{
		"I checked the MySQL slow query log during an outage, found a full table scan, and used EXPLAIN to confirm it. I suggested an index; the DBA team applied the change in production.",
		"I decided to recommend the index because EXPLAIN showed no index was used. I did not measure the final improvement and I do not know the exact timing result.",
		"I wrote up the finding in the ticket so the DBA could act. The customer impact I saw was the payroll page loading again, but I cannot prove the index alone caused it.",
	}
	for _, a := range answers {
		s, err = discovery.Next(ctx, generate, s)
		if err != nil {
			fmt.Println("NEXT retry after:", err)
			s, err = discovery.Next(ctx, generate, s)
		}
		if err != nil {
			fmt.Println("NEXT FAIL:", err)
			break
		}
		fmt.Println("Q:", s.Question)
		if s.Proposal != nil {
			fmt.Printf("PROPOSAL: %s | claim=%s | unresolved=%v\n", s.Proposal.Title, s.Proposal.Claim, s.Proposal.Unresolved)
			ok := s.ValidateProposal() == nil
			fmt.Println("proposal source-valid:", ok)
		}
		s, err = discovery.RecordAnswer(s, a)
		if err != nil {
			fmt.Println("RECORD FAIL:", err)
			break
		}
		fmt.Println("A recorded:", a[:60], "...")
	}
	fmt.Printf("SUMMARY calls=%d failures=%d avg_latency=%.1fs\n", calls, failures, totalLatency.Seconds()/float64(max(calls, 1)))
}
