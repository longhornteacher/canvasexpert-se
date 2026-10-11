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
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"canvasexpertse/internal/workflow"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type reviewTask struct {
	id, url, kind  string
	done           chan struct{}
	mu             sync.Mutex
	stdout, stderr strings.Builder
	err            error
	process        *os.Process
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
	return s.startSession(ctx, "review", args)
}

func (s *Server) startLocalUI(ctx context.Context, in LocalUIInput) (TextOutput, error) {
	commands := map[string]string{"hub": "ui", "setup": "setup", "students": "students"}
	command, ok := commands[in.Page]
	if !ok {
		return TextOutput{}, errors.New("page must be hub, setup, or students")
	}
	return s.startSession(ctx, in.Page, []string{command})
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
	go func() {
		select {
		case <-s.ctx.Done():
			_ = cmd.Process.Kill()
		case <-task.done:
		}
	}()
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
		result.State = "finished"
		if t.err != nil {
			result.State = "failed"
		}
		if t.kind != "review" {
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
			message += ". No Canvas changes occur unless the teacher approves on the page."
		} else {
			message += ". This page is private to the teacher; do not read its contents through MCP."
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
		return TextOutput{}, errors.New("unknown review_id")
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
	return task.status()
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
				if task.process != nil {
					_ = task.process.Kill()
				}
			}
		}
	}
}
