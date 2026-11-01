package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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

func TestSpeechAllowancesAndKillSwitch(t *testing.T) {
	t.Setenv("REALIZE_SPEECH_ENABLED", "true")
	t.Setenv("REALIZE_STT_SESSION_LIMIT", "1")
	t.Setenv("REALIZE_STT_SERVER_LIMIT", "2")
	t.Setenv("REALIZE_TTS_SESSION_LIMIT", "1")
	t.Setenv("REALIZE_TTS_SERVER_LIMIT", "2")
	s := &server{}
	a, b := &session{}, &session{}
	if !s.reserveSpeech(a, false) || s.reserveSpeech(a, false) {
		t.Fatal("session transcription allowance not enforced")
	}
	if !s.reserveSpeech(b, false) || s.reserveSpeech(&session{}, false) {
		t.Fatal("server transcription allowance not enforced")
	}
	if !s.reserveSpeech(a, true) || s.reserveSpeech(a, true) {
		t.Fatal("session TTS allowance not enforced")
	}
	if !s.reserveSpeech(b, true) || s.reserveSpeech(&session{}, true) {
		t.Fatal("server TTS allowance not enforced")
	}
	t.Setenv("REALIZE_SPEECH_ENABLED", "off")
	if s.reserveSpeech(&session{}, true) {
		t.Fatal("kill switch allowed provider request")
	}
}

func TestDefaultSTTSessionAllowanceAndOverride(t *testing.T) {
	t.Setenv("REALIZE_SPEECH_ENABLED", "true")
	t.Setenv("REALIZE_STT_SESSION_LIMIT", "")
	t.Setenv("REALIZE_STT_SERVER_LIMIT", "50")
	t.Setenv("DEEPGRAM_API_KEY", "test-key")
	old := speechTransport
	defer func() { speechTransport = old }()
	speechTransport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		return providerResponse(200, "application/json", `{"results":{"channels":[{"alternatives":[{"transcript":"heard words"}]}]}}`), nil
	})
	request := func(s *server, cookie string) int {
		r := httptest.NewRequest("POST", "/speech/transcribe", strings.NewReader("audio"))
		r.Header.Set("Content-Type", "audio/webm")
		r.AddCookie(&http.Cookie{Name: "realize_session", Value: cookie})
		w := httptest.NewRecorder()
		s.serve(w, r)
		return w.Code
	}
	newServer := func(cookie string) *server {
		state, _ := discovery.New("Resume material long enough for interview")
		state.Question = "What happened?"
		return &server{sessions: map[string]*session{cookie: {Stage: "conversation", State: state}}}
	}
	defaultServer := newServer("default")
	for i := 0; i < 6; i++ {
		if got := request(defaultServer, "default"); got != http.StatusOK {
			t.Fatalf("default STT request %d returned %d; want 200", i+1, got)
		}
	}
	if got := request(defaultServer, "default"); got != http.StatusTooManyRequests {
		t.Fatalf("seventh default STT request returned %d; want 429", got)
	}

	t.Setenv("REALIZE_STT_SESSION_LIMIT", "2")
	overrideServer := newServer("override")
	for i := 0; i < 2; i++ {
		if got := request(overrideServer, "override"); got != http.StatusOK {
			t.Fatalf("override STT request %d returned %d; want 200", i+1, got)
		}
	}
	if got := request(overrideServer, "override"); got != http.StatusTooManyRequests {
		t.Fatalf("request above override returned %d; want 429", got)
	}
}

func TestSpeechRoutesReleaseSessionLockDuringProviderCall(t *testing.T) {
	t.Setenv("REALIZE_SPEECH_ENABLED", "true")
	t.Setenv("FISH_AUDIO_API_KEY", "test-key")
	t.Setenv("REALIZE_TTS_SESSION_LIMIT", "1")
	entered, release := make(chan struct{}), make(chan struct{})
	old := speechTransport
	defer func() { speechTransport = old }()
	speechTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(r.Body)
		if !bytes.Contains(body, []byte("authorized old question")) {
			t.Error("provider received a question outside the locked snapshot")
		}
		close(entered)
		<-release
		return providerResponse(200, "audio/mpeg", "audio"), nil
	})
	state, _ := discovery.New("Resume material long enough for interview")
	state.Question = "authorized old question"
	v := &session{Stage: "conversation", State: state, Voice: "feminine"}
	s := &server{sessions: map[string]*session{"session": v}}
	r := httptest.NewRequest("POST", "/speech/question", nil)
	r.AddCookie(&http.Cookie{Name: "realize_session", Value: "session"})
	w := httptest.NewRecorder()
	done := make(chan struct{})
	go func() { s.serve(w, r); close(done) }()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("provider was not called")
	}
	changed := make(chan struct{})
	go func() { v.mu.Lock(); v.State.Question = "new question"; v.mu.Unlock(); close(changed) }()
	select {
	case <-changed:
	case <-time.After(time.Second):
		t.Fatal("session lock remained held during provider I/O")
	}
	close(release)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("speech handler did not finish")
	}
	if w.Code != 200 || w.Body.String() != "audio" || v.State.Question != "new question" {
		t.Fatal("provider response disrupted the changed session")
	}
}

