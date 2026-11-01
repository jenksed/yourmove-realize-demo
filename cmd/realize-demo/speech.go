package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

var speechTransport http.RoundTripper = http.DefaultTransport

func speechRequestID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "unavailable"
	}
	return hex.EncodeToString(b)
}

func transcriptionError(w http.ResponseWriter, requestID string, status int, code, message string) {
	w.Header().Set("X-Realize-Request-ID", requestID)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"code": code, "message": message})
}

func recordingDuration(r *http.Request) string {
	n, err := strconv.ParseInt(r.Header.Get("X-Recording-Duration-Ms"), 10, 64)
	if err != nil || n < 0 || n > 120_000 {
		return "unknown"
	}
	return strconv.FormatInt(n, 10)
}

// transcribe accepts an ephemeral browser recording and never changes session state.
func transcribe(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	requestID := speechRequestID()
	const limit = 10 << 20
	contentType := strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Type")))
	media := strings.TrimSpace(strings.Split(contentType, ";")[0])
	finish := func(phase string, status, providerStatus, size int) {
		log.Printf("speech_transcribe request_id=%s phase=%s mime=%q bytes=%d duration_ms=%s status=%d provider_status=%d elapsed_ms=%d", requestID, phase, contentType, size, recordingDuration(r), status, providerStatus, time.Since(started).Milliseconds())
		w.Header().Set("X-Realize-Request-ID", requestID)
	}
	switch media {
	case "audio/webm", "audio/mp4", "audio/ogg", "audio/wav", "audio/mpeg", "audio/aac", "audio/m4a", "audio/x-m4a":
	default:
		finish("unsupported_mime", http.StatusUnsupportedMediaType, 0, 0)
		transcriptionError(w, requestID, http.StatusUnsupportedMediaType, "unsupported_audio", "This recording format is not supported. Try again or type your answer.")
		return
	}
	audio, err := io.ReadAll(io.LimitReader(r.Body, limit+1))
	if err != nil || len(audio) == 0 {
		finish("empty_or_unreadable_recording", http.StatusBadRequest, 0, len(audio))
		transcriptionError(w, requestID, http.StatusBadRequest, "empty_recording", "No audio was received. Check your microphone, try again, or type your answer.")
		return
	}
	if len(audio) > limit {
		finish("upload_too_large", http.StatusRequestEntityTooLarge, 0, len(audio))
		transcriptionError(w, requestID, http.StatusRequestEntityTooLarge, "recording_too_large", "This recording is too large. Make a shorter recording or type your answer.")
		return
	}
	key := strings.TrimSpace(os.Getenv("DEEPGRAM_API_KEY"))
	if key == "" {
		finish("provider_not_configured", http.StatusServiceUnavailable, 0, len(audio))
		transcriptionError(w, requestID, http.StatusServiceUnavailable, "speech_unavailable", "Transcription is unavailable right now. You can type your answer.")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.deepgram.com/v1/listen?model=nova-3&smart_format=true", bytes.NewReader(audio))
	if err != nil {
		finish("request_build_failed", http.StatusBadGateway, 0, len(audio))
		transcriptionError(w, requestID, http.StatusBadGateway, "speech_unavailable", "Transcription is unavailable right now. You can type your answer.")
		return
	}
	req.Header.Set("Authorization", "Token "+key)
	upstreamMedia := media
	if media == "audio/m4a" || media == "audio/x-m4a" {
		upstreamMedia = "audio/mp4"
	}
	req.Header.Set("Content-Type", upstreamMedia)
	client := http.Client{Transport: speechTransport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		finish("provider_network_error", http.StatusBadGateway, 0, len(audio))
		transcriptionError(w, requestID, http.StatusBadGateway, "speech_unavailable", "The transcription service could not be reached. Try again or type your answer.")
		return
	}
	defer resp.Body.Close()
	providerBody, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	providerRequestID := ""
	var providerMetadata struct {
		RequestID string `json:"request_id"`
	}
	_ = json.Unmarshal(providerBody, &providerMetadata)
	providerRequestID = providerMetadata.RequestID
	if providerRequestID == "" {
		var wrapped struct {
			Metadata struct {
				RequestID string `json:"request_id"`
			} `json:"metadata"`
		}
		_ = json.Unmarshal(providerBody, &wrapped)
		providerRequestID = wrapped.Metadata.RequestID
	}
	if providerRequestID != "" {
		providerRequestID = safeProviderRequestID(providerRequestID)
	}
	if resp.StatusCode != http.StatusOK {
		status, code, message := http.StatusBadGateway, "speech_unavailable", "The transcription service could not process this recording. Try again or type your answer."
		if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusUnprocessableEntity {
			status, code, message = http.StatusUnprocessableEntity, "audio_rejected", "The recording could not be read. Try recording again or type your answer."
		} else if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
			status, code, message = http.StatusServiceUnavailable, "speech_busy", "Transcription is temporarily unavailable. Try again shortly or type your answer."
		}
		log.Printf("speech_transcribe request_id=%s provider_request_id=%s phase=provider_rejected mime=%q bytes=%d duration_ms=%s status=%d provider_status=%d elapsed_ms=%d", requestID, providerRequestID, contentType, len(audio), recordingDuration(r), status, resp.StatusCode, time.Since(started).Milliseconds())
		transcriptionError(w, requestID, status, code, message)
		return
	}
	if readErr != nil {
		finish("provider_response_read_failed", http.StatusBadGateway, resp.StatusCode, len(audio))
		transcriptionError(w, requestID, http.StatusBadGateway, "speech_unavailable", "The transcription service returned an incomplete response. Try again or type your answer.")
		return
	}
	var result struct {
		Results struct {
			Channels []struct {
				Alternatives []struct {
					Transcript string `json:"transcript"`
				} `json:"alternatives"`
			} `json:"channels"`
		} `json:"results"`
	}
	if json.Unmarshal(providerBody, &result) != nil {
		finish("provider_invalid_json", http.StatusBadGateway, resp.StatusCode, len(audio))
		transcriptionError(w, requestID, http.StatusBadGateway, "speech_unavailable", "The transcription service returned an invalid response. Try again or type your answer.")
		return
	}
	transcript := ""
	if len(result.Results.Channels) > 0 && len(result.Results.Channels[0].Alternatives) > 0 {
		transcript = result.Results.Channels[0].Alternatives[0].Transcript
	}
	if strings.TrimSpace(transcript) == "" {
		finish("no_speech", http.StatusOK, resp.StatusCode, len(audio))
	} else {
		finish("success", http.StatusOK, resp.StatusCode, len(audio))
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"transcript": transcript})
}

func safeProviderRequestID(s string) string {
	if len(s) > 64 {
		s = s[:64]
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			return ""
		}
	}
	return s
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
