package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	discovery "github.com/jenksed/yourmove-career-discovery"
)

func TestFixtures(t *testing.T) {
	for _, ext := range []string{"pdf", "docx"} {
		t.Run(ext, func(t *testing.T) {
			b, e := os.ReadFile("../../realize-demo-pack/fixtures/avery-morgan-resume." + ext)
			if e != nil {
				t.Fatal(e)
			}
			text, e := extract("resume."+ext, b)
			if e != nil {
				t.Fatal(e)
			}
			if !strings.Contains(text, "MySQL") || !strings.Contains(text, "Northline") {
				t.Fatal("missing fixture content")
			}
		})
	}
	if _, e := extract("fake.pdf", []byte("not a PDF")); e == nil {
		t.Fatal("accepted invalid PDF")
	}
}
func TestBoundedUTF8(t *testing.T) {
	s, truncated := bounded(strings.Repeat("a", 7999) + "é" + "rest")
	if !truncated || len(s) != 7999 {
		t.Fatalf("bad boundary %d", len(s))
	}
}
func TestReviewAndFailedInference(t *testing.T) {
	source := "I diagnosed the issue and handed my findings to engineering."
	st, _ := discovery.New("A useful resume excerpt about troubleshooting.")
	st.Question = "What did you do?"
	st, _ = discovery.RecordAnswer(st, source)
	s := &server{sessions: map[string]*session{}, generate: func(context.Context, string, string) (string, error) { return "", errors.New("unavailable") }}
	v := &session{Stage: "conversation", State: st}
	s.sessions["test"] = v
	post := func(path string, values url.Values) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", path, strings.NewReader(values.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.AddCookie(&http.Cookie{Name: "realize_session", Value: "test"})
		w := httptest.NewRecorder()
		s.serve(w, r)
		return w
	}
	post("/retry", url.Values{})
	if len(v.State.Responses) != 1 || v.State.Responses[0].Source.Text != source {
		t.Fatal("answer was lost")
	}
	v.State.Proposal = &discovery.Proposal{Title: "Diagnosis", Experience: "Investigated an issue", Claim: "Diagnostic judgment", SourceID: "response-1", Quote: "diagnosed the issue", Unresolved: []string{"Outcome unknown"}}
	v.State.Question = "What happened next?"
	post("/reveal", url.Values{})
	post("/review", url.Values{})
	post("/keep", url.Values{"title": {"Corrected title"}, "experience": {"Investigated and handed off"}, "claim": {"Diagnostic work"}})
	if v.Saved == nil || v.Saved.Title != "Corrected title" || v.Saved.SourceText != source || v.Saved.ReviewStatus != "candidate_reviewed" {
		t.Fatal("review or source not preserved")
	}
	r := httptest.NewRequest("GET", "/download", nil)
	r.AddCookie(&http.Cookie{Name: "realize_session", Value: "test"})
	w := httptest.NewRecorder()
	s.serve(w, r)
	var got evidence
	if json.Unmarshal(w.Body.Bytes(), &got) != nil || got.Quote != "diagnosed the issue" || got.SourceText != source {
		t.Fatal("bad export")
	}
}
func TestBasetenRequest(t *testing.T) {
	var body map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" || r.Header.Get("Authorization") != "Bearer test-key" {
			t.Error("bad request")
		}
		b, _ := io.ReadAll(r.Body)
		json.Unmarshal(b, &body)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"content":"{}"}}]}`))
	}))
	defer ts.Close()
	t.Setenv("BASETEN_API_KEY", "test-key")
	t.Setenv("BASETEN_BASE_URL", ts.URL)
	t.Setenv("BASETEN_MODEL", "test-model")
	got, e := generate(context.Background(), "input", "policy")
	if e != nil || got != "{}" || body["model"] != "test-model" {
		t.Fatal("bad transport", e)
	}
	if bytes.Contains([]byte(got), []byte("test-key")) {
		t.Fatal("key leaked")
	}
}
