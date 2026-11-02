package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
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
	}{{"audio/webm;codecs=opus", "audio", 200}, {"text/plain", "audio", 415}, {"audio/wav", "", 400}, {"audio/wav", strings.Repeat("x", (10<<20)+1), 413}} {
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
	if w.Code != 503 || strings.Contains(w.Body.String(), "private") {
		t.Fatal("provider failure leaked or accepted")
	}
}

func TestSafariMP4CodecContentTypeReachesDeepgramAsMP4(t *testing.T) {
	t.Setenv("DEEPGRAM_API_KEY", "test-key")
	old := speechTransport
	defer func() { speechTransport = old }()
	speechTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Content-Type") != "audio/mp4" {
			t.Errorf("Deepgram Content-Type = %q, want audio/mp4", r.Header.Get("Content-Type"))
		}
		return providerResponse(200, "application/json", `{"results":{"channels":[{"alternatives":[{"transcript":"mobile recording"}]}]}}`), nil
	})
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/speech/transcribe", strings.NewReader("recorded bytes"))
	r.Header.Set("Content-Type", "audio/mp4;codecs=mp4a.40.2")
	r.Header.Set("X-Recording-Duration-Ms", "5000")
	transcribe(w, r)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "mobile recording") || w.Header().Get("X-Realize-Request-ID") == "" {
		t.Fatalf("Safari MP4 request status=%d body=%q request_id=%q", w.Code, w.Body.String(), w.Header().Get("X-Realize-Request-ID"))
	}
}

func TestTranscriptionNoSpeechIsNotProviderFailure(t *testing.T) {
	t.Setenv("DEEPGRAM_API_KEY", "test-key")
	old := speechTransport
	defer func() { speechTransport = old }()
	speechTransport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		return providerResponse(200, "application/json", `{"results":{"channels":[]}}`), nil
	})
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/speech/transcribe", strings.NewReader("recorded bytes"))
	r.Header.Set("Content-Type", "audio/mp4")
	transcribe(w, r)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"transcript":""`) {
		t.Fatalf("no-speech response status=%d body=%q", w.Code, w.Body.String())
	}
}

func TestConversationQuestionPlaybackWiring(t *testing.T) {
	state, _ := discovery.New("A short fictional career summary for the conversation view.")
	state.Question = "What did you do when the process changed?"
	var output bytes.Buffer
	if err := page.Execute(&output, &session{Stage: "conversation", State: state}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`class="question-row"`, `aria-label="Play or replay question"`,
		`.voice-controls .speak-question{display:grid`, `width:44px;height:44px`,
		`const startsInterview=new URL(form.action).pathname==='/begin'`,
		`if(startsInterview&&!next.querySelector('.notice'))autoQuestionPlayback=true`,
		`nextQuestion.textContent!==previousQuestion && autoQuestionPlayback`,
		`@media(max-width:700px){.composer{position:static}`,
		`questionAudioText`, `err.name==='NotAllowedError'`,
	} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("conversation voice playback is missing %q", want)
		}
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

func TestTranscriptionAuthFailureLogsSanitizedDiagnostics(t *testing.T) {
	t.Setenv("DEEPGRAM_API_KEY", "test-key-sentinel-xyz")
	old := speechTransport
	defer func() { speechTransport = old }()
	speechTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Authorization") != "Token test-key-sentinel-xyz" {
			t.Error("incorrect provider request")
		}
		return providerResponse(401, "application/json", `{"err_code":"INVALID_AUTH","err_msg":"Invalid credentials.","request_id":"01a1975b-fa30-7e92-829b-e81902280f87"}`), nil
	})
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/speech/transcribe", strings.NewReader("audio"))
	r.Header.Set("Content-Type", "audio/webm")
	transcribe(w, r)
	if w.Code != http.StatusBadGateway {
		t.Fatalf("auth failure status=%d", w.Code)
	}
	logs := buf.String()
	for _, want := range []string{"provider_err_code=INVALID_AUTH", "provider_request_id=01a1975b-fa30-7e92-829b-e81902280f87", "provider_status=401"} {
		if !strings.Contains(logs, want) {
			t.Fatalf("diagnostic log missing %q in %q", want, logs)
		}
	}
	if strings.Contains(logs, "test-key-sentinel-xyz") || strings.Contains(w.Body.String(), "INVALID_AUTH") {
		t.Fatal("sensitive diagnostic leaked")
	}
}

func TestDeepgramAuthDiagnostic(t *testing.T) {
	oldTransport, oldOutput := speechTransport, log.Writer()
	defer func() { speechTransport = oldTransport; log.SetOutput(oldOutput) }()
	for _, tc := range []struct {
		name, flag, key, body, want string
		status                      int
		request                     bool
	}{
		{"disabled", "", "sentinel-secret", "", "", 0, false},
		{"missing", "1", "  ", "", `status=0 err_code="" key_present=false surrounding_quotes=false embedded_prefix=false`, 0, false},
		{"accepted", "1", "  sentinel-secret  ", `{"secret":"sentinel-secret"}`, `status=200 err_code="" key_present=true surrounding_quotes=false embedded_prefix=false`, 200, true},
		{"rejected", "1", `"Bearer sentinel-secret"`, `{"err_code":"INVALID_AUTH","err_msg":"sentinel-secret"}`, `status=401 err_code="INVALID_AUTH" key_present=true surrounding_quotes=true embedded_prefix=true`, 401, true},
		{"untrusted code", "1", "Token sentinel-secret", `{"err_code":"sentinel-secret"}`, `status=403 err_code="" key_present=true surrounding_quotes=false embedded_prefix=true`, 403, true},
		{"network", "1", "sentinel-secret", "", `status=0 err_code="" key_present=true surrounding_quotes=false embedded_prefix=false`, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("DEEPGRAM_AUTH_DIAGNOSTIC", tc.flag)
			t.Setenv("DEEPGRAM_API_KEY", tc.key)
			var logs bytes.Buffer
			log.SetOutput(&logs)
			called := false
			speechTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				called = true
				if r.Method != http.MethodGet || r.URL.String() != "https://api.deepgram.com/v1/auth/token" || r.Header.Get("Authorization") != "Token "+strings.TrimSpace(tc.key) {
					t.Error("unexpected diagnostic request")
				}
				if tc.status == 0 {
					return nil, io.ErrUnexpectedEOF
				}
				return providerResponse(tc.status, "application/json", tc.body), nil
			})
			deepgramAuthDiagnostic()
			if called != tc.request || !strings.Contains(logs.String(), tc.want) || strings.Contains(logs.String(), "sentinel-secret") {
				t.Errorf("unexpected request or sanitized log: called=%t log=%q", called, logs.String())
			}
			if tc.flag == "" && logs.Len() != 0 {
				t.Error("disabled diagnostic logged output")
			}
		})
	}
}
