package mcpserver

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"canvasexpertse/internal/workflow"

	"canvasexpertse/internal/hubcoord"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type reviewTask struct {
	id, url, kind    string
	subject          string // reviewed file or folder, for a review that outlives its adapter
	done             chan struct{}
	external, reused bool
	mu               sync.Mutex
	stdout, stderr   strings.Builder
	err              error
	process          *os.Process
}

type LocalUIInput struct {
	Page string `json:"page" jsonschema:"Teacher-only local page: hub, setup, or students,enum=hub,enum=setup,enum=students"`
}
type LocalUIStatusInput struct {
	SessionID string `json:"session_id" jsonschema:"ID returned by start_local_ui"`
}

func (s *Server) registerSessionTools(server *mcp.Server) {
	addActionTool(server, "start_local_ui", "Open a private teacher browser page for the CE-SE hub, Canvas credentials, or student alias/settings editor. Return only the loopback URL and session status. Credentials and real student identities remain in the private browser page; never fetch its contents through MCP.", false, func(ctx context.Context, _ *mcp.CallToolRequest, in LocalUIInput) (TextOutput, error) {
		return s.startLocalUI(ctx, in)
	})
	addReadTool(server, "local_ui_status", "Check whether a private teacher browser session is ready or finished. Private page contents are never returned.", func(_ context.Context, _ *mcp.CallToolRequest, in LocalUIStatusInput) (TextOutput, error) {
		return s.localUIStatus(in.SessionID)
	})
}

func (s *Server) startReview(ctx context.Context, in ReviewInput) (TextOutput, error) {
	path, err := s.coursePath(in.Path)
	if err != nil {
		return TextOutput{}, err
	}
	if _, err = workflow.ValidateReviewInput(s.root, path, in.Course); err != nil {
		return TextOutput{}, err
	}
	args := []string{"review", path}
	if in.Course != "" {
		args = append(args, in.Course)
	}
	return s.startReviewSession(ctx, path, args)
}

// startReviewSession reconnects to a review page for the same file that is
// still open (for example after the agent app restarted this adapter) instead
// of starting a second page; otherwise it starts one.
func (s *Server) startReviewSession(ctx context.Context, subject string, args []string) (TextOutput, error) {
	if rec, ok := hubcoord.FindPage(s.root, "review", subject); ok {
		pageURL := rec.URL + "/review"
		var nonce [16]byte
		if _, err := rand.Read(nonce[:]); err != nil {
			return TextOutput{}, err
		}
		task := &reviewTask{id: hex.EncodeToString(nonce[:]), kind: "review", url: pageURL, subject: subject, done: make(chan struct{}), external: true}
		s.mu.Lock()
		if s.reviews == nil {
			s.reviews = make(map[string]*reviewTask)
		}
		s.reviews[task.id] = task
		s.mu.Unlock()
		return TextOutput{State: "pending", ReviewID: task.id, URL: pageURL, Text: "Reconnected to the review page that is still open for this file: " + pageURL + ". review_id: " + task.id + ". No Canvas changes occur unless the teacher approves on the page."}, nil
	}
	return s.startSession(ctx, "review", args)
}

func (s *Server) startLocalUI(ctx context.Context, in LocalUIInput) (TextOutput, error) {
	commands := map[string]string{"hub": "ui", "setup": "setup", "students": "students"}
	command, ok := commands[in.Page]
	if !ok {
		return TextOutput{}, errors.New("page must be hub, setup, or students")
	}
	if in.Page == "hub" {
		if loc, found := hubcoord.Find(s.root); found {
			return s.trackReusedHub(loc.URL)
		}
	}
	return s.startSession(ctx, in.Page, []string{command})
}

func (s *Server) trackReusedHub(pageURL string) (TextOutput, error) {
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return TextOutput{}, err
	}
	task := &reviewTask{id: hex.EncodeToString(nonce[:]), kind: "hub", url: pageURL, done: make(chan struct{}), external: true}
	s.mu.Lock()
	if s.sessions == nil {
		s.sessions = make(map[string]*reviewTask)
	}
	s.sessions[task.id] = task
	s.mu.Unlock()
	result := TextOutput{State: "pending", SessionID: task.id, URL: pageURL, Text: "Reusing the running private hub for this UserInstall. It remains open independently of this agent session; ask the teacher to open " + pageURL + ". Never fetch the page contents through MCP."}
	return result, nil
}

