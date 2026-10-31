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
	SourceText   string `json:"source_text"`
	ReviewStatus string `json:"review_status"`
}
type session struct {
	mu                                sync.Mutex
	Filename, FullText, Stage, Notice string
	Truncated                         bool
	State                             discovery.State
	Saved                             *evidence
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
	body, _ := json.Marshal(map[string]any{"model": env("BASETEN_MODEL", "meta-models/muse-spark-open"), "max_tokens": 2048, "messages": []map[string]string{{"role": "system", "content": policy}, {"role": "user", "content": prompt}}})
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
			v.Stage = "preview"
		case "/begin":
			if v.Stage == "preview" {
				leads, e := discovery.FindLeads(r.Context(), s.generate, v.State.Resume.Text)
				if e != nil {
					v.Notice = message(e)
				} else {
					v.State.Leads = leads
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
				v.Saved = &evidence{Proposal: p, SourceText: source, ReviewStatus: "candidate_reviewed"}
				v.Stage = "saved"
			}
		case "/reset":
			v.Filename = ""
			v.FullText = ""
			v.Stage = "welcome"
			v.Notice = ""
			v.Truncated = false
			v.State = discovery.State{}
			v.Saved = nil
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	if r.URL.Path == "/download" && v.Saved != nil {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", `attachment; filename="career-evidence.json"`)
		json.NewEncoder(w).Encode(v.Saved)
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
