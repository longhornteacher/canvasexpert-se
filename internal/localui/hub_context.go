package localui

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"canvasexpertse/internal/canvas"
	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/platform"
	"canvasexpertse/internal/workflow"
	"canvasexpertse/internal/workspace"
)

// maxContextText bounds each teacher-written box so a stray paste cannot bloat
// the agent's always-read settings files.
const maxContextText = 8000

const (
	canvasHealthChecking      = "Checking…"
	canvasHealthConnected     = "Connected"
	canvasHealthRejected      = "Token rejected"
	canvasHealthUnavailable   = "Unavailable"
	canvasHealthNotConnected  = "Not connected"
	defaultCanvasHealthWindow = 8 * time.Second // the page polls for 15s; keep this well inside it
)

const canvasHealthScript = `(()=>{
const panel=document.querySelector("[data-canvas-health]");
if(!panel||panel.dataset.state!=="Checking…")return;
const state=panel.querySelector("[data-health-state]");
const note=panel.querySelector("[data-health-note]");
const mark=panel.querySelector("[data-health-mark]");
const buttons=document.querySelectorAll("[data-canvas-test]");
const setButton=(checking)=>buttons.forEach(b=>{b.disabled=checking;b.textContent=checking?"Checking…":"Test"});
const deadline=Date.now()+15000;let finished=false;
const finishUnavailable=()=>{if(finished)return;finished=true;panel.dataset.state="Unavailable";state.textContent="Unavailable";note.textContent="The local page could not read the check result. Reload to retry.";mark.className="status-mark bad";setButton(false)};
setButton(true);
const poll=()=>fetch("/_cse/canvas-health",{cache:"no-store",headers:{Accept:"application/json"}})
 .then(r=>{if(!r.ok)throw new Error("status");return r.json()})
 .then(s=>{panel.dataset.state=s.state;state.textContent=s.state;note.textContent=s.note;mark.className="status-mark "+(s.state==="Connected"?"good":s.state==="Token rejected"||s.state==="Unavailable"?"bad":s.state==="Checking…"?"waiting":"");const account=panel.querySelector("[data-health-account]"),accountRow=panel.querySelector("[data-health-account-row]");if(account&&accountRow){account.textContent=s.account||"";accountRow.hidden=!s.account}if(s.state==="Checking…"){if(Date.now()<deadline)setTimeout(poll,250);else finishUnavailable()}else{finished=true;setButton(false)}})
 .catch(()=>{if(Date.now()<deadline)setTimeout(poll,250);else finishUnavailable()});
setTimeout(poll,150);
})();`

func (h *Hub) loadSavedCredential() (platform.Credential, error) {
	if h.loadCredentialFn != nil {
		return h.loadCredentialFn()
	}
	return platform.LoadCredential()
}

func canvasHealthNote(state string) string {
	switch state {
	case canvasHealthChecking:
		return "Checking the saved token with Canvas."
	case canvasHealthConnected:
		return "Canvas accepted the saved token."
	case canvasHealthRejected:
		return "Canvas rejected the saved token. Replace it to reconnect."
	case canvasHealthUnavailable:
		return "Canvas could not be reached. The saved token may still be valid; try again."
	default:
		return "No Canvas access token is saved on this computer."
	}
}

func (h *Hub) invalidateCanvasHealth() {
	h.canvasHealthGeneration++
	h.canvasHealthFingerprint = ""
	h.canvasHealth = ""
	h.canvasHealthNote = ""
}

func (h *Hub) markCanvasValidated(c platform.Credential) {
	h.canvasHealthGeneration++
	h.canvasHealthFingerprint = credentialFingerprint(c)
	h.canvasHealth = canvasHealthConnected
	h.canvasHealthNote = canvasHealthNote(canvasHealthConnected)
}

func (h *Hub) ensureCanvasHealth() {
	h.startCanvasHealthCheck(false, false)
}

func (h *Hub) retryCanvasHealth() {
	h.startCanvasHealthCheck(true, true)
}