func (s *Server) startSession(ctx context.Context, kind string, args []string) (TextOutput, error) {
	full := append([]string{"--workspace", s.root}, args...)
	var cmd *exec.Cmd
	if s.newSessionCommand != nil {
		cmd = s.newSessionCommand(full)
	} else {
		exe, err := os.Executable()
		if err != nil {
			return TextOutput{}, err
		}
		cmd = exec.Command(exe, full...)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return TextOutput{}, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return TextOutput{}, err
	}
	var nonce [16]byte
	if _, err = rand.Read(nonce[:]); err != nil {
		return TextOutput{}, err
	}
	task := &reviewTask{id: hex.EncodeToString(nonce[:]), kind: kind, done: make(chan struct{})}
	s.mu.Lock()
	tasks := s.sessions
	if kind == "review" {
		tasks = s.reviews
	}
	if tasks == nil {
		tasks = make(map[string]*reviewTask)
		if kind == "review" {
			s.reviews = tasks
		} else {
			s.sessions = tasks
		}
	}
	for _, current := range tasks {
		if current.kind != kind {
			continue
		}
		select {
		case <-current.done:
		default:
			s.mu.Unlock()
			idKey := "session_id"
			if kind == "review" {
				idKey = "review_id"
			}
			return TextOutput{}, fmt.Errorf("a CE-SE %s session is already running; %s: %s; check its status", kind, idKey, current.id)
		}
	}
	if err = cmd.Start(); err != nil {
		s.mu.Unlock()
		return TextOutput{}, err
	}
	task.process = cmd.Process
	tasks[task.id] = task
	s.mu.Unlock()
	stdoutDone, stderrDone := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(stdoutDone)
		if kind != "review" {
			_, _ = io.Copy(io.Discard, stdout)
			return
		}
		b, _ := io.ReadAll(stdout)
		task.mu.Lock()
		task.stdout.Write(b)
		task.mu.Unlock()
	}()
	urlReady := make(chan struct{}, 1)
	go func() {
		defer close(stderrDone)
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			if kind == "hub" && strings.Contains(line, "Reusing the running CanvasExpert-SE hub") {
				task.mu.Lock()
				task.reused = true
				task.external = true
				task.mu.Unlock()
			}
			raw := sessionURL(kind, line)
			task.mu.Lock()
			if kind == "review" && raw == "" {
				task.stderr.WriteString(line + "\n")
			}
			if raw != "" {
				task.url = raw
			}
			task.mu.Unlock()
			if raw != "" {
				select {
				case urlReady <- struct{}{}:
				default:
				}
			}
		}
	}()
	go func() {
		<-stdoutDone
		<-stderrDone
		cmdErr := cmd.Wait()
		task.mu.Lock()
		task.err = cmdErr
		task.mu.Unlock()
		close(task.done)
	}()
	if kind != "review" {
		// Setup and student pages close with the adapter. A review page keeps
		// running so a teacher's approval still lands if the app restarts.
		go func() {
			select {
			case <-s.ctx.Done():
				_ = cmd.Process.Kill()
			case <-task.done:
			}
		}()
	}
	wait := s.sessionReadyWait
	if wait <= 0 {
		wait = 30 * time.Second
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-urlReady:
	case <-task.done:
	case <-ctx.Done():
	case <-timer.C:
	}
	return task.status()
}

func sessionURL(kind, line string) string {
	prefixes := map[string]string{"review": "Open the local review page: ", "setup": "Open the local setup page: ", "students": "Open the local student editor: ", "hub": "CanvasExpert-SE hub is running: "}
	prefix := prefixes[kind]
	if prefix == "" || !strings.HasPrefix(line, prefix) {
		return ""
	}
	fields := strings.Fields(strings.TrimPrefix(line, prefix))
	if len(fields) == 0 {
		return ""
	}
	u, err := url.Parse(fields[0])
	if err != nil || u.Scheme != "http" || u.Hostname() != "127.0.0.1" || u.Port() == "" || u.User != nil {
		return ""
	}
	return u.String()
}