func TestSpeechRouteLimitsAndDisabledLeaveTextAvailable(t *testing.T) {
	t.Setenv("REALIZE_SPEECH_ENABLED", "false")
	state, _ := discovery.New("Resume material long enough for interview")
	state.Question = "What happened?"
	v := &session{Stage: "conversation", State: state}
	s := &server{sessions: map[string]*session{"s": v}}
	for _, path := range []string{"/speech/question", "/speech/transcribe"} {
		r := httptest.NewRequest("POST", path, strings.NewReader("audio"))
		r.Header.Set("Content-Type", "audio/webm")
		r.AddCookie(&http.Cookie{Name: "realize_session", Value: "s"})
		w := httptest.NewRecorder()
		s.serve(w, r)
		if w.Code != http.StatusServiceUnavailable || strings.Contains(w.Body.String(), "API_KEY") {
			t.Fatalf("%s: status %d body %q", path, w.Code, w.Body.String())
		}
	}
	if v.State.Question != "What happened?" {
		t.Fatal("disabled speech changed interview state")
	}
}

func TestSpeechEndpointSessionAndServerLimits(t *testing.T) {
	t.Setenv("REALIZE_SPEECH_ENABLED", "true")
	t.Setenv("FISH_AUDIO_API_KEY", "test-key")
	t.Setenv("DEEPGRAM_API_KEY", "test-key")
	t.Setenv("REALIZE_TTS_SESSION_LIMIT", "1")
	t.Setenv("REALIZE_TTS_SERVER_LIMIT", "1")
	t.Setenv("REALIZE_STT_SESSION_LIMIT", "1")
	t.Setenv("REALIZE_STT_SERVER_LIMIT", "1")
	old := speechTransport
	defer func() { speechTransport = old }()
	speechTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if strings.Contains(r.URL.Host, "fish.audio") {
			return providerResponse(200, "audio/mpeg", "audio"), nil
		}
		return providerResponse(200, "application/json", `{"results":{"channels":[{"alternatives":[{"transcript":"heard words"}]}]}}`), nil
	})
	state, _ := discovery.New("Resume material long enough for interview")
	state.Question = "What happened?"
	s := &server{sessions: map[string]*session{
		"one": {Stage: "conversation", State: state},
		"two": {Stage: "conversation", State: state},
	}}
	request := func(path, cookie, contentType string) int {
		r := httptest.NewRequest("POST", path, strings.NewReader("recording"))
		r.Header.Set("Content-Type", contentType)
		r.AddCookie(&http.Cookie{Name: "realize_session", Value: cookie})
		w := httptest.NewRecorder()
		s.serve(w, r)
		return w.Code
	}
	if got := request("/speech/question", "one", "application/octet-stream"); got != http.StatusOK {
		t.Fatalf("first TTS request: %d", got)
	}
	if got := request("/speech/question", "one", "application/octet-stream"); got != http.StatusTooManyRequests {
		t.Fatalf("session TTS limit: %d", got)
	}
	if got := request("/speech/question", "two", "application/octet-stream"); got != http.StatusTooManyRequests {
		t.Fatalf("server TTS limit: %d", got)
	}
	if got := request("/speech/transcribe", "one", "audio/webm"); got != http.StatusOK {
		t.Fatalf("first STT request: %d", got)
	}
	if got := request("/speech/transcribe", "one", "audio/webm"); got != http.StatusTooManyRequests {
		t.Fatalf("session STT limit: %d", got)
	}
	if got := request("/speech/transcribe", "two", "audio/webm"); got != http.StatusTooManyRequests {
		t.Fatalf("server STT limit: %d", got)
	}
}

func TestQuestionRouteUsesSessionVoiceReference(t *testing.T) {
	t.Setenv("REALIZE_SPEECH_ENABLED", "true")
	t.Setenv("FISH_AUDIO_API_KEY", "test-key")
	t.Setenv("FISH_AUDIO_VOICE_ID", "laura-reference")
	t.Setenv("FISH_AUDIO_MASCULINE_VOICE_ID", "adrian-reference")
	old := speechTransport
	defer func() { speechTransport = old }()
	var references []string
	speechTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var payload struct {
			Reference string `json:"reference_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		references = append(references, payload.Reference)
		return providerResponse(200, "audio/mpeg", "audio"), nil
	})
	state, _ := discovery.New("Resume material long enough for interview")
	state.Question = "What happened?"
	s := &server{sessions: map[string]*session{
		"laura":  {Stage: "conversation", State: state, Voice: "feminine"},
		"adrian": {Stage: "conversation", State: state, Voice: "masculine"},
	}}
	for _, tc := range []struct{ cookie, want string }{{"laura", "laura-reference"}, {"adrian", "adrian-reference"}} {
		r := httptest.NewRequest("POST", "/speech/question", nil)
		r.AddCookie(&http.Cookie{Name: "realize_session", Value: tc.cookie})
		w := httptest.NewRecorder()
		s.serve(w, r)
		if w.Code != http.StatusOK {
			t.Fatalf("%s route status: %d", tc.cookie, w.Code)
		}
		if references[len(references)-1] != tc.want {
			t.Fatalf("%s selected %q, want %q", tc.cookie, references[len(references)-1], tc.want)
		}
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
