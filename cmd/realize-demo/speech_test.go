package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func providerResponse(status int, kind, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": []string{kind}}, Body: io.NopCloser(strings.NewReader(body))}
}
func TestTranscription(t *testing.T) {
	t.Setenv("DEEPGRAM_API_KEY", "test-key")
	old := speechTransport
	defer func() { speechTransport = old }()
	speechTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Query().Get("model") != "nova-3" || r.Header.Get("Authorization") != "Token test-key" {
			t.Error("incorrect provider request")
		}
		return providerResponse(200, "application/json", `{"results":{"channels":[{"alternatives":[{"transcript":"Hello there."}]}]}}`), nil
	})
	for _, tc := range []struct {
		media, body string
		want        int
	}{{"audio/webm;codecs=opus", "audio", 200}, {"text/plain", "audio", 415}, {"audio/wav", "", 400}, {"audio/wav", strings.Repeat("x", (10<<20)+1), 400}} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/speech/transcribe", strings.NewReader(tc.body))
		r.Header.Set("Content-Type", tc.media)
		transcribe(w, r)
		if w.Code != tc.want {
			t.Fatalf("%s: got %d", tc.media, w.Code)
		}
		if tc.want == 200 && !strings.Contains(w.Body.String(), "Hello there.") {
			t.Fatal(w.Body.String())
		}
	}
	speechTransport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		return providerResponse(503, "application/json", `{"secret":"private"}`), nil
	})
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/speech/transcribe", strings.NewReader("audio"))
	r.Header.Set("Content-Type", "audio/mp4")
	transcribe(w, r)
	if w.Code != 502 || strings.Contains(w.Body.String(), "private") {
		t.Fatal("provider failure leaked or accepted")
	}
}
func TestQuestionSpeech(t *testing.T) {
	t.Setenv("FISH_AUDIO_API_KEY", "test-key")
	old := speechTransport
	defer func() { speechTransport = old }()
	speechTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(r.Body)
		if r.Header.Get("model") != "s2.1-pro-free" || r.Header.Get("Authorization") != "Bearer test-key" || !bytes.Contains(body, []byte("Current question?")) {
			t.Error("incorrect Fish request")
		}
		return providerResponse(200, "audio/mpeg", "mp3-data"), nil
	})
	w := httptest.NewRecorder()
	speak(w, httptest.NewRequest("POST", "/speech/question", nil), "Current question?")
	if w.Code != 200 || w.Body.String() != "mp3-data" {
		t.Fatal("audio missing")
	}
	speechTransport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		return providerResponse(402, "application/json", "private provider error"), nil
	})
	w = httptest.NewRecorder()
	speak(w, httptest.NewRequest("POST", "/speech/question", nil), "Current question?")
	if w.Code != 502 || strings.Contains(w.Body.String(), "private") {
		t.Fatal("provider failure leaked or accepted")
	}
}

func TestSpeechDoesNotRecordAnswer(t *testing.T) {
	t.Setenv("DEEPGRAM_API_KEY", "test-key")
	old := speechTransport
	defer func() { speechTransport = old }()
	speechTransport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		return providerResponse(200, "application/json", `{"results":{"channels":[{"alternatives":[{"transcript":"unreviewed text"}]}]}}`), nil
	})
	v := &session{Stage: "conversation", Notice: "preserve"}
	v.State.Question = "What happened?"
	s := &server{sessions: map[string]*session{"test": v}}
	r := httptest.NewRequest("POST", "/speech/transcribe", strings.NewReader("audio"))
	r.Header.Set("Content-Type", "audio/webm")
	r.AddCookie(&http.Cookie{Name: "realize_session", Value: "test"})
	w := httptest.NewRecorder()
	s.serve(w, r)
	if w.Code != 200 || len(v.State.Responses) != 0 || v.State.Question != "What happened?" || v.Notice != "preserve" {
		t.Fatal("transcription changed session state")
	}
	v.Stage = "review"
	r = httptest.NewRequest("POST", "/speech/question", nil)
	r.AddCookie(&http.Cookie{Name: "realize_session", Value: "test"})
	w = httptest.NewRecorder()
	s.serve(w, r)
	if w.Code != http.StatusConflict {
		t.Fatalf("speech allowed in review: %d", w.Code)
	}
}