func (t *reviewTask) output() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return strings.TrimSpace(t.stdout.String() + "\n" + t.stderr.String())
}

func (t *reviewTask) status() (TextOutput, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	idKey := "session_id"
	if t.kind == "review" {
		idKey = "review_id"
	}
	identity := fmt.Sprintf("%s: %s", idKey, t.id)
	result := TextOutput{State: "pending"}
	if t.kind == "review" {
		result.ReviewID = t.id
	} else {
		result.SessionID = t.id
	}
	select {
	case <-t.done:
		if t.external && t.kind == "hub" {
			result.State = "pending"
			result.Text = "Reusing the running private hub for this UserInstall. It remains open independently of this agent session. Ask the teacher to open " + t.url + ". Never fetch the page contents through MCP."
			result.URL = t.url
			return result, nil
		}
		result.State = "finished"
		if t.err != nil {
			result.State = "failed"
		}
		if t.kind != "review" {
			if t.kind == "setup" && t.err == nil {
				// The setup command exits successfully only after Canvas validation
				// and credential save both complete. Do not inspect its output.
				result.SetupStatusAvailable = true
				result.CredentialSaved = boolPointer(true)
				result.ValidationSucceeded = boolPointer(true)
				result.ConnectionTested = boolPointer(true)
				result.SetupOutcome = "saved_this_session"
			}
			if t.err != nil {
				result.Text = "Private teacher page failed or stopped. " + identity + ". Open the hub to inspect or retry."
				return result, nil
			}
			result.Text = "Private teacher page finished. " + identity
			return result, nil
		}
		var readable []string
		for _, line := range strings.Split(t.stdout.String(), "\n") {
			if strings.HasPrefix(line, "Review result: ") {
				var outcome struct {
					Status string           `json:"outcome"`
					Path   string           `json:"result_path"`
					Revise []ReviewRevision `json:"revision_notes"`
				}
				if json.Unmarshal([]byte(strings.TrimPrefix(line, "Review result: ")), &outcome) == nil {
					result.Outcome, result.ResultPath, result.RevisionNotes = outcome.Status, outcome.Path, outcome.Revise
					continue
				}
			}
			readable = append(readable, line)
		}
		var progress []string
		for _, line := range strings.Split(t.stderr.String(), "\n") {
			if !strings.HasPrefix(line, "Open the local review page:") {
				progress = append(progress, line)
			}
		}
		output := strings.TrimSpace(strings.Join(readable, "\n") + "\n" + strings.Join(progress, "\n"))
		if t.err != nil {
			result.Text = fmt.Sprintf("Review finished with a failed command: %v. %s\n%s", t.err, identity, output)
			return result, nil
		}
		result.Text = "Review finished. " + identity + "\n" + output
		return result, nil
	default:
		if t.url == "" {
			result.Text = "CE-SE " + t.kind + " is still starting. " + identity
			return result, nil
		}
		message := "Ask the teacher to open " + t.url + ". " + identity
		if t.kind == "review" {
			message = "The review page is opening in the teacher's browser; fallback address " + t.url + ". " + identity + ". No Canvas changes occur unless the teacher approves on the page."
		} else {
			message += ". This page is private to the teacher; do not read its contents through MCP."
			if t.kind == "setup" {
				message += " Connection test and save status are not confirmed while setup is still open."
			}
		}
		result.Text, result.URL = message, t.url
		return result, nil
	}
}

