package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	discovery "github.com/jenksed/yourmove-career-discovery"
)

const maxUpload = 10 << 20

type evidence struct {
	discovery.Proposal
	SourceText   string               `json:"source_text"`
	ReviewStatus string               `json:"review_status"`
	ID           int64                `json:"id"`
	Revision     int64                `json:"revision"`
	Responses    []discovery.Response `json:"responses"`
}
type session struct {
	mu                                sync.Mutex
	Filename, FullText, Stage, Notice string
	Truncated                         bool
	State                             discovery.State
	Saved                             *evidence
	Discoveries                       []evidence
	Synthesis                         *discovery.ConnectionSynthesis
	Editing                           int64
	InputKind                         string
	OriginalLeads                     []discovery.Lead
	Challenge                         string
}
type server struct {
	mu       sync.Mutex
	sessions map[string]*session
	generate discovery.Generate
}

func (s *server) get(w http.ResponseWriter, r *http.Request) *session {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c, e := r.Cookie("realize_session"); e == nil {
		if v := s.sessions[c.Value]; v != nil {
			return v
		}
	}
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	id := hex.EncodeToString(b)
	v := &session{Stage: "welcome"}
	s.sessions[id] = v
	http.SetCookie(w, &http.Cookie{Name: "realize_session", Value: id, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode, Secure: r.TLS != nil})
	return v
}
func loadEnv(path string) {
	b, e := os.ReadFile(path)
	if e != nil {
		return
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		k = strings.TrimSpace(k)
		if !ok || k == "" {
			continue
		}
		if _, exists := os.LookupEnv(k); !exists {
			os.Setenv(k, strings.Trim(strings.TrimSpace(v), "\"'"))
		}
	}
}
func env(k, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return fallback
}
func generate(ctx context.Context, prompt, policy string) (string, error) {
	key := strings.TrimSpace(os.Getenv("BASETEN_API_KEY"))
	if key == "" {
		return "", errors.New("Baseten is not configured. Add BASETEN_API_KEY to .env and restart.")
	}
	body, _ := json.Marshal(map[string]any{"model": env("BASETEN_MODEL", "meta-models/muse-spark-open"), "max_tokens": 3072, "reasoning_effort": "low", "response_format": map[string]string{"type": "json_object"}, "messages": []map[string]string{{"role": "system", "content": policy}, {"role": "user", "content": prompt}}})
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	req, e := http.NewRequestWithContext(ctx, "POST", strings.TrimRight(env("BASETEN_BASE_URL", "https://inference.baseten.co/v1"), "/")+"/chat/completions", bytes.NewReader(body))
	if e != nil {
		return "", errors.New("The inference service is not configured correctly.")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)
	client := http.Client{Timeout: 120 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, e := client.Do(req)
	if e != nil {
		return "", errors.New("Could not reach Baseten. Your answers are saved in this session; please try again.")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Baseten returned HTTP %d. Your answers are saved in this session; please try again.", resp.StatusCode)
	}
	raw, e := io.ReadAll(io.LimitReader(resp.Body, 128<<10))
	if e != nil {
		return "", errors.New("Could not read the inference response.")
	}
	var result struct {
		Choices []struct {
			Finish  string `json:"finish_reason"`
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if json.Unmarshal(raw, &result) != nil || len(result.Choices) != 1 || result.Choices[0].Finish != "stop" {
		return "", errors.New("The inference response was incomplete. Your answers are saved; please try again.")
	}
	return result.Choices[0].Message.Content, nil
}
func extract(name string, data []byte) (string, error) {
	var out string
	switch strings.ToLower(filepath.Ext(name)) {
	case ".pdf":
		if !bytes.HasPrefix(data, []byte("%PDF-")) {
			return "", errors.New("This file is not a valid PDF.")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "pdftotext", "-layout", "-", "-")
		cmd.Stdin = bytes.NewReader(data)
		b, e := cmd.Output()
		if e != nil {
			return "", errors.New("This PDF could not be read. It may be damaged or password protected; make sure pdftotext is installed.")
		}
		out = string(b)
	case ".docx":
		z, e := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if e != nil {
			return "", errors.New("This Word document is invalid or unreadable.")
		}
		var doc *zip.File
		for _, f := range z.File {
			if f.Name == "word/document.xml" {
				doc = f
				break
			}
		}
		if doc == nil || doc.UncompressedSize64 > 20<<20 {
			return "", errors.New("This Word document is invalid or too complex to read.")
		}
		f, e := doc.Open()
		if e != nil {
			return "", errors.New("This Word document could not be read.")
		}
		defer f.Close()
		d := xml.NewDecoder(io.LimitReader(f, 20<<20))
		var b strings.Builder
		inText := false
		for {
			tok, e := d.Token()
			if e == io.EOF {
				break
			}
			if e != nil {
				return "", errors.New("This Word document contains unreadable text.")
			}
			switch t := tok.(type) {
			case xml.StartElement:
				if t.Name.Local == "t" {
					inText = true
				}
				if t.Name.Local == "tab" {
					b.WriteByte('\t')
				}
			case xml.EndElement:
				if t.Name.Local == "t" {
					inText = false
				}
				if t.Name.Local == "p" {
					b.WriteByte('\n')
				}
			case xml.CharData:
				if inText {
					b.Write([]byte(t))
				}
			}
		}
		out = b.String()
	default:
		return "", errors.New("Choose a PDF or DOCX résumé.")
	}
	out = strings.TrimSpace(strings.ReplaceAll(out, "\x00", ""))
	if len(strings.Fields(out)) < 5 || !utf8.ValidString(out) {
		return "", errors.New("No readable text was found. Scanned documents need a text-based PDF or DOCX; OCR is not supported.")
	}
	return out, nil
}
func bounded(t string) (string, bool) {
	if len(t) <= 8000 {
		return t, false
	}
	n := 8000
	for !utf8.RuneStart(t[n]) {
		n--
	}
	return t[:n], true
}
func (s *server) serve(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	v := s.get(w, r)
	v.mu.Lock()
	defer v.mu.Unlock()
	if r.Method == http.MethodPost {
		if origin := r.Header.Get("Origin"); origin != "" && origin != "http://"+r.Host && origin != "https://"+r.Host {
			http.Error(w, "Invalid origin", 403)
			return
		}
		v.Notice = ""
		switch r.URL.Path {
		case "/upload":
			r.Body = http.MaxBytesReader(w, r.Body, maxUpload+(1<<20))
			if e := r.ParseMultipartForm(1 << 20); e != nil {
				v.Notice = "The file is too large or could not be uploaded. Maximum size is 10 MiB."
				break
			}
			defer r.MultipartForm.RemoveAll()
			f, h, e := r.FormFile("resume")
			if e != nil {
				v.Notice = "Choose a PDF or DOCX résumé."
				break
			}
			defer f.Close()
			b, e := io.ReadAll(io.LimitReader(f, maxUpload+1))
			if e != nil || len(b) > maxUpload {
				v.Notice = "The file is too large. Maximum size is 10 MiB."
				break
			}
			t, e := extract(h.Filename, b)
			if e != nil {
				v.Notice = e.Error()
				break
			}
			excerpt, truncated := bounded(t)
			state, e := discovery.New(excerpt)
			if e != nil {
				v.Notice = "The document text could not be used. Try another file."
				break
			}
			v.Filename = filepath.Base(h.Filename)
			v.FullText = t
			v.Truncated = truncated
			v.State = state
			v.Saved = nil
			v.Discoveries = nil
			v.Synthesis = nil
			v.InputKind = "upload"
			v.OriginalLeads = nil
			v.Stage = "preview"
		case "/paste":
			r.Body = http.MaxBytesReader(w, r.Body, 100<<10)
			if err := r.ParseForm(); err != nil {
				v.Notice = "Text is too large. Please use up to 100 KiB."
				break
			}
			t := strings.TrimSpace(r.FormValue("career_text"))
			excerpt, truncated := bounded(t)
			state, err := discovery.New(excerpt)
			if err != nil {
				v.Notice = "Please enter some career-related text."
				break
			}
			state.Resume.Kind = "pasted_text"
			v.FullText, v.Filename, v.InputKind, v.Truncated, v.State = t, "Pasted career text", "paste", truncated, state
			v.Discoveries, v.Saved, v.Synthesis = nil, nil, nil
			v.OriginalLeads = nil
			v.Stage = "preview"
		case "/begin":
			if v.Stage == "preview" {
				if v.InputKind == "paste" && r.FormValue("approach") == "focused" {
					v.Stage = "conversation"
					s.next(r, v)
					break
				}
				leads, e := discovery.FindLeads(r.Context(), s.generate, v.State.Resume.Text)
				if e != nil {
					v.Notice = message(e)
				} else {
					v.State.Leads = leads
					v.OriginalLeads = append([]discovery.Lead(nil), leads...)
					v.Stage = "leads"
				}
			}
		case "/select":
			if v.Stage == "leads" {
				st, e := discovery.SelectLead(v.State, r.FormValue("lead"))
				if e == nil {
					v.State = st
					v.Stage = "conversation"
					s.next(r, v)
				} else {
					v.Notice = "Please choose an experience to explore."
				}
			}
		case "/answer":
			if v.Stage == "conversation" && v.State.Question != "" {
				st, e := discovery.RecordAnswer(v.State, strings.TrimSpace(r.FormValue("answer")))
				if e != nil {
					v.Notice = "Please enter an answer of up to 2,000 bytes."
				} else {
					v.State = st
					s.next(r, v)
				}
			}
		case "/retry":
			if v.Stage == "conversation" && v.State.Question == "" {
				s.next(r, v)
			}
		case "/reveal":
			if v.Stage == "conversation" && v.State.ValidateProposal() == nil {
				v.Stage = "reveal"
			}
		case "/review":
			if v.Stage == "reveal" && v.State.ValidateProposal() == nil {
				v.Stage = "review"
			}
		case "/continue":
			if v.Stage == "reveal" || v.Stage == "review" {
				v.Stage = "conversation"
			}
		case "/reject":
			if v.Stage == "review" {
				v.State.Proposal = nil
				v.Stage = "conversation"
				v.Notice = "The suggestion was not kept. You can continue the conversation."
			}
		case "/keep":
			if v.Stage == "review" && v.State.ValidateProposal() == nil {
				p := *v.State.Proposal
				title := strings.TrimSpace(r.FormValue("title"))
				experience := strings.TrimSpace(r.FormValue("experience"))
				claim := strings.TrimSpace(r.FormValue("claim"))
				if title == "" || experience == "" || claim == "" || len(title) > 160 || len(experience) > 1500 || len(claim) > 1500 {
					v.Notice = "Please complete each field within its length limit."
					break
				}
				p.Title = title
				p.Experience = experience
				p.Claim = claim
				var source string
				for _, a := range v.State.Responses {
					if a.Source.ID == p.SourceID {
						source = a.Source.Text
					}
				}
				item := evidence{Proposal: p, SourceText: source, ReviewStatus: "candidate_reviewed", Responses: append([]discovery.Response(nil), v.State.Responses...)}
				if v.Editing != 0 {
					for i := range v.Discoveries {
						if v.Discoveries[i].ID == v.Editing {
							item.ID = v.Editing
							item.Revision = v.Discoveries[i].Revision + 1
							v.Discoveries[i] = item
							break
						}
					}
				} else if len(v.Discoveries) < 4 {
					item.ID = int64(len(v.Discoveries) + 1)
					item.Revision = 1
					v.Discoveries = append(v.Discoveries, item)
				} else {
					v.Notice = "This demo holds up to four discoveries."
					break
				}
				v.Saved = &item
				v.Editing = 0
				v.Synthesis = nil
				v.Challenge = ""
				v.Stage = "saved"
			}
		case "/explore":
			if len(v.Discoveries) < 4 && v.State.Resume.Text != "" {
				resume := v.State.Resume
				leads := v.OriginalLeads
				v.State = discovery.State{Resume: resume, Leads: leads}
				v.Editing = 0
				focus := strings.TrimSpace(r.FormValue("focus"))
				if len(focus) > 120 {
					v.Notice = "Please keep the next focus under 120 bytes."
					v.Stage = "saved"
					break
				}
				if focus != "" {
					v.State.Leads = []discovery.Lead{{ID: "lead-1", Title: focus, Why: "The candidate chose this direction for further exploration."}}
					v.State.SelectedLeadID = "lead-1"
					v.Stage = "conversation"
					s.next(r, v)
				} else if len(leads) > 0 {
					v.Stage = "leads"
				} else {
					v.Stage = "conversation"
					s.next(r, v)
				}
			}
		case "/collection":
			if len(v.Discoveries) > 0 {
				v.Stage = "saved"
			}
		case "/edit":
			for _, item := range v.Discoveries {
				if fmt.Sprint(item.ID) == r.FormValue("id") {
					p := item.Proposal
					v.State.Responses = append([]discovery.Response(nil), item.Responses...)
					v.State.Proposal = &p
					v.Editing = item.ID
					v.Stage = "review"
					break
				}
			}
		case "/connections":
			if len(v.Discoveries) >= 2 {
				out, err := discovery.SynthesizeConnections(r.Context(), s.generate, connectionEvidence(v.Discoveries))
				if err != nil {
					v.Notice = message(err)
				} else {
					v.Synthesis = &out
					v.Challenge = ""
					v.Stage = "connections"
				}
			}
		case "/challenge":
			if v.Synthesis != nil {
				challenge := strings.TrimSpace(r.FormValue("challenge"))
				if challenge == "" || len(challenge) > 2000 {
					v.Notice = "Enter a challenge of up to 2,000 bytes."
				} else {
					v.Challenge = challenge
					v.Notice = "Your challenge is recorded as your perspective, not approved evidence."
				}
				v.Stage = "connections"
			}
		case "/takeaway":
			if len(v.Discoveries) > 0 {
				v.Stage = "takeaway"
			}
		case "/reset":
			v.Filename = ""
			v.FullText = ""
			v.Stage = "welcome"
			v.Notice = ""
			v.Truncated = false
			v.State = discovery.State{}
			v.Saved = nil
			v.Discoveries = nil
			v.Synthesis = nil
			v.Editing = 0
			v.InputKind = ""
			v.OriginalLeads = nil
			v.Challenge = ""
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	if r.URL.Path == "/download" && v.Saved != nil {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", `attachment; filename="career-evidence.json"`)
		if len(v.Discoveries) == 1 {
			json.NewEncoder(w).Encode(v.Saved)
		} else {
			json.NewEncoder(w).Encode(v.Discoveries)
		}
		return
	}
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if e := page.Execute(w, v); e != nil {
		log.Print("template rendering failed")
	}
}
func connectionEvidence(items []evidence) []discovery.ConnectionEvidence {
	out := make([]discovery.ConnectionEvidence, 0, len(items))
	for _, item := range items {
		var sources []string
		for _, response := range item.Responses {
			sources = append(sources, response.Source.Text)
		}
		out = append(out, discovery.ConnectionEvidence{ID: item.ID, Revision: item.Revision, Title: item.Title, Situation: item.Experience, Actions: item.Claim, OutcomeState: "unknown", SourceTexts: sources})
	}
	return out
}
func message(e error) string {
	if errors.Is(e, discovery.ErrInvalid) {
		return "Realize could not produce a reliable, source-supported response this time. Your answers are saved; you can try again."
	}
	return e.Error()
}
func (s *server) next(r *http.Request, v *session) {
	st, e := discovery.Next(r.Context(), s.generate, v.State)
	if e != nil {
		v.Notice = message(e)
	} else {
		v.State = st
	}
}
func main() {
	loadEnv(".env")
	s := &server{sessions: map[string]*session{}, generate: generate}
	http.HandleFunc("/", s.serve)
	log.Print("YourMove Realize listening at http://localhost:8080")
	log.Fatal(http.ListenAndServe("127.0.0.1:8080", nil))
}

var page = template.Must(template.New("page").Parse(pageHTML))