func (h *Hub) startCanvasHealthCheck(force, manual bool) {
	credential, err := h.loadSavedCredential()
	h.mu.Lock()
	if err != nil {
		state, note := canvasHealthUnavailable, "Could not read the saved Access token from Windows Credential Manager. Check credential access on this computer."
		if errors.Is(err, platform.ErrNoCredential) {
			state, note = canvasHealthNotConnected, canvasHealthNote(canvasHealthNotConnected)
		}
		if h.canvasHealth != state || h.canvasHealthFingerprint != "" {
			h.canvasHealthGeneration++
			h.canvasHealthFingerprint = ""
			h.canvasHealth, h.canvasHealthNote = state, note
		}
		h.canvasUser = ""
		h.mu.Unlock()
		return
	}
	fingerprint := credentialFingerprint(credential)
	if h.canvasHealthFingerprint == fingerprint && h.canvasHealth != "" &&
		(h.canvasHealth == canvasHealthChecking || !force) {
		h.mu.Unlock()
		return
	}
	h.canvasHealthGeneration++
	generation := h.canvasHealthGeneration
	h.canvasHealthFingerprint = fingerprint
	h.canvasHealth, h.canvasHealthNote = canvasHealthChecking, canvasHealthNote(canvasHealthChecking)
	h.canvasUser = ""
	whoamiFn := h.whoamiFn
	timeout := h.canvasHealthTimeout
	if timeout <= 0 {
		timeout = defaultCanvasHealthWindow
	}
	base := h.ctx
	if base == nil {
		base = context.Background()
	}
	ctx, cancel := context.WithTimeout(base, timeout)
	h.mu.Unlock()

	go func() {
		defer cancel()
		var name string
		var checkErr error
		if whoamiFn != nil {
			name, checkErr = whoamiFn(ctx)
		} else {
			client, newErr := canvas.New(credential.Origin, credential.Token)
			if newErr != nil {
				checkErr = newErr
			} else {
				var user workflow.CanvasUser
				checkErr = client.Get(ctx, "/api/v1/users/self", &user)
				name = user.Name
			}
		}
		if ctx.Err() != nil {
			checkErr = ctx.Err()
		}
		latest, latestErr := h.loadSavedCredential()
		h.mu.Lock()
		defer h.mu.Unlock()
		if generation != h.canvasHealthGeneration || fingerprint != h.canvasHealthFingerprint || latestErr != nil || credentialFingerprint(latest) != fingerprint {
			return
		}
		if checkErr == nil {
			h.canvasHealth, h.canvasHealthNote = canvasHealthConnected, canvasHealthNote(canvasHealthConnected)
			h.canvasUser = name
			if manual {
				h.connectionTested = true
			}
			return
		}
		h.canvasUser = ""
		var ce *canvas.Error
		if errors.As(checkErr, &ce) && ce.Kind == "authentication" {
			h.canvasHealth, h.canvasHealthNote = canvasHealthRejected, canvasHealthNote(canvasHealthRejected)
			return
		}
		h.canvasHealth, h.canvasHealthNote = canvasHealthUnavailable, canvasFailureNote(checkErr)
	}()
}

// canvasFailureNote distinguishes a Canvas response from no response; neither
// proves the saved token is bad. canvas.Error text never contains the token.
func canvasFailureNote(err error) string {
	var ce *canvas.Error
	switch {
	case errors.As(err, &ce) && ce.Status > 0:
		return fmt.Sprintf("Canvas answered but the check did not pass (%s; HTTP %d). Canvas did not reject the saved token; check the Canvas address or try again.", ce.Error(), ce.Status)
	case errors.Is(err, context.DeadlineExceeded):
		return "Canvas did not answer in time. The saved token may still be valid; try again."
	default:
		return "Canvas could not be reached (" + err.Error() + "). The saved token may still be valid; try again."
	}
}

func credentialFingerprint(c platform.Credential) string {
	sum := sha256.Sum256([]byte(c.Origin + "\x00" + c.Token))
	return hex.EncodeToString(sum[:])
}

const gradingBody = "Edited from the CE-SE hub Settings page. `voice` is how feedback should sound; `feedback_guidance` holds grading habits and rubric preferences. Put student-specific notes in `students.md`. Never store a Canvas token here.\n"

const notesBody = "Edited from the CE-SE hub Students page. Each note names a student by stable `student_ref` (or an unambiguous `name`, with optional `course`). Notes are private teacher context and must never be copied into student-facing feedback.\n"

func teacherFile(root, rel string) (string, error) { return workspace.ResolveTeacherPath(root, rel) }

func notesPath(root string) string { return filepath.Join(root, "context", "students.md") }

// readGrading tolerates a missing file so a fresh workspace still shows the form.
func readGrading(root string) (domain.GradingConfig, string, error) {
	config := domain.GradingConfig{Schema: 1}
	p, err := teacherFile(root, "context/grading.md")
	if err != nil {
		return config, "", err
	}
	body, err := workspace.ReadDocument(p, &config)
	if errors.Is(err, os.ErrNotExist) {
		return domain.GradingConfig{Schema: 1}, gradingBody, nil
	}
	return config, body, err
}

func readNotes(root string) (domain.StudentNotes, string, error) {
	notes := domain.StudentNotes{Schema: 1}
	body, err := workspace.ReadDocument(notesPath(root), &notes)
	if errors.Is(err, os.ErrNotExist) {
		return domain.StudentNotes{Schema: 1}, notesBody, nil
	}
	return notes, body, err
}