func (s *Server) reviewStatus(id string) (TextOutput, error) {
	s.mu.Lock()
	task := s.reviews[id]
	s.mu.Unlock()
	if task == nil {
		return TextOutput{}, errors.New("unknown review_id; if the app restarted, call start_review (or start_parent_review) again for the same file: it reconnects to a page that is still open, and a closed page has already saved its outcomes in the results file")
	}
	if task.external {
		return s.externalReviewStatus(task)
	}
	return task.status()
}
func (s *Server) localUIStatus(id string) (TextOutput, error) {
	s.mu.Lock()
	task := s.sessions[id]
	s.mu.Unlock()
	if task == nil {
		return TextOutput{}, errors.New("unknown session_id")
	}
	if task.kind == "hub" && task.external {
		if _, ok := hubcoord.Find(s.root); !ok {
			task.mu.Lock()
			select {
			case <-task.done:
			default:
				close(task.done)
			}
			task.external = false
			task.err = fmt.Errorf("hub stopped")
			task.mu.Unlock()
		}
	}
	result, err := task.status()
	if err != nil {
		return result, err
	}
	if task.kind == "hub" {
		task.mu.Lock()
		pageURL := task.url
		task.mu.Unlock()
		status, statusErr := fetchHubSetupStatus(pageURL)
		if statusErr == nil {
			result.SetupStatusAvailable = true
			result.CredentialSaved = boolPointer(status.CredentialSaved)
			result.ValidationSucceeded = boolPointer(status.ValidationSucceeded)
			result.ConnectionTested = boolPointer(status.ConnectionTested)
			result.SetupOutcome = status.SetupOutcome
		} else if result.State != "failed" {
			result.Text += ". Connection status is unavailable; this agent session cannot confirm whether the saved token was tested"
		}
	}
	return result, nil
}

type hubSetupStatus struct {
	CredentialSaved     bool   `json:"credential_saved"`
	SetupOutcome        string `json:"setup_outcome"`
	ValidationSucceeded bool   `json:"validation_succeeded"`
	ConnectionTested    bool   `json:"connection_tested"`
}

// fetchHubSetupStatus reads only the hub's intentionally non-secret status
// fields. The session URL is accepted only for a loopback HTTP origin.
func fetchHubSetupStatus(pageURL string) (hubSetupStatus, error) {
	var status hubSetupStatus
	u, err := url.Parse(pageURL)
	if err != nil || u.Scheme != "http" || u.Hostname() != "127.0.0.1" || u.Port() == "" || u.User != nil || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return status, errors.New("hub status route unavailable")
	}
	u.Path, u.RawPath = "/setup/status", ""
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return status, errors.New("hub status route unavailable")
	}
	client := &http.Client{Timeout: 2 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		return status, errors.New("hub status route unavailable")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return status, errors.New("hub status route unavailable")
	}
	if err = json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&status); err != nil {
		return hubSetupStatus{}, errors.New("hub status route unavailable")
	}
	switch status.SetupOutcome {
	case "not_saved", "saved_not_tested", "saved_this_session":
	default:
		return hubSetupStatus{}, errors.New("hub status route unavailable")
	}
	return status, nil
}

// Run invokes this synchronously before returning: the CLI uses os.Exit, so a
// cancellation goroutine alone would not reliably terminate child UI servers.
func (s *Server) stopSessions() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, tasks := range []map[string]*reviewTask{s.reviews, s.sessions} {
		for _, task := range tasks {
			select {
			case <-task.done:
			default:
				if task.process != nil && !task.external && task.kind != "review" {
					_ = task.process.Kill()
				}
			}
		}
	}
}

// externalReviewStatus follows a review page started by an earlier adapter.
// Its outcome is not on this adapter's pipes; the page saves outcomes in the
// results file (or outcomes.json for family email) and revision notes in
// <results>.review.json.
func (s *Server) externalReviewStatus(task *reviewTask) (TextOutput, error) {
	result := TextOutput{State: "pending", ReviewID: task.id, URL: task.url}
	if hubcoord.PageOpen(s.root, task.url) {
		result.Text = "The review page is still open at " + task.url + ". review_id: " + task.id + ". No Canvas changes occur unless the teacher approves on the page."
		return result, nil
	}
	result.State, result.URL = "finished", ""
	result.ResultPath = task.subject
	if st, err := workflow.LoadReviewState(task.subject); err == nil {
		for ref, item := range st.Items {
			if item.Status == "revise" {
				result.RevisionNotes = append(result.RevisionNotes, ReviewRevision{Ref: ref, Note: item.Note})
			}
		}
	}
	result.Text = "The review page that outlived the previous connection has closed. review_id: " + task.id + ". Read the outcomes saved in " + task.subject + " (outcomes.json for family email) for what was approved and posted; revision notes, if any, are listed here."
	return result, nil
}
