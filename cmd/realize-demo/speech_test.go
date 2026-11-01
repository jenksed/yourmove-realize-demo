package main

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	discovery "github.com/jenksed/yourmove-career-discovery"
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
	t.Setenv("FISH_AUDIO_VOICE_ID", "audition-voice-id")
	wantVoice := "audition-voice-id"
	old := speechTransport
	defer func() { speechTransport = old }()
	speechTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(r.Body)
		if r.Header.Get("model") != "s2.1-pro-free" || r.Header.Get("Authorization") != "Bearer test-key" || !bytes.Contains(body, []byte("Current question?")) || !bytes.Contains(body, []byte(`"reference_id":"`+wantVoice+`"`)) {
			t.Error("incorrect Fish request")
		}
		return providerResponse(200, "audio/mpeg", "mp3-data"), nil
	})
	w := httptest.NewRecorder()
	speak(w, httptest.NewRequest("POST", "/speech/question", nil), "Current question?", "feminine")
	if w.Code != 200 || w.Body.String() != "mp3-data" {
		t.Fatal("audio missing")
	}
	t.Setenv("FISH_AUDIO_VOICE_ID", "")
	wantVoice = "e3cd384158934cc9a01029cd7d278634"
	w = httptest.NewRecorder()
	speak(w, httptest.NewRequest("POST", "/speech/question", nil), "Current question?", "feminine")
	if w.Code != 200 {
		t.Fatal("default voice failed")
	}
	t.Setenv("FISH_AUDIO_MASCULINE_VOICE_ID", "")
	wantVoice = "bf322df2096a46f18c579d0baa36f41d"
	w = httptest.NewRecorder()
	speak(w, httptest.NewRequest("POST", "/speech/question", nil), "Current question?", "masculine")
	if w.Code != 200 {
		t.Fatal("masculine voice failed")
	}
	speechTransport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		return providerResponse(402, "application/json", "private provider error"), nil
	})
	w = httptest.NewRecorder()
	speak(w, httptest.NewRequest("POST", "/speech/question", nil), "Current question?", "feminine")
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

func TestVoiceSelectionRoute(t *testing.T) {
	t.Setenv("FISH_AUDIO_API_KEY", "test-key")
	t.Setenv("FISH_AUDIO_VOICE_ID", "")
	t.Setenv("FISH_AUDIO_MASCULINE_VOICE_ID", "")
	old := speechTransport
	defer func() { speechTransport = old }()
	var got string
	speechTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		b, _ := io.ReadAll(r.Body)
		got = string(b)
		return providerResponse(200, "audio/mpeg", "audio"), nil
	})
	for _, tc := range []struct{ choice, want string }{
		{"", "e3cd384158934cc9a01029cd7d278634"},
		{"feminine", "e3cd384158934cc9a01029cd7d278634"},
		{"masculine", "bf322df2096a46f18c579d0baa36f41d"},
	} {
		st, _ := discovery.New("I helped customers solve problems at work.")
		v := &session{Stage: "preview", InputKind: "paste", State: st}
		s := &server{sessions: map[string]*session{"test": v}, generate: func(context.Context, string, string) (string, error) {
			return `{"question":"What did you do?","proposal":null}`, nil
		}}
		post := func(path, body string) *httptest.ResponseRecorder {
			r := httptest.NewRequest("POST", path, strings.NewReader(body))
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			r.AddCookie(&http.Cookie{Name: "realize_session", Value: "test"})
			w := httptest.NewRecorder()
			s.serve(w, r)
			return w
		}
		post("/begin", "approach=focused&voice="+tc.choice)
		if v.Stage != "conversation" || v.Voice != tc.choice {
			t.Fatalf("begin: stage=%s voice=%s", v.Stage, v.Voice)
		}
		for _, body := range []string{"", "voice=masculine&text=Injected", "voice=feminine"} {
			got = ""
			w := post("/speech/question", body)
			if w.Code != 200 || !strings.Contains(got, tc.want) || strings.Contains(got, "Injected") || !strings.Contains(got, "What did you do?") {
				t.Fatalf("speech request: status=%d body=%s", w.Code, got)
			}
		}
		post("/begin", "approach=focused&voice=masculine")
		if v.Voice != tc.choice || len(v.State.Responses) != 0 {
			t.Fatal("voice or answers changed during conversation")
		}
		post("/reset", "")
		if v.Voice != "" {
			t.Fatal("voice did not reset")
		}
	}
}