// hubNote is the note the hub edits for one student: referenced by stable ref
// and not scoped to a course, so it follows the student everywhere.
func hubNote(n domain.StudentNote, ref string) bool {
	return n.StudentRef == ref && n.Course == ""
}

// notesByRef gathers each student's hub-editable note text.
func notesByRef(notes domain.StudentNotes) map[string]string {
	out := map[string]string{}
	for _, n := range notes.Notes {
		if n.StudentRef != "" && n.Course == "" {
			if prev := out[n.StudentRef]; prev != "" {
				out[n.StudentRef] = prev + "\n\n" + n.Note
			} else {
				out[n.StudentRef] = n.Note
			}
		}
	}
	return out
}

// cleanText normalizes browser line endings and trims; ok is false when too long.
func cleanText(raw string) (string, bool) {
	text := strings.TrimSpace(strings.ReplaceAll(raw, "\r\n", "\n"))
	return text, len([]rune(text)) <= maxContextText && !strings.ContainsRune(text, 0)
}

// saveNotes replaces the hub notes for the edited students and leaves every
// other note (name-based or course-scoped, written by hand) untouched.
func saveNotes(root string, edited map[string]string) error {
	notes, body, err := readNotes(root)
	if err != nil {
		return err
	}
	kept := notes.Notes[:0:0]
	for _, n := range notes.Notes {
		if _, changing := edited[n.StudentRef]; changing && hubNote(n, n.StudentRef) {
			continue
		}
		kept = append(kept, n)
	}
	for ref, text := range edited {
		if text != "" {
			kept = append(kept, domain.StudentNote{StudentRef: ref, Note: text})
		}
	}
	notes.Notes = kept
	return workspace.WriteDocument(notesPath(root), &notes, body)
}

func (h *Hub) saveGrading(w http.ResponseWriter, r *http.Request) {
	if !h.checkPost(w, r) || h.needWorkspace(w) {
		return
	}
	config, body, err := readGrading(h.root)
	if err != nil {
		h.redirect(w, r, "/settings?section=preferences", "", "Could not read context/grading.md: "+err.Error())
		return
	}
	voice, okVoice := cleanText(r.PostForm.Get("voice"))
	guidance, okGuidance := cleanText(r.PostForm.Get("feedback_guidance"))
	if !okVoice || !okGuidance {
		h.redirect(w, r, "/settings?section=preferences", "", "Each box holds up to 8,000 characters.")
		return
	}
	config.Schema, config.Voice, config.FeedbackGuidance = 1, voice, guidance
	p, pathErr := teacherFile(h.root, "context/grading.md")
	if pathErr != nil {
		h.redirect(w, r, "/settings?section=preferences", "", "Could not save context/grading.md: "+pathErr.Error())
		return
	}
	if err = workspace.WriteDocument(p, &config, body); err != nil {
		h.redirect(w, r, "/settings?section=preferences", "", "Could not save context/grading.md: "+err.Error())
		return
	}
	h.redirect(w, r, "/settings?section=preferences&clear=grading", "Feedback voice saved. The agent uses it next time it writes feedback.", "")
}

// whoami asks Canvas which account the saved token belongs to.
func (h *Hub) whoami(r *http.Request) (string, error) {
	if h.whoamiFn != nil {
		return h.whoamiFn(r.Context())
	}
	c, err := h.canvasClient()
	if err != nil {
		return "", err
	}
	ctx, cancel := h.reqCtx()
	defer cancel()
	var user workflow.CanvasUser
	if err = c.Get(ctx, "/api/v1/users/self", &user); err != nil {
		return "", err
	}
	return user.Name, nil
}

func (h *Hub) testCanvas(w http.ResponseWriter, r *http.Request) {
	if !h.checkPost(w, r) {
		return
	}
	h.mu.Lock()
	h.connectionTested = false
	h.canvasUser = ""
	h.mu.Unlock()
	h.retryCanvasHealth()
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

func (h *Hub) forgetCanvas(w http.ResponseWriter, r *http.Request) {
	if !h.checkPost(w, r) {
		return
	}
	if r.PostForm.Get("confirm") != "on" {
		h.redirect(w, r, "/settings", "", "Tick the box to confirm before removing the token.")
		return
	}
	forget := h.forgetFn
	if forget == nil {
		forget = platform.DeleteCredential
	}
	if err := forget(); err != nil {
		h.redirect(w, r, "/settings", "", "Windows could not remove the token: "+err.Error())
		return
	}
	h.canvasUser = ""
	h.connectionTested = false
	h.setupSaved = false
	h.setupValidated = false
	h.canvasHealthGeneration++
	h.canvasHealthFingerprint = ""
	h.canvasHealth, h.canvasHealthNote = canvasHealthNotConnected, canvasHealthNote(canvasHealthNotConnected)
	h.redirect(w, r, "/settings", "Token removed from this computer. Your CE-SE files are untouched.", "")
}
