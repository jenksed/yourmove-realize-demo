// discovery is a standalone synthetic-data interview runner. The full YourMove
// host injects its existing provider instead of using this small CLI transport.
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	discovery "github.com/jenksed/yourmove-career-discovery"
)

var model = discovery.DevelopmentModel

func generate(ctx context.Context, prompt, policy string) (string, error) {
	key := os.Getenv("MODEL_API_KEY")
	if strings.TrimSpace(key) == "" {
		return "", fmt.Errorf("MODEL_API_KEY is required")
	}
	body, _ := json.Marshal(map[string]any{"model": model, "max_tokens": 2048, "reasoning_effort": "low", "response_format": map[string]string{"type": "json_object"}, "stream": false, "messages": []map[string]string{{"role": "system", "content": policy}, {"role": "user", "content": prompt}}})
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "POST", discovery.Endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)
	client := http.Client{Timeout: 120 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("Meta request failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("Meta HTTP %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, (128<<10)+1))
	if err != nil || len(raw) > 128<<10 {
		return "", fmt.Errorf("invalid response size")
	}
	var result struct {
		Choices []struct {
			Finish  string `json:"finish_reason"`
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage json.RawMessage `json:"usage"`
	}
	if json.Unmarshal(raw, &result) != nil || len(result.Choices) != 1 || result.Choices[0].Finish != "stop" {
		return "", fmt.Errorf("incomplete response")
	}
	fmt.Fprintf(os.Stderr, "Model: %s; elapsed: %.2fs; usage: %s\n", model, time.Since(start).Seconds(), result.Usage)
	return result.Choices[0].Message.Content, nil
}

func main() {
	standard := flag.Bool("standard", false, "Use standard mode instead of synthetic-data Contributor mode")
	flag.Parse()
	if *standard {
		model = discovery.Model
	}
	fmt.Fprintln(os.Stderr, "Synthetic-data demo. Input is sent to Meta. No claim is approved or saved to YourMove by this runner.")
	if !*standard {
		fmt.Fprintln(os.Stderr, "CONTRIBUTOR MODE: Meta may use submitted content for training. Enter only synthetic/public development examples. Use -standard for standard mode.")
	}
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 4096), 16000)
	fmt.Println("Paste one synthetic resume excerpt (one line):")
	if !in.Scan() {
		return
	}
	s, err := discovery.New(in.Text())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for {
		s, err = discovery.Next(context.Background(), generate, s)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if s.Proposal != nil {
			raw, _ := json.MarshalIndent(s.Proposal, "", "  ")
			fmt.Printf("Proposed, pending candidate review:\n%s\n", raw)
		}
		fmt.Println("Muse:", s.Question)
		if len(s.Responses) >= discovery.MaxResponses {
			return
		}
		fmt.Println("Candidate response (blank to stop):")
		if !in.Scan() || strings.TrimSpace(in.Text()) == "" {
			return
		}
		s, err = discovery.RecordAnswer(s, in.Text())
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}
