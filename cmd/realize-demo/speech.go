package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

var speechTransport http.RoundTripper = http.DefaultTransport

// transcribe accepts an ephemeral browser recording and never changes session state.
func transcribe(w http.ResponseWriter, r *http.Request) {
	const limit = 10 << 20
	media := strings.ToLower(strings.TrimSpace(strings.Split(r.Header.Get("Content-Type"), ";")[0]))
	switch media {
	case "audio/webm", "audio/mp4", "audio/ogg", "audio/wav", "audio/mpeg":
	default:
		http.Error(w, "Unsupported recording", http.StatusUnsupportedMediaType)
		return
	}
	audio, err := io.ReadAll(io.LimitReader(r.Body, limit+1))
	if err != nil || len(audio) == 0 || len(audio) > limit {
		http.Error(w, "Invalid recording", http.StatusBadRequest)
		return
	}
	key := strings.TrimSpace(os.Getenv("DEEPGRAM_API_KEY"))
	if key == "" {
		http.Error(w, "Transcription unavailable", http.StatusServiceUnavailable)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.deepgram.com/v1/listen?model=nova-3&smart_format=true", bytes.NewReader(audio))
	if err != nil {
		http.Error(w, "Transcription unavailable", http.StatusBadGateway)
		return
	}
	req.Header.Set("Authorization", "Token "+key)
	req.Header.Set("Content-Type", media)
	client := http.Client{Transport: speechTransport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "Transcription unavailable", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	var result struct {
		Results struct {
			Channels []struct {
				Alternatives []struct {
					Transcript string `json:"transcript"`
				} `json:"alternatives"`
			} `json:"channels"`
		} `json:"results"`
	}
	if resp.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&result) != nil || len(result.Results.Channels) == 0 || len(result.Results.Channels[0].Alternatives) == 0 {
		http.Error(w, "Transcription unavailable", http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"transcript": result.Results.Channels[0].Alternatives[0].Transcript})
}

// speak synthesizes only the question supplied from the locked session.
func speak(w http.ResponseWriter, r *http.Request, question, voice string) {
	voiceID := env("FISH_AUDIO_VOICE_ID", "e3cd384158934cc9a01029cd7d278634")
	if voice == "masculine" {
		voiceID = env("FISH_AUDIO_MASCULINE_VOICE_ID", "bf322df2096a46f18c579d0baa36f41d")
	}
	key := strings.TrimSpace(os.Getenv("FISH_AUDIO_API_KEY"))
	if key == "" {
		http.Error(w, "Speech unavailable", http.StatusServiceUnavailable)
		return
	}
	body, _ := json.Marshal(map[string]string{"text": question, "reference_id": voiceID, "format": "mp3"})
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.fish.audio/v1/tts", bytes.NewReader(body))
	if err != nil {
		http.Error(w, "Speech unavailable", http.StatusBadGateway)
		return
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("model", "s2.1-pro-free")
	client := http.Client{Transport: speechTransport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "Speech unavailable", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.HasPrefix(resp.Header.Get("Content-Type"), "audio/") {
		http.Error(w, "Speech unavailable", http.StatusBadGateway)
		return
	}
	audio, err := io.ReadAll(io.LimitReader(resp.Body, (5<<20)+1))
	if err != nil || len(audio) == 0 || len(audio) > 5<<20 {
		http.Error(w, "Speech unavailable", http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "audio/mpeg")
	w.Write(audio)
}
